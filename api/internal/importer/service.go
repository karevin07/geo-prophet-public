// Package importer loads open data (door-number coordinates, 實價登錄 sales, leases and
// presales, and rail stations) into storage.
package importer

import (
	"archive/zip"
	"bytes"
	"context"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"log/slog"
	"os"
	"slices"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/addresspoint"
	"github.com/geo-prophet/geo-prophet/api/internal/districtmap"
	"github.com/geo-prophet/geo-prophet/api/internal/domain"
	"github.com/geo-prophet/geo-prophet/api/internal/geocode"
	"github.com/geo-prophet/geo-prophet/api/internal/lvr"
	"github.com/geo-prophet/geo-prophet/api/internal/transit"
)

// Store persists import results. Implementations must make each replace or upsert atomic.
type Store interface {
	StartRun(ctx context.Context, run domain.ImportRun) (int64, error)
	FinishRun(ctx context.Context, runID int64, stats domain.ImportStats, runErr error) error
	// LastSucceeded returns when a matching run last succeeded; an empty season matches any.
	LastSucceeded(ctx context.Context, run domain.ImportRun) (time.Time, bool, error)
	ReplaceAddressPoints(ctx context.Context, cityCode string, points []addresspoint.Point) error
	LoadAddressPoints(ctx context.Context, cityCode string) ([]addresspoint.Point, error)
	ReplaceDistrictBoundaries(ctx context.Context, cityCode string, boundaries []domain.DistrictBoundary) error
	HasDistrictBoundaries(ctx context.Context, cityCode string) (bool, error)
	UpsertTransactions(ctx context.Context, runID int64, transactions []domain.Transaction) error
	UpsertRentals(ctx context.Context, runID int64, rentals []domain.Rental) error
	UpsertPresales(ctx context.Context, runID int64, presales []domain.Presale) error
	ReplaceFacilities(ctx context.Context, runID int64, source string, facilities []domain.Facility) error
	// ReplaceTransit replaces the rail network and returns how many sale–station pairs
	// lie within 800 m.
	ReplaceTransit(ctx context.Context, network transit.Network) (int, error)
}

// TransitSource loads the rail network, normally transit.Load with a TDX client.
type TransitSource func(ctx context.Context) (transit.Network, error)

// Fetcher downloads open data files.
type Fetcher interface {
	LatestCSVURL(ctx context.Context, datasetID string) (string, error)
	LVRSeasonURL(season string) string
	LVRCurrentURL() string
	OSMExtractURL() string
	Open(ctx context.Context, target string) (io.ReadCloser, error)
}

var (
	ErrNoAddressPoints   = errors.New("no address points loaded for city; run the addresses import first")
	ErrSeasonUnavailable = errors.New("season archive is not published")
)

const (
	sourceAddressPoint = "addresspoint"
	sourceLVR          = "lvr"
	sourceTDX          = "tdx"
	datasetDoorNumbers = "door_numbers"
	datasetBoundaries  = "district_boundaries"
	datasetTransit     = "transit"
)

// Dataset is a kind of 實價登錄 file inside each archive. A season's run is recorded under
// the dataset's name, a 10-day release's under the name with a _release suffix.
type Dataset string

const (
	DatasetSales    Dataset = "sales"
	DatasetRentals  Dataset = "rentals"
	DatasetPresales Dataset = "presales"
)

// Datasets lists every dataset imported from the archives.
var Datasets = []Dataset{DatasetSales, DatasetRentals, DatasetPresales}

func (dataset Dataset) releaseName() string { return string(dataset) + "_release" }

type Service struct {
	store   Store
	fetcher Fetcher
	transit TransitSource
	logger  *slog.Logger
	now     func() time.Time
}

// NewService creates an importer; transit may be nil when TDX is not configured.
func NewService(store Store, fetcher Fetcher, transit TransitSource, logger *slog.Logger) *Service {
	return &Service{store: store, fetcher: fetcher, transit: transit, logger: logger, now: time.Now}
}

// SyncOptions sets how often Sync refreshes slowly changing datasets.
type SyncOptions struct {
	AddressMaxAge  time.Duration
	TransitMaxAge  time.Duration
	FacilityMaxAge time.Duration
}

// ErrTransitNotConfigured means no TDX source was given.
var ErrTransitNotConfigured = errors.New("TDX is not configured; set TDX_CLIENT_ID and TDX_CLIENT_SECRET")

// ImportTransit replaces the rail network and re-measures every sale's distance to it.
func (service *Service) ImportTransit(ctx context.Context) error {
	if service.transit == nil {
		return ErrTransitNotConfigured
	}
	run := domain.ImportRun{Source: sourceTDX, Dataset: datasetTransit, Season: service.now().Format("2006-01")}
	_, err := service.record(ctx, run, func(int64) (domain.ImportStats, error) {
		network, err := service.transit(ctx)
		if err != nil {
			return domain.ImportStats{}, err
		}
		stats := domain.ImportStats{RowsRead: len(network.Stations), Detail: map[string]int{"lines": len(network.Lines)}}
		for _, station := range network.Stations {
			stats.Detail["platforms"] += len(station.Platforms)
			stats.Detail["exits"] += len(station.Exits())
		}
		if len(network.Stations) == 0 {
			return stats, errors.New("TDX returned no stations in the covered cities")
		}
		pairs, err := service.store.ReplaceTransit(ctx, network)
		stats.RowsUpserted, stats.Detail["sale_station_pairs"] = len(network.Stations), pairs
		return stats, err
	})
	return err
}

// ImportAddressPoints replaces a city's door-number coordinates. When localFile is empty
// the newest file is downloaded from data.gov.tw.
func (service *Service) ImportAddressPoints(ctx context.Context, cityCode, localFile string) (domain.ImportStats, error) {
	source, ok := addresspoint.Sources[cityCode]
	if !ok {
		return domain.ImportStats{}, fmt.Errorf("address points are not supported for city %q", cityCode)
	}
	run := domain.ImportRun{Source: sourceAddressPoint, Dataset: datasetDoorNumbers, Season: service.now().Format("2006-01"), CityCode: cityCode}
	return service.record(ctx, run, func(runID int64) (domain.ImportStats, error) {
		body, err := service.openSource(ctx, localFile, func() (string, error) {
			return service.fetcher.LatestCSVURL(ctx, source.DatasetID)
		})
		if err != nil {
			return domain.ImportStats{}, err
		}
		defer body.Close()

		result, err := addresspoint.Parse(body)
		if err != nil {
			return domain.ImportStats{}, fmt.Errorf("parse address points: %w", err)
		}
		stats := domain.ImportStats{RowsRead: result.Rows, Detail: map[string]int{}}
		for reason, count := range result.Skipped {
			stats.RowsSkipped += count
			stats.Detail["skipped_"+string(reason)] = count
		}
		if len(result.Points) == 0 {
			return stats, errors.New("address point file contains no usable rows")
		}
		if err = service.store.ReplaceAddressPoints(ctx, cityCode, result.Points); err != nil {
			return stats, err
		}
		stats.RowsUpserted = len(result.Points)
		districts, err := service.replaceBoundaries(ctx, cityCode, result.Points)
		stats.Detail["district_boundaries"] = districts
		return stats, err
	})
}

// ImportBoundaries recomputes a city's district boundaries from its stored door numbers.
func (service *Service) ImportBoundaries(ctx context.Context, cityCode string) error {
	if err := validateCities([]string{cityCode}); err != nil {
		return err
	}
	run := domain.ImportRun{Source: sourceAddressPoint, Dataset: datasetBoundaries, Season: service.now().Format("2006-01"), CityCode: cityCode}
	_, err := service.record(ctx, run, func(int64) (domain.ImportStats, error) {
		points, err := service.store.LoadAddressPoints(ctx, cityCode)
		if err != nil {
			return domain.ImportStats{}, err
		}
		if len(points) == 0 {
			return domain.ImportStats{}, ErrNoAddressPoints
		}
		districts, err := service.replaceBoundaries(ctx, cityCode, points)
		return domain.ImportStats{RowsRead: len(points), RowsUpserted: districts, Detail: map[string]int{}}, err
	})
	return err
}

func (service *Service) replaceBoundaries(ctx context.Context, cityCode string, points []addresspoint.Point) (int, error) {
	boundaries, err := districtmap.Build(points, addresspoint.Sources[cityCode].Districts)
	if err != nil {
		return 0, err
	}
	if err = service.store.ReplaceDistrictBoundaries(ctx, cityCode, boundaries); err != nil {
		return 0, err
	}
	return len(boundaries), nil
}

// ImportSeasons imports datasets from 實價登錄 season archives, downloading each once
// for all cities. In a multi-season backfill, unpublished seasons are skipped with a
// warning; otherwise it stops at the first failure and earlier seasons stay imported.
func (service *Service) ImportSeasons(ctx context.Context, seasons []lvr.Season, cityCodes []string, datasets []Dataset, localFile string) error {
	if err := validateCities(cityCodes); err != nil {
		return err
	}
	if err := validateDatasets(datasets); err != nil {
		return err
	}
	if localFile != "" && len(seasons) != 1 {
		return errors.New("a local archive holds exactly one season")
	}
	matchers := service.newMatcherCache()
	for _, season := range seasons {
		archive, err := service.openArchive(ctx, localFile, service.fetcher.LVRSeasonURL(season.String()))
		if errors.Is(err, ErrSeasonUnavailable) && len(seasons) > 1 {
			service.logger.Warn("skipping unpublished season", "season", season)
			continue
		}
		if err != nil {
			return fmt.Errorf("season %s: %w", season, err)
		}
		for _, dataset := range datasets {
			for _, cityCode := range cityCodes {
				run := domain.ImportRun{Source: sourceLVR, Dataset: string(dataset), Season: season.String(), CityCode: cityCode}
				if err = service.importArchive(ctx, run, dataset, archive, season, matchers); err != nil {
					return fmt.Errorf("season %s %s city %s: %w", season, dataset, cityCode, err)
				}
			}
		}
	}
	return nil
}

// ImportCurrentRelease imports datasets from the latest 10-day release. With
// skipImported, datasets and cities that already imported this release are left alone,
// so it is safe to run on a schedule.
func (service *Service) ImportCurrentRelease(ctx context.Context, cityCodes []string, datasets []Dataset, localFile string, skipImported bool) error {
	if err := validateCities(cityCodes); err != nil {
		return err
	}
	if err := validateDatasets(datasets); err != nil {
		return err
	}
	archive, err := service.openArchive(ctx, localFile, service.fetcher.LVRCurrentURL())
	if err != nil {
		return fmt.Errorf("current release: %w", err)
	}
	release, err := readRelease(archive)
	if err != nil {
		return err
	}
	matchers := service.newMatcherCache()
	for _, dataset := range datasets {
		for _, cityCode := range cityCodes {
			run := domain.ImportRun{Source: sourceLVR, Dataset: dataset.releaseName(), Season: release.Label(), CityCode: cityCode}
			if skipImported {
				if _, done, err := service.store.LastSucceeded(ctx, run); err != nil {
					return err
				} else if done {
					service.logger.Info("release already imported", "release", release.Label(), "dataset", dataset, "city", cityCode)
					continue
				}
			}
			if err = service.importArchive(ctx, run, dataset, archive, release.Season(), matchers); err != nil {
				return fmt.Errorf("release %s %s city %s: %w", release.Label(), dataset, cityCode, err)
			}
		}
	}
	return nil
}

// Sync brings the data up to date and is idempotent, so it can run on any schedule:
//  1. refresh door-number coordinates older than addressMaxAge (and derive district
//     boundaries from them if a city has none yet);
//  2. import each dataset of the newest published season archive once (it carries
//     official corrections);
//  3. import each dataset of the latest 10-day release once;
//  4. refresh rail stations older than TransitMaxAge, when TDX is configured;
//  5. refresh each facility source older than FacilityMaxAge, when set.
//
// A failing step is reported but does not stop the later ones.
func (service *Service) Sync(ctx context.Context, cityCodes []string, options SyncOptions) error {
	if err := validateCities(cityCodes); err != nil {
		return err
	}
	var errs []error
	for _, cityCode := range cityCodes {
		run := domain.ImportRun{Source: sourceAddressPoint, Dataset: datasetDoorNumbers, CityCode: cityCode}
		last, done, err := service.store.LastSucceeded(ctx, run)
		if err == nil && (!done || service.now().Sub(last) > options.AddressMaxAge) {
			_, err = service.ImportAddressPoints(ctx, cityCode, "")
		}
		if err != nil {
			errs = append(errs, fmt.Errorf("addresses %s: %w", cityCode, err))
			continue
		}
		// Databases filled before boundaries existed get them from the stored door numbers.
		if has, err := service.store.HasDistrictBoundaries(ctx, cityCode); err != nil || !has {
			if err == nil {
				err = service.ImportBoundaries(ctx, cityCode)
			}
			if err != nil {
				errs = append(errs, fmt.Errorf("district boundaries %s: %w", cityCode, err))
			}
		}
	}
	if err := service.syncLatestSeason(ctx, cityCodes); err != nil {
		errs = append(errs, err)
	}
	if err := service.ImportCurrentRelease(ctx, cityCodes, Datasets, "", true); err != nil {
		errs = append(errs, err)
	}
	if err := service.syncTransit(ctx, options.TransitMaxAge); err != nil {
		errs = append(errs, fmt.Errorf("transit: %w", err))
	}
	if options.FacilityMaxAge > 0 {
		if err := service.syncFacilities(ctx, options.FacilityMaxAge); err != nil {
			errs = append(errs, err)
		}
	}
	return errors.Join(errs...)
}

// syncTransit refreshes the rail network when it is older than maxAge. Stations change
// rarely, and sales imported meanwhile are measured to the stored network as they arrive.
func (service *Service) syncTransit(ctx context.Context, maxAge time.Duration) error {
	if service.transit == nil {
		service.logger.Warn("skipping rail stations: " + ErrTransitNotConfigured.Error())
		return nil
	}
	last, done, err := service.store.LastSucceeded(ctx, domain.ImportRun{Source: sourceTDX, Dataset: datasetTransit})
	if err != nil || (done && service.now().Sub(last) <= maxAge) {
		return err
	}
	return service.ImportTransit(ctx)
}

// syncLatestSeason walks back from the season in progress to the newest published archive
// and imports it for the datasets and cities that lack it. It stops at the first season
// already imported.
func (service *Service) syncLatestSeason(ctx context.Context, cityCodes []string) error {
	season := lvr.SeasonOfRegistration(service.now())
	for range 3 {
		pending := map[Dataset][]string{}
		for _, dataset := range Datasets {
			for _, cityCode := range cityCodes {
				run := domain.ImportRun{Source: sourceLVR, Dataset: string(dataset), Season: season.String(), CityCode: cityCode}
				if _, done, err := service.store.LastSucceeded(ctx, run); err != nil {
					return err
				} else if !done {
					pending[dataset] = append(pending[dataset], cityCode)
				}
			}
		}
		if len(pending) == 0 {
			return nil
		}
		var err error
		for _, dataset := range Datasets {
			if len(pending[dataset]) == 0 {
				continue
			}
			if err = service.ImportSeasons(ctx, []lvr.Season{season}, pending[dataset], []Dataset{dataset}, ""); err != nil {
				break
			}
		}
		if !errors.Is(err, ErrSeasonUnavailable) {
			return err
		}
		season = season.Previous()
	}
	return nil
}

// matcherCache builds each city's geocoder once per import, which matters for backfills.
type matcherCache func(ctx context.Context, cityCode string) (*geocode.Matcher, error)

func (service *Service) newMatcherCache() matcherCache {
	matchers := map[string]*geocode.Matcher{}
	return func(ctx context.Context, cityCode string) (*geocode.Matcher, error) {
		if matcher, ok := matchers[cityCode]; ok {
			return matcher, nil
		}
		points, err := service.store.LoadAddressPoints(ctx, cityCode)
		if err != nil {
			return nil, err
		}
		if len(points) == 0 {
			return nil, ErrNoAddressPoints
		}
		matchers[cityCode] = geocode.NewMatcher(lvr.Cities[cityCode], addresspoint.Sources[cityCode].Districts, points)
		return matchers[cityCode], nil
	}
}

func (service *Service) importArchive(ctx context.Context, run domain.ImportRun, dataset Dataset, archive *zip.Reader, season lvr.Season, matchers matcherCache) error {
	_, err := service.record(ctx, run, func(runID int64) (domain.ImportStats, error) {
		switch dataset {
		case DatasetRentals:
			return service.importCityRentals(ctx, runID, archive, run.CityCode, season, matchers)
		case DatasetPresales:
			return service.importCityPresales(ctx, runID, archive, run.CityCode, season, matchers)
		}
		return service.importCitySales(ctx, runID, archive, run.CityCode, season, matchers)
	})
	return err
}

// cityFile imports one dataset's file of a city from an archive: open it, parse and flag
// its rows, geocode each item and store them.
type cityFile[T any] struct {
	open   func(*zip.Reader, string) (io.ReadCloser, error)
	parse  func(io.Reader, string, lvr.Season) (items []T, rows int, skipped map[lvr.SkipReason]int, err error)
	place  func(item *T) (district, address string)
	locate func(item *T, location geocode.Location)
	store  func(ctx context.Context, runID int64, items []T) error
}

func importCity[T any](ctx context.Context, runID int64, archive *zip.Reader, cityCode string, season lvr.Season, matchers matcherCache, file cityFile[T]) (domain.ImportStats, error) {
	reader, err := file.open(archive, cityCode)
	if errors.Is(err, fs.ErrNotExist) {
		// The earliest seasons only cover some cities, and have no leases or presales.
		return domain.ImportStats{Detail: map[string]int{"missing_file": 1}}, nil
	}
	if err != nil {
		return domain.ImportStats{}, err
	}
	defer reader.Close()
	matcher, err := matchers(ctx, cityCode)
	if err != nil {
		return domain.ImportStats{}, err
	}
	items, rows, skipped, err := file.parse(reader, cityCode, season)
	if err != nil {
		return domain.ImportStats{}, err
	}

	stats := domain.ImportStats{RowsRead: rows, Detail: map[string]int{}}
	for reason, count := range skipped {
		stats.RowsSkipped += count
		stats.Detail["skipped_"+string(reason)] = count
	}
	for index := range items {
		location, ok := matcher.Locate(file.place(&items[index]))
		if !ok {
			stats.RowsGeocodeFailed++
			continue
		}
		file.locate(&items[index], location)
		stats.Detail["geocode_"+string(location.Precision)]++
	}

	if err = file.store(ctx, runID, items); err != nil {
		return stats, err
	}
	stats.RowsUpserted = len(items)
	return stats, nil
}

func (service *Service) importCitySales(ctx context.Context, runID int64, archive *zip.Reader, cityCode string, season lvr.Season, matchers matcherCache) (domain.ImportStats, error) {
	return importCity(ctx, runID, archive, cityCode, season, matchers, cityFile[domain.Transaction]{
		open: lvr.OpenSales,
		parse: func(reader io.Reader, cityCode string, season lvr.Season) ([]domain.Transaction, int, map[lvr.SkipReason]int, error) {
			result, err := lvr.ParseSales(reader, cityCode, season.String())
			if err != nil {
				return nil, 0, nil, err
			}
			// Mark bad dates first so they stay out of the outlier quartiles.
			_, registeredTo := season.RegistrationWindow()
			lvr.MarkInvalidDates(result.Transactions, registeredTo)
			lvr.MarkPriceOutliers(result.Transactions)
			return result.Transactions, result.Rows, result.Skipped, nil
		},
		place: func(sale *domain.Transaction) (string, string) { return sale.District, sale.Address },
		locate: func(sale *domain.Transaction, location geocode.Location) {
			sale.Lat, sale.Lng, sale.GeocodePrecision = location.Lat, location.Lng, string(location.Precision)
		},
		store: service.store.UpsertTransactions,
	})
}

func (service *Service) importCityRentals(ctx context.Context, runID int64, archive *zip.Reader, cityCode string, season lvr.Season, matchers matcherCache) (domain.ImportStats, error) {
	return importCity(ctx, runID, archive, cityCode, season, matchers, cityFile[domain.Rental]{
		open: lvr.OpenRentals,
		parse: func(reader io.Reader, cityCode string, season lvr.Season) ([]domain.Rental, int, map[lvr.SkipReason]int, error) {
			result, err := lvr.ParseRentals(reader, cityCode, season.String())
			if err != nil {
				return nil, 0, nil, err
			}
			_, registeredTo := season.RegistrationWindow()
			lvr.MarkInvalidLeaseDates(result.Rentals, registeredTo)
			lvr.MarkRentOutliers(result.Rentals)
			return result.Rentals, result.Rows, result.Skipped, nil
		},
		place: func(rental *domain.Rental) (string, string) { return rental.District, rental.Address },
		locate: func(rental *domain.Rental, location geocode.Location) {
			rental.Lat, rental.Lng, rental.GeocodePrecision = location.Lat, location.Lng, string(location.Precision)
		},
		store: service.store.UpsertRentals,
	})
}

func (service *Service) importCityPresales(ctx context.Context, runID int64, archive *zip.Reader, cityCode string, season lvr.Season, matchers matcherCache) (domain.ImportStats, error) {
	return importCity(ctx, runID, archive, cityCode, season, matchers, cityFile[domain.Presale]{
		open: lvr.OpenPresales,
		parse: func(reader io.Reader, cityCode string, season lvr.Season) ([]domain.Presale, int, map[lvr.SkipReason]int, error) {
			result, err := lvr.ParsePresales(reader, cityCode, season.String())
			if err != nil {
				return nil, 0, nil, err
			}
			_, registeredTo := season.RegistrationWindow()
			lvr.MarkPresaleOutliers(result.Presales, registeredTo)
			return result.Presales, result.Rows, result.Skipped, nil
		},
		place: func(presale *domain.Presale) (string, string) { return presale.District, presale.Address },
		locate: func(presale *domain.Presale, location geocode.Location) {
			presale.Lat, presale.Lng, presale.GeocodePrecision = location.Lat, location.Lng, string(location.Precision)
		},
		store: service.store.UpsertPresales,
	})
}

// record wraps work in an import_runs row so failures are auditable too.
func (service *Service) record(ctx context.Context, run domain.ImportRun, work func(runID int64) (domain.ImportStats, error)) (domain.ImportStats, error) {
	runID, err := service.store.StartRun(ctx, run)
	if err != nil {
		return domain.ImportStats{}, err
	}
	started := service.now()
	stats, workErr := work(runID)
	// Record the outcome even when ctx was cancelled mid-run.
	finishCtx, cancel := context.WithTimeout(context.WithoutCancel(ctx), 10*time.Second)
	defer cancel()
	if err = service.store.FinishRun(finishCtx, runID, stats, workErr); err != nil {
		return stats, errors.Join(workErr, err)
	}
	if workErr != nil {
		return stats, workErr
	}
	service.logger.Info("import finished", "source", run.Source, "dataset", run.Dataset, "season", run.Season,
		"city", run.CityCode, "read", stats.RowsRead, "skipped", stats.RowsSkipped, "upserted", stats.RowsUpserted,
		"geocode_failed", stats.RowsGeocodeFailed, "detail", stats.Detail, "elapsed", service.now().Sub(started).Round(time.Millisecond))
	return stats, nil
}

func (service *Service) openSource(ctx context.Context, localFile string, resolveURL func() (string, error)) (io.ReadCloser, error) {
	if localFile != "" {
		return os.Open(localFile)
	}
	target, err := resolveURL()
	if err != nil {
		return nil, err
	}
	service.logger.Info("downloading", "url", target)
	return service.fetcher.Open(ctx, target)
}

// openArchive loads a 實價登錄 archive into memory; one holds about 15MB for all cities.
func (service *Service) openArchive(ctx context.Context, localFile, target string) (*zip.Reader, error) {
	body, err := service.openSource(ctx, localFile, func() (string, error) { return target, nil })
	if err != nil {
		return nil, err
	}
	defer body.Close()
	content, err := io.ReadAll(body)
	if err != nil {
		return nil, fmt.Errorf("read archive: %w", err)
	}
	// An unpublished season is answered with 200 and an HTML page.
	if !bytes.HasPrefix(content, []byte("PK")) {
		return nil, ErrSeasonUnavailable
	}
	archive, err := zip.NewReader(bytes.NewReader(content), int64(len(content)))
	if err != nil {
		return nil, fmt.Errorf("open archive: %w", err)
	}
	return archive, nil
}

func readRelease(archive *zip.Reader) (lvr.Release, error) {
	file, err := archive.Open("build_time.xml")
	if err != nil {
		return lvr.Release{}, fmt.Errorf("open build_time.xml: %w", err)
	}
	defer file.Close()
	content, err := io.ReadAll(file)
	if err != nil {
		return lvr.Release{}, fmt.Errorf("read build_time.xml: %w", err)
	}
	return lvr.ParseRelease(content)
}

func validateDatasets(datasets []Dataset) error {
	if len(datasets) == 0 {
		return errors.New("no dataset given")
	}
	for _, dataset := range datasets {
		if !slices.Contains(Datasets, dataset) {
			return fmt.Errorf("unknown dataset %q", dataset)
		}
	}
	return nil
}

func validateCities(cityCodes []string) error {
	for _, cityCode := range cityCodes {
		if _, ok := addresspoint.Sources[cityCode]; !ok {
			return fmt.Errorf("city %q is not supported", cityCode)
		}
	}
	return nil
}
