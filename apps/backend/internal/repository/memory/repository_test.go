package memory

import (
	"context"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
)

func TestInsertReadAuditCopiesMetadata(t *testing.T) {
	store := NewStore()
	event := domain.AuditEvent{
		Actor:        "admin",
		Action:       "read",
		ResourceType: "dashboard",
		Metadata: map[string]any{
			"path": "/v1/dashboard/summary",
			"nested": map[string]any{
				"status": 200,
			},
		},
		CreatedAt: time.Now().UTC(),
	}

	if err := store.InsertReadAudit(context.Background(), event); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	event.Metadata["path"] = "/mutated"
	event.Metadata["nested"].(map[string]any)["status"] = 500

	events := store.AuditEvents()
	if got := events[len(events)-1].Metadata["path"]; got != "/v1/dashboard/summary" {
		t.Fatalf("expected stored path to remain unchanged, got %v", got)
	}
	nested := events[len(events)-1].Metadata["nested"].(map[string]any)
	if got := nested["status"]; got != 200 {
		t.Fatalf("expected stored nested status to remain unchanged, got %v", got)
	}
}

func TestAuditEventsReturnsCopies(t *testing.T) {
	store := NewStore()
	if err := store.InsertReadAudit(context.Background(), domain.AuditEvent{
		Actor:        "admin",
		Action:       "read",
		ResourceType: "dashboard",
		Metadata: map[string]any{
			"path": "/v1/dashboard/summary",
		},
		CreatedAt: time.Now().UTC(),
	}); err != nil {
		t.Fatalf("expected no error, got %v", err)
	}

	events := store.AuditEvents()
	events[0].Metadata["path"] = "/mutated"

	eventsAgain := store.AuditEvents()
	if got := eventsAgain[0].Metadata["path"]; got != "/v1/dashboard/summary" {
		t.Fatalf("expected stored path to remain unchanged, got %v", got)
	}
}
