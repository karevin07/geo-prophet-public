// Command importer loads door-number coordinates and 實價登錄 sales into PostgreSQL.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"io"
	"log/slog"
	"os"
	"os/signal"
	"slices"
	"strings"
	"syscall"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/importer"
	"github.com/geo-prophet/geo-prophet/api/internal/infra/postgres"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/opendata"
	"github.com/geo-prophet/geo-prophet/api/internal/transit"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

const usage = `Usage: importer <command> [flags]

Commands:
  addresses  load door-number coordinates (門牌位置坐標) for geocoding
  sales      import 實價登錄 sales for seasons or the latest 10-day release
  rentals    import 實價登錄 leases (租賃) the same way
  presales   import 實價登錄 presales (預售屋) the same way; sync imports all three
  sync       bring everything up to date; with -interval, keep doing so
  boundaries recompute district boundaries from stored door-number coordinates
  transit    load rail stations (MRT, 台鐵, 高鐵) from TDX and measure sales and leases to them
  facilities load shops, schools, clinics, parks and nuisances (OpenStreetMap, NHI, MOE)
  cities     list supported cities

Run "importer <command> -h" for command flags. DATABASE_URL must point to PostgreSQL.
TDX_CLIENT_ID and TDX_CLIENT_SECRET hold the TDX API key used for rail stations;
TDX_BASE_URL overrides the API address.

Examples:
  importer addresses -city a,f,h
  importer sales -season 115S2 -city a,f,h
  importer sales -season 105S1-115S2 -city a,f,h    # backfill
  importer sales -current -city a,f,h               # latest 10-day release
  importer rentals -season 101S4-115S3 -city a,f,h  # backfill leases
  importer presales -season 101S4-115S3 -city a,f,h # backfill presales
  importer transit
  importer facilities -source osm,nhi,moe
  importer sync -city a,f,h -interval 24h
`

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	logger := slog.New(slog.NewTextHandler(os.Stderr, nil))
	if err := run(ctx, os.Args[1:], os.Stdout, logger); err != nil {
		if !errors.Is(err, flag.ErrHelp) {
			logger.Error("import failed", "error", err)
		}
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string, stdout io.Writer, logger *slog.Logger) error {
	if len(args) == 0 {
		fmt.Fprint(stdout, usage)
		return flag.ErrHelp
	}
	command, args := args[0], args[1:]
	switch command {
	case "cities":
		for _, code := range sortedCityCodes() {
			fmt.Fprintf(stdout, "%s  %s\n", code, lvr.Cities[code])
		}
		return nil
	case "addresses", "sales", "rentals", "presales", "sync", "boundaries", "transit", "facilities":
	case "-h", "--help", "help":
		fmt.Fprint(stdout, usage)
		return nil
	default:
		fmt.Fprint(stdout, usage)
		return fmt.Errorf("unknown command %q", command)
	}

	flags := flag.NewFlagSet(command, flag.ContinueOnError)
	flags.SetOutput(stdout)
	cities := flags.String("city", "a", "comma-separated 實價登錄 city codes, e.g. a,f,h")
	file := new(string) // sync always downloads; boundaries use stored data
	archive := command == "sales" || command == "rentals" || command == "presales"
	if command == "facilities" {
		file = flags.String("file", "", "read a local OpenStreetMap PBF extract instead of downloading it")
	}
	if command == "addresses" || archive {
		file = flags.String("file", "", "read a local file instead of downloading (CSV for addresses, zip for sales, rentals and presales)")
	}
	databaseURL := flags.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	var season *string
	var current *bool
	if archive {
		season = flags.String("season", "", "season such as 115S2, a list, or a range such as 105S1-115S2")
		current = flags.Bool("current", false, "import the latest 10-day release instead of seasons")
	}
	var interval, addressMaxAge, transitMaxAge, facilityMaxAge *time.Duration
	var sources *string
	if command == "facilities" {
		sources = flags.String("source", strings.Join(importer.FacilitySources, ","), "comma-separated facility sources: osm, nhi, moe")
	}
	if command == "sync" {
		interval = flags.Duration("interval", 0, "repeat the sync this often; 0 runs once")
		addressMaxAge = flags.Duration("address-max-age", 30*24*time.Hour, "refresh door-number coordinates older than this")
		transitMaxAge = flags.Duration("transit-max-age", 7*24*time.Hour, "refresh rail stations older than this")
		facilityMaxAge = flags.Duration("facility-max-age", 30*24*time.Hour, "refresh each facility source older than this; 0 skips them")
	}
	if err := flags.Parse(args); err != nil {
		if errors.Is(err, flag.ErrHelp) {
			return nil
		}
		return err
	}
	cityCodes, err := parseCities(*cities)
	if err != nil {
		return err
	}
	if command == "addresses" && *file != "" && len(cityCodes) > 1 {
		return errors.New("-file holds one city's addresses; pass a single -city")
	}
	var seasons []lvr.Season
	if archive {
		switch {
		case *current && *season != "":
			return errors.New("use either -season or -current")
		case !*current && *season == "":
			return errors.New("-season or -current is required, e.g. -season 115S2")
		case !*current:
			if seasons, err = lvr.ParseSeasons(*season); err != nil {
				return err
			}
		}
	}
	if *databaseURL == "" {
		return errors.New("DATABASE_URL or -database-url is required")
	}

	store, err := postgres.NewImportStore(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer store.Close()
	service := importer.NewService(store, opendata.NewClient(), transitSource(command == "transit", logger), logger)
	var publisher *usecase.QualityPublisher
	if target := os.Getenv("QUALITY_WEBHOOK_URL"); target != "" {
		webhook, err := usecase.NewQualityWebhook(target, os.Getenv("QUALITY_WEBHOOK_TOKEN"))
		if err != nil {
			return err
		}
		qualityStore, err := postgres.NewTransactionRepository(ctx, *databaseURL)
		if err != nil {
			return err
		}
		defer qualityStore.Close()
		publisher = usecase.NewQualityPublisher(qualityStore, webhook)
	}

	switch {
	case command == "transit":
		return service.ImportTransit(ctx)
	case command == "facilities":
		return service.ImportFacilities(ctx, strings.Split(*sources, ","), *file)
	case command == "boundaries":
		for _, cityCode := range cityCodes {
			if err = service.ImportBoundaries(ctx, cityCode); err != nil {
				return fmt.Errorf("city %s: %w", cityCode, err)
			}
		}
		return nil
	case command == "sync":
		jobs := &importer.Jobs{
			Service: service, Store: store, Cities: cityCodes,
			Options: importer.SyncOptions{AddressMaxAge: *addressMaxAge, TransitMaxAge: *transitMaxAge, FacilityMaxAge: *facilityMaxAge},
		}
		return runSync(ctx, service, jobs, publisher, *interval, logger)
	case archive && *current:
		return service.ImportCurrentRelease(ctx, cityCodes, []importer.Dataset{importer.Dataset(command)}, *file, false)
	case archive:
		return service.ImportSeasons(ctx, seasons, cityCodes, []importer.Dataset{importer.Dataset(command)}, *file)
	}
	for _, cityCode := range cityCodes {
		if _, err = service.ImportAddressPoints(ctx, cityCode, *file); err != nil {
			return fmt.Errorf("city %s: %w", cityCode, err)
		}
	}
	return nil
}

// jobPollInterval is how often the scheduler checks for jobs queued in the admin console.
const jobPollInterval = 15 * time.Second

// runSync syncs once, or every interval until ctx is cancelled, running admin console
// jobs in between. In repeat mode a failed sync or job is logged and the scheduler goes on.
func runSync(ctx context.Context, service *importer.Service, jobs *importer.Jobs, publisher *usecase.QualityPublisher, interval time.Duration, logger *slog.Logger) error {
	if interval <= 0 {
		syncErr := service.Sync(ctx, jobs.Cities, jobs.Options)
		if publisher != nil {
			return errors.Join(syncErr, publisher.Check(ctx))
		}
		return syncErr
	}
	if err := jobs.RecoverInterrupted(ctx); err != nil {
		logger.Error("could not fail interrupted jobs", "error", err)
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	poll := time.NewTicker(jobPollInterval)
	defer poll.Stop()
	qualityPoll := time.NewTicker(5 * time.Minute)
	defer qualityPoll.Stop()
	for {
		if err := service.Sync(ctx, jobs.Cities, jobs.Options); err != nil && ctx.Err() == nil {
			logger.Error("sync failed; retrying at next interval", "error", err, "interval", interval)
		}
		if publisher != nil && ctx.Err() == nil {
			if err := publisher.Check(ctx); err != nil {
				logger.Error("quality webhook check failed", "error", err)
			}
		}
		logger.Info("sync finished", "next", time.Now().Add(interval).Format(time.RFC3339))
	wait:
		for {
			select {
			case <-ctx.Done():
				return nil
			case <-ticker.C:
				break wait
			case <-poll.C:
				if err := jobs.RunQueued(ctx); err != nil && ctx.Err() == nil {
					logger.Error("admin job failed", "error", err)
				}
			case <-qualityPoll.C:
				if publisher != nil {
					if err := publisher.Check(ctx); err != nil && ctx.Err() == nil {
						logger.Error("quality webhook check failed", "error", err)
					}
				}
			}
		}
	}
}

// transitSource reads TDX settings from the environment. Scheduled syncs skip rail
// stations until a key is set; an explicit transit import tries anonymously, which TDX
// answers with an error naming the missing key.
func transitSource(explicit bool, logger *slog.Logger) importer.TransitSource {
	clientID, baseURL := os.Getenv("TDX_CLIENT_ID"), os.Getenv("TDX_BASE_URL")
	if clientID == "" && baseURL == "" && !explicit {
		return nil
	}
	client := transit.NewClient(clientID, os.Getenv("TDX_CLIENT_SECRET"))
	client.Logger = logger
	if baseURL != "" {
		client.BaseURL = baseURL
	}
	return func(ctx context.Context) (transit.Network, error) { return transit.Load(ctx, client) }
}

func parseCities(value string) ([]string, error) {
	var codes []string
	for _, code := range strings.Split(value, ",") {
		code = strings.TrimSpace(code)
		if code == "" {
			continue
		}
		if _, ok := addresspoint.Sources[code]; !ok {
			return nil, fmt.Errorf("city %q is not supported; supported: %s", code, strings.Join(sortedCityCodes(), ","))
		}
		if !slices.Contains(codes, code) {
			codes = append(codes, code)
		}
	}
	if len(codes) == 0 {
		return nil, errors.New("-city is required")
	}
	return codes, nil
}

func sortedCityCodes() []string {
	codes := make([]string, 0, len(addresspoint.Sources))
	for code := range addresspoint.Sources {
		codes = append(codes, code)
	}
	slices.Sort(codes)
	return codes
}
