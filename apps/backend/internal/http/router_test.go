package httpapi_test

import (
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	httpapi "github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
)

func TestHealthzDoesNotRequireAuth(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/v1/healthz", nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestProtectedRouteRequiresBearerToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	from := time.Now().UTC().Add(-1 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestProtectedRouteWritesAuditLog(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	from := time.Now().UTC().Add(-24 * time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	events := store.AuditEvents()
	if len(events) == 0 {
		t.Fatalf("expected at least one audit event")
	}
	last := events[len(events)-1]
	if last.Actor != "admin" {
		t.Fatalf("expected actor admin, got %q", last.Actor)
	}
	if last.ResourceID != "" {
		t.Fatalf("expected empty resource id for static route, got %q", last.ResourceID)
	}
}

func TestProtectedConversationRouteWritesPathIdentifierToAuditLog(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/1", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	events := store.AuditEvents()
	if len(events) == 0 {
		t.Fatalf("expected at least one audit event")
	}
	last := events[len(events)-1]
	if last.ResourceType != "conversations" {
		t.Fatalf("expected resource type conversations, got %q", last.ResourceType)
	}
	if last.ResourceID != "1" {
		t.Fatalf("expected resource id 1, got %q", last.ResourceID)
	}
}

func TestProtectedConversationNotFoundWritesAuditLog(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/999", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Fatalf("expected status %d, got %d", http.StatusNotFound, rr.Code)
	}

	events := store.AuditEvents()
	if len(events) == 0 {
		t.Fatalf("expected at least one audit event")
	}
	last := events[len(events)-1]
	if last.ResourceType != "conversations" {
		t.Fatalf("expected resource type conversations, got %q", last.ResourceType)
	}
	if last.ResourceID != "999" {
		t.Fatalf("expected resource id 999, got %q", last.ResourceID)
	}
	if got := last.Metadata["status"]; got != http.StatusNotFound {
		t.Fatalf("expected audit status %d, got %v", http.StatusNotFound, got)
	}
}

func TestProtectedConversationBadRequestWritesAuditLog(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/0", nil)
	req.Header.Set("Authorization", "Bearer test-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	events := store.AuditEvents()
	if len(events) == 0 {
		t.Fatalf("expected at least one audit event")
	}
	last := events[len(events)-1]
	if last.ResourceType != "conversations" {
		t.Fatalf("expected resource type conversations, got %q", last.ResourceType)
	}
	if last.ResourceID != "0" {
		t.Fatalf("expected resource id 0, got %q", last.ResourceID)
	}
	if got := last.Metadata["status"]; got != http.StatusBadRequest {
		t.Fatalf("expected audit status %d, got %v", http.StatusBadRequest, got)
	}
}

func testDependencies(store *memory.Store) httpapi.Dependencies {
	return httpapi.Dependencies{
		Readiness:    store,
		Dashboard:    store,
		Conversation: store,
		Infra:        store,
		Audit:        store,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}
