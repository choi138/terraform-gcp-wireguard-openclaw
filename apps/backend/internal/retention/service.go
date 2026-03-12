package retention

import (
	"context"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type Store interface {
	CompactRawMessagePayloads(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error)
	DeleteExpiredAuditEvents(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error)
	DeleteExpiredInfraSnapshots(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error)
	DeleteExpiredIngestEvents(ctx context.Context, cutoff time.Time, limit int, dryRun bool) (int, error)
}

type AuditWriter interface {
	InsertAuditEvent(ctx context.Context, event domain.AuditEvent) error
}

type Config struct {
	DryRun              bool
	MaxRowsPerRun       int
	RawMessageWindow    time.Duration
	AuditEventWindow    time.Duration
	InfraSnapshotWindow time.Duration
	IngestEventWindow   time.Duration
}

type Service struct {
	store Store
	audit AuditWriter
	cfg   Config
}

func NewService(store Store, audit AuditWriter, cfg Config) *Service {
	return &Service{
		store: store,
		audit: audit,
		cfg:   cfg,
	}
}

func (s *Service) RunOnce(ctx context.Context, now time.Time) (domain.RetentionRunReport, error) {
	report := domain.RetentionRunReport{
		DryRun:    s.cfg.DryRun,
		StartedAt: now.UTC(),
		Targets:   make([]domain.RetentionTargetReport, 0, 4),
	}

	targets := []struct {
		name   string
		action string
		cutoff time.Time
		run    func(context.Context, time.Time, int, bool) (int, error)
	}{
		{
			name:   "raw_message_payloads",
			action: "compact",
			cutoff: now.Add(-s.cfg.RawMessageWindow),
			run:    s.store.CompactRawMessagePayloads,
		},
		{
			name:   "audit_events",
			action: "delete",
			cutoff: now.Add(-s.cfg.AuditEventWindow),
			run:    s.store.DeleteExpiredAuditEvents,
		},
		{
			name:   "infra_snapshots",
			action: "delete",
			cutoff: now.Add(-s.cfg.InfraSnapshotWindow),
			run:    s.store.DeleteExpiredInfraSnapshots,
		},
		{
			name:   "ingest_events",
			action: "delete",
			cutoff: now.Add(-s.cfg.IngestEventWindow),
			run:    s.store.DeleteExpiredIngestEvents,
		},
	}

	for _, target := range targets {
		affected, err := target.run(ctx, target.cutoff.UTC(), s.cfg.MaxRowsPerRun, s.cfg.DryRun)
		if err != nil {
			return report, err
		}
		report.Targets = append(report.Targets, domain.RetentionTargetReport{
			Target:         target.name,
			Action:         target.action,
			Cutoff:         target.cutoff.UTC(),
			CandidateCount: affected,
			AffectedCount:  affected,
		})
	}

	report.CompletedAt = time.Now().UTC()
	if s.audit != nil {
		if err := s.audit.InsertAuditEvent(ctx, domain.AuditEvent{
			Actor:        "retention-worker",
			Action:       "retention.run",
			ResourceType: "ops_api_data",
			Metadata: map[string]any{
				"dry_run": report.DryRun,
				"targets": report.Targets,
			},
			CreatedAt: report.CompletedAt,
		}); err != nil {
			return report, err
		}
	}

	return report, nil
}
