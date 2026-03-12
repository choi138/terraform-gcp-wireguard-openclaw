package memory

import (
	"context"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

func TestCompactRawMessagePayloadsClearsExpiredPayloads(t *testing.T) {
	store := NewStore()
	store.messages[0].CreatedAt = time.Now().UTC().Add(-72 * time.Hour)
	store.messageRawPayload[store.messages[0].ID] = []byte("secret")

	affected, err := store.CompactRawMessagePayloads(context.Background(), time.Now().UTC().Add(-48*time.Hour), 10, false)
	if err != nil {
		t.Fatalf("expected raw payload compaction to succeed, got %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected one compacted payload, got %d", affected)
	}
	if len(store.messageRawPayload[store.messages[0].ID]) != 0 {
		t.Fatal("expected raw payload bytes to be removed")
	}
}

func TestDeleteExpiredInfraSnapshotsPreservesLatestSnapshot(t *testing.T) {
	store := NewStore()
	store.snapshots = append(store.snapshots, domain.InfraSnapshot{
		ID:           2,
		VPNPeerCount: 1,
		OpenClawUp:   true,
		CPUPct:       10,
		MemPct:       20,
		CapturedAt:   time.Now().UTC().Add(-72 * time.Hour),
	})
	store.snapshots = append(store.snapshots, domain.InfraSnapshot{
		ID:           3,
		VPNPeerCount: 2,
		OpenClawUp:   true,
		CPUPct:       15,
		MemPct:       25,
		CapturedAt:   time.Now().UTC().Add(-time.Hour),
	})

	affected, err := store.DeleteExpiredInfraSnapshots(context.Background(), time.Now().UTC().Add(-24*time.Hour), 10, false)
	if err != nil {
		t.Fatalf("expected infra snapshot retention to succeed, got %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected one old snapshot to be removed, got %d", affected)
	}
	latest, err := store.GetLatestStatus(context.Background())
	if err != nil {
		t.Fatalf("expected latest status to remain, got %v", err)
	}
	if latest.ID != 1 {
		t.Fatalf("expected seeded latest snapshot to remain, got id=%d", latest.ID)
	}
	if len(store.snapshots) != 2 {
		t.Fatalf("expected two snapshots to remain, got %d", len(store.snapshots))
	}
}

func TestDeleteExpiredIngestEventsKeepsQueuedItems(t *testing.T) {
	store := NewStore()
	old := time.Now().UTC().Add(-72 * time.Hour)
	store.ingestEvents["done"] = domain.IngestEventRecord{
		EventKey:    domain.EventKey{EventType: "conversation_event", Source: "openclaw", EventID: "done"},
		Status:      domain.IngestEventStatusCompleted,
		FirstSeenAt: old,
	}
	store.ingestEvents["queued"] = domain.IngestEventRecord{
		EventKey:    domain.EventKey{EventType: "conversation_event", Source: "openclaw", EventID: "queued"},
		Status:      domain.IngestEventStatusRetryScheduled,
		FirstSeenAt: old,
	}

	affected, err := store.DeleteExpiredIngestEvents(context.Background(), time.Now().UTC().Add(-24*time.Hour), 10, false)
	if err != nil {
		t.Fatalf("expected ingest retention to succeed, got %v", err)
	}
	if affected != 1 {
		t.Fatalf("expected one completed event to be removed, got %d", affected)
	}
	if _, ok := store.ingestEvents["queued"]; !ok {
		t.Fatal("expected queued ingest event to remain")
	}
}
