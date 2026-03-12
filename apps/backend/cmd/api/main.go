package main

import (
	"context"
	"database/sql"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	_ "github.com/lib/pq"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	httpapi "github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/ingest"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/postgres"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/retention"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/security"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/worker"
)

func main() {
	logger := slog.New(slog.NewJSONHandler(os.Stdout, &slog.HandlerOptions{Level: slog.LevelInfo}))

	cfg, err := config.LoadFromEnv()
	if err != nil {
		logger.Error("failed to load configuration", "error", err)
		os.Exit(1)
	}

	deps, ingestService, retentionService, cleanup, err := buildDependencies(cfg)
	if err != nil {
		logger.Error("failed to initialize repositories", "error", err)
		os.Exit(1)
	}
	defer cleanup()

	runtimeCtx, runtimeCancel := context.WithCancel(context.Background())
	defer runtimeCancel()
	go worker.NewRetryWorker(
		ingestService,
		logger,
		cfg.IngestRetryWorkerInterval,
		cfg.IngestRetryBatchSize,
	).Run(runtimeCtx)
	if cfg.Retention.Enabled {
		go worker.NewRetentionWorker(retentionService, logger, cfg.Retention.Interval).Run(runtimeCtx)
	}

	srv := &http.Server{
		Addr:         cfg.Addr,
		Handler:      httpapi.NewRouter(cfg, deps, logger),
		ReadTimeout:  cfg.ReadTimeout,
		WriteTimeout: cfg.WriteTimeout,
		IdleTimeout:  120 * time.Second,
	}

	errCh := make(chan error, 1)
	go func() {
		logger.Info("ops api server starting", "addr", cfg.Addr)
		errCh <- srv.ListenAndServe()
	}()

	sigCh := make(chan os.Signal, 1)
	signal.Notify(sigCh, syscall.SIGINT, syscall.SIGTERM)

	select {
	case sig := <-sigCh:
		logger.Info("shutdown signal received", "signal", sig.String())
	case err := <-errCh:
		if err != nil && !errors.Is(err, http.ErrServerClosed) {
			logger.Error("server failed", "error", err)
			os.Exit(1)
		}
		return
	}

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := srv.Shutdown(ctx); err != nil {
		logger.Error("graceful shutdown failed", "error", err)
		os.Exit(1)
	}

	logger.Info("server stopped")
}

func buildDependencies(cfg config.Config) (httpapi.Dependencies, *ingest.Service, *retention.Service, func(), error) {
	if cfg.DatabaseDSN == "" {
		store := memory.NewStore()
		ingestService := ingest.NewService(store, ingest.Config{
			RetryBaseDelay:   cfg.IngestRetryBaseDelay,
			RetryMaxDelay:    cfg.IngestRetryMaxDelay,
			RetryMaxAttempts: cfg.IngestRetryMaxAttempts,
		})
		securityService := security.NewService(store)
		retentionService := retention.NewService(store, store, retention.Config{
			DryRun:              cfg.Retention.DryRun,
			MaxRowsPerRun:       cfg.Retention.MaxRowsPerRun,
			RawMessageWindow:    cfg.Retention.RawMessageWindow,
			AuditEventWindow:    cfg.Retention.AuditEventWindow,
			InfraSnapshotWindow: cfg.Retention.InfraSnapshotWindow,
			IngestEventWindow:   cfg.Retention.IngestEventWindow,
		})
		return httpapi.Dependencies{
			Readiness:    store,
			Dashboard:    store,
			Conversation: store,
			Infra:        store,
			Security:     securityService,
			Ingest:       ingestService,
			Audit:        store,
		}, ingestService, retentionService, func() {}, nil
	}

	db, err := sql.Open(cfg.DatabaseDriver, cfg.DatabaseDSN)
	if err != nil {
		return httpapi.Dependencies{}, nil, nil, nil, err
	}

	db.SetMaxOpenConns(10)
	db.SetMaxIdleConns(5)
	db.SetConnMaxLifetime(5 * time.Minute)

	store := postgres.NewStore(db)
	ingestService := ingest.NewService(store, ingest.Config{
		RetryBaseDelay:   cfg.IngestRetryBaseDelay,
		RetryMaxDelay:    cfg.IngestRetryMaxDelay,
		RetryMaxAttempts: cfg.IngestRetryMaxAttempts,
	})
	securityService := security.NewService(store)
	retentionService := retention.NewService(store, store, retention.Config{
		DryRun:              cfg.Retention.DryRun,
		MaxRowsPerRun:       cfg.Retention.MaxRowsPerRun,
		RawMessageWindow:    cfg.Retention.RawMessageWindow,
		AuditEventWindow:    cfg.Retention.AuditEventWindow,
		InfraSnapshotWindow: cfg.Retention.InfraSnapshotWindow,
		IngestEventWindow:   cfg.Retention.IngestEventWindow,
	})
	cleanup := func() {
		_ = db.Close()
	}

	return httpapi.Dependencies{
		Readiness:    store,
		Dashboard:    store,
		Conversation: store,
		Infra:        store,
		Security:     securityService,
		Ingest:       ingestService,
		Audit:        store,
	}, ingestService, retentionService, cleanup, nil
}
