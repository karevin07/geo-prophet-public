package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/geo-prophet/geo-prophet/api/internal/config"
	delivery "github.com/geo-prophet/geo-prophet/api/internal/delivery/http"
	"github.com/geo-prophet/geo-prophet/api/internal/infra/backupdir"
	"github.com/geo-prophet/geo-prophet/api/internal/infra/mock"
	"github.com/geo-prophet/geo-prophet/api/internal/infra/postgres"
	"github.com/geo-prophet/geo-prophet/api/internal/usecase"
)

func main() {
	configuration := config.Load()
	ctx, cancel := signal.NotifyContext(context.Background(), syscall.SIGINT, syscall.SIGTERM)
	defer cancel()

	var repository interface {
		usecase.TransactionRepository
		usecase.PlaceRepository
		usecase.FeedRepository
		usecase.TransitRepository
		usecase.InvestmentRepository
	}
	var rentals *usecase.RentalService
	var workbench *usecase.InvestmentWorkbenchService
	var liquidity *usecase.InvestmentLiquidityService
	var estimates *usecase.EstimateService
	var presales *usecase.PresaleService
	var communities *usecase.CommunityService
	var facilities *usecase.FacilityService
	var dataStatus *usecase.DataStatusService
	var admin *usecase.AdminService
	var adminSettings *usecase.AdminSettings
	var adminRepository usecase.AdminRepository
	if configuration.DatabaseURL == "" {
		slog.Warn("DATABASE_URL is empty; using deterministic mock repository")
		repository = mock.NewTransactionRepository(3000, time.Now())
	} else {
		postgresRepository, err := postgres.NewTransactionRepository(ctx, configuration.DatabaseURL)
		if err != nil {
			slog.Error("database initialization failed", "error", err)
			os.Exit(1)
		}
		defer postgresRepository.Close()
		repository = postgresRepository
		rentals = usecase.NewRentalService(postgresRepository)
		workbench = usecase.NewInvestmentWorkbenchService(postgresRepository)
		liquidity = usecase.NewInvestmentLiquidityService(postgresRepository)
		estimates = usecase.NewEstimateService(postgresRepository)
		presales = usecase.NewPresaleService(postgresRepository)
		communities = usecase.NewCommunityService(postgresRepository)
		facilities = usecase.NewFacilityService(postgresRepository)
		dataStatus = usecase.NewDataStatusService(postgresRepository)
		settings := usecase.AdminSettings{
			Logins: configuration.AdminLogins, BackupKeep: configuration.BackupKeep, BackupIntervalHours: configuration.BackupIntervalHours,
		}
		if configuration.BackupDir != "" {
			settings.Backups = backupdir.Dir{Path: configuration.BackupDir}
		}
		if len(configuration.AdminLogins) == 0 {
			slog.Warn("ADMIN_LOGINS is empty; nobody can open the admin console")
		}
		// Built once the transit service exists, so opening edits can clear its cache.
		adminSettings, adminRepository = &settings, postgresRepository
	}

	transactions := usecase.NewTransactionService(repository)
	transit := usecase.NewTransitService(repository, transactions)
	if adminSettings != nil {
		adminSettings.OpeningsChanged = transit.InvalidateOpenings
		admin = usecase.NewAdminService(adminRepository, *adminSettings)
	}
	handler := delivery.NewHandler(delivery.Services{
		Transactions: transactions, Places: usecase.NewPlaceService(repository), Feed: usecase.NewFeedService(repository),
		Investment: usecase.NewInvestmentService(repository), Workbench: workbench, Liquidity: liquidity, Rentals: rentals, Estimates: estimates, Presales: presales, Communities: communities, Facilities: facilities, Transit: transit, DataStatus: dataStatus, Admin: admin,
	})
	server := &http.Server{
		Addr:              configuration.Address,
		Handler:           delivery.NewRouter(handler, configuration.AllowedOrigins),
		ReadHeaderTimeout: 5 * time.Second,
		ReadTimeout:       15 * time.Second,
		WriteTimeout:      30 * time.Second,
		IdleTimeout:       60 * time.Second,
	}

	go func() {
		slog.Info("api listening", "address", configuration.Address)
		if err := server.ListenAndServe(); err != nil && !errors.Is(err, http.ErrServerClosed) {
			slog.Error("api server failed", "error", err)
			cancel()
		}
	}()

	<-ctx.Done()
	shutdownCtx, shutdownCancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownCancel()
	if err := server.Shutdown(shutdownCtx); err != nil {
		slog.Error("graceful shutdown failed", "error", err)
	}
}
