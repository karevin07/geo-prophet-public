// Command snapshot writes the static data of the online demo from the database; see
// internal/snapshot and docs/DEMO.md.
package main

import (
	"context"
	"flag"
	"fmt"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	delivery "github.com/geo-prophet/geo-prophet/api/internal/delivery/http"
	"github.com/geo-prophet/geo-prophet/api/internal/infra/postgres"
	"github.com/geo-prophet/geo-prophet/api/internal/snapshot"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
	"github.com/gin-gonic/gin"
)

func main() {
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()
	if err := run(ctx, os.Args[1:]); err != nil {
		slog.Error("snapshot failed", "error", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, args []string) error {
	flags := flag.NewFlagSet("snapshot", flag.ContinueOnError)
	out := flags.String("out", "build/demo-data", "directory to write the snapshot into")
	city := flags.String("city", "a", "實價登錄 city code")
	months := flags.Int("months", 36, "months of sales to keep, ending with the current month")
	databaseURL := flags.String("database-url", os.Getenv("DATABASE_URL"), "PostgreSQL connection string")
	if err := flags.Parse(args); err != nil {
		return err
	}
	if *databaseURL == "" {
		return fmt.Errorf("DATABASE_URL or -database-url is required")
	}

	gin.SetMode(gin.ReleaseMode)
	repository, err := postgres.NewTransactionRepository(ctx, *databaseURL)
	if err != nil {
		return err
	}
	defer repository.Close()
	transactions := usecase.NewTransactionService(repository)
	handler := delivery.NewHandler(delivery.Services{
		Transactions: transactions, Places: usecase.NewPlaceService(repository), Feed: usecase.NewFeedService(repository),
		Investment: usecase.NewInvestmentService(repository), Rentals: usecase.NewRentalService(repository),
		Presales: usecase.NewPresaleService(repository), Facilities: usecase.NewFacilityService(repository),
		Transit: usecase.NewTransitService(repository, transactions),
	})
	meta, err := snapshot.Write(ctx, *out, repository, snapshot.Options{
		CityCode: *city, Months: *months, Now: time.Now(), CompleteMonth: transactions.CompleteMonth,
		// Requests are served in process and carry no Origin; the router needs one to allow.
		API: delivery.NewRouter(handler, []string{"http://localhost"}),
	})
	if err != nil {
		return err
	}
	slog.Info("snapshot written", "out", *out, "sales", meta.Sales, "from", meta.From, "through", meta.Through,
		"complete_month", meta.CompleteMonth, "responses", meta.Responses)
	return nil
}
