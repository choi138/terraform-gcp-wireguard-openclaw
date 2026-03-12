package retention

import (
	"context"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

type fakeStore struct {
	counts map[string]int
}

func (s fakeStore) CompactRawMessagePayloads(context.Context, time.Time, int, bool) (int, error) {
	return s.counts["raw_message_payloads"], nil
}

func (s fakeStore) DeleteExpiredAuditEvents(context.Context, time.Time, int, bool) (int, error) {
	return s.counts["audit_events"], nil
}

func (s fakeStore) DeleteExpiredInfraSnapshots(context.Context, time.Time, int, bool) (int, error) {
	return s.counts["infra_snapshots"], nil
}

func (s fakeStore) DeleteExpiredIngestEvents(context.Context, time.Time, int, bool) (int, error) {
	return s.counts["ingest_events"], nil
}

type fakeAuditWriter struct {
	events []domain.AuditEvent
}

func (w *fakeAuditWriter) InsertAuditEvent(_ context.Context, event domain.AuditEvent) error {
	w.events = append(w.events, event)
	return nil
}

func TestRunOnceReportsTargetsAndWritesAudit(t *testing.T) {
	audit := &fakeAuditWriter{}
	service := NewService(fakeStore{
		counts: map[string]int{
			"raw_message_payloads": 2,
			"audit_events":         4,
			"infra_snapshots":      1,
			"ingest_events":        3,
		},
	}, audit, Config{
		DryRun:              true,
		MaxRowsPerRun:       10,
		RawMessageWindow:    7 * 24 * time.Hour,
		AuditEventWindow:    30 * 24 * time.Hour,
		InfraSnapshotWindow: 30 * 24 * time.Hour,
		IngestEventWindow:   14 * 24 * time.Hour,
	})

	now := time.Date(2026, 3, 12, 12, 0, 0, 0, time.UTC)
	report, err := service.RunOnce(context.Background(), now)
	if err != nil {
		t.Fatalf("expected retention run to succeed, got %v", err)
	}
	if !report.DryRun {
		t.Fatal("expected retention report to preserve dry-run flag")
	}
	if len(report.Targets) != 4 {
		t.Fatalf("expected 4 targets, got %d", len(report.Targets))
	}
	if len(audit.events) != 1 {
		t.Fatalf("expected one audit event, got %d", len(audit.events))
	}
	if audit.events[0].Action != "retention.run" {
		t.Fatalf("expected retention audit action, got %q", audit.events[0].Action)
	}
}
