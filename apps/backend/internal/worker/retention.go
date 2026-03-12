package worker

import (
	"context"
	"log/slog"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/retention"
)

type RetentionWorker struct {
	service  *retention.Service
	logger   *slog.Logger
	interval time.Duration
}

func NewRetentionWorker(service *retention.Service, logger *slog.Logger, interval time.Duration) *RetentionWorker {
	if interval <= 0 {
		interval = time.Hour
	}
	return &RetentionWorker{
		service:  service,
		logger:   logger,
		interval: interval,
	}
}

func (w *RetentionWorker) Run(ctx context.Context) {
	if w == nil || w.service == nil {
		return
	}

	w.runOnce(ctx)

	ticker := time.NewTicker(w.interval)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			w.runOnce(ctx)
		}
	}
}

func (w *RetentionWorker) runOnce(ctx context.Context) {
	report, err := w.service.RunOnce(ctx, time.Now().UTC())
	if err != nil {
		if w.logger != nil {
			w.logger.Error("retention worker failed", "error", err)
		}
		return
	}
	if w.logger != nil {
		w.logger.Info("retention worker completed",
			"dry_run", report.DryRun,
			"targets", report.Targets,
		)
	}
}
