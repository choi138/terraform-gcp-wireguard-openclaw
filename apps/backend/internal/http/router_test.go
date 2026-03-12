package httpapi_test

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/domain"
	httpapi "github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/ingest"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/security"
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
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

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
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

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
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

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
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

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
	h := httpapi.NewRouter(config.Config{AdminToken: "test-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

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

func TestIngestRouteRequiresIngestToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "admin-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/conversation-events", strings.NewReader(validConversationEventJSON()))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestIngestRouteRejectsAdminToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{AdminToken: "admin-token", IngestToken: "ingest-token"}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/conversation-events", strings.NewReader(validConversationEventJSON()))
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestIngestConversationRouteHandlesDuplicates(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:         "admin-token",
		IngestToken:        "ingest-token",
		IngestMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/conversation-events", strings.NewReader(validConversationEventJSON()))
	req.Header.Set("Authorization", "Bearer ingest-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusCreated {
		t.Fatalf("expected status %d, got %d", http.StatusCreated, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/ingest/conversation-events", strings.NewReader(validConversationEventJSON()))
	req.Header.Set("Authorization", "Bearer ingest-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestIngestStatusRouteUsesAdminToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:         "admin-token",
		IngestToken:        "ingest-token",
		IngestMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodGet, "/v1/ingest/status", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}
}

func TestSecurityAnalyzeRouteRequiresAdminToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:           "admin-token",
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestSecurityAnalyzeRouteRejectsIngestToken(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:           "admin-token",
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer ingest-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d", http.StatusUnauthorized, rr.Code)
	}
}

func TestSecurityAnalyzeRoutePersistsFindingsAndAudit(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:           "admin-token",
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response domain.SecurityAnalysisResult
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("expected valid json response, got %v", err)
	}
	if len(response.Findings) < 6 {
		t.Fatalf("expected at least 6 findings, got %d", len(response.Findings))
	}

	events := waitForAuditEvents(t, store, 1)
	if len(events) == 0 {
		t.Fatal("expected audit event for security analysis")
	}
	last := events[len(events)-1]
	if last.Action != "security.analyze" {
		t.Fatalf("expected action security.analyze, got %q", last.Action)
	}
	if last.ResourceType != "security_findings" {
		t.Fatalf("expected resource type security_findings, got %q", last.ResourceType)
	}
}

func TestSecurityFindingsRouteFiltersBySeverityAndWritesReadAudit(t *testing.T) {
	store := memory.NewStore()
	h := httpapi.NewRouter(config.Config{
		AdminToken:           "admin-token",
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	waitForAuditEvents(t, store, 1)

	req = httptest.NewRequest(http.MethodGet, "/v1/security/findings?severity=critical&page=1&page_size=10&order=desc", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d", http.StatusOK, rr.Code)
	}

	var response struct {
		Page     int                      `json:"page"`
		PageSize int                      `json:"page_size"`
		Order    string                   `json:"order"`
		Items    []domain.SecurityFinding `json:"items"`
	}
	if err := json.NewDecoder(rr.Body).Decode(&response); err != nil {
		t.Fatalf("expected valid json response, got %v", err)
	}
	if response.Page != 1 || response.PageSize != 10 || response.Order != "desc" {
		t.Fatalf("unexpected pagination payload: %+v", response)
	}
	if len(response.Items) == 0 {
		t.Fatal("expected at least one critical finding")
	}
	for _, item := range response.Items {
		if item.Severity != domain.SecuritySeverityCritical {
			t.Fatalf("expected only critical findings, got %q", item.Severity)
		}
	}

	events := store.AuditEvents()
	if len(events) < 2 {
		t.Fatalf("expected at least two audit events, got %d", len(events))
	}
	last := events[len(events)-1]
	if last.Action != "read" {
		t.Fatalf("expected read audit action, got %q", last.Action)
	}
	if last.ResourceType != "security" {
		t.Fatalf("expected resource type security, got %q", last.ResourceType)
	}
}

func TestIngestRouteReturnsBadRequestForPermanentServiceError(t *testing.T) {
	store := memory.NewStore()
	deps := testDependencies(store)
	deps.Ingest = invalidInputIngestWriter{}

	h := httpapi.NewRouter(config.Config{
		AdminToken:         "admin-token",
		IngestToken:        "ingest-token",
		IngestMaxBodyBytes: 1 << 20,
	}, deps, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/ingest/conversation-events", strings.NewReader(validConversationEventJSON()))
	req.Header.Set("Authorization", "Bearer ingest-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusBadRequest, rr.Code, rr.Body.String())
	}
}

func TestSecurityAnalyzeRouteDoesNotBlockOnDetachedAuditWrite(t *testing.T) {
	store := memory.NewStore()
	audit := newBlockingAuditWriter(store)
	deps := testDependencies(store)
	deps.Audit = audit

	h := httpapi.NewRouter(config.Config{
		AdminToken:           "admin-token",
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}, deps, testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()

	done := make(chan struct{})
	go func() {
		h.ServeHTTP(rr, req)
		close(done)
	}()

	<-audit.started
	select {
	case <-done:
	case <-time.After(200 * time.Millisecond):
		t.Fatal("expected response to finish before detached audit write completed")
	}

	close(audit.release)
	events := waitForAuditEvents(t, store, 1)
	if len(events) != 1 {
		t.Fatalf("expected one detached audit event, got %d", len(events))
	}
}

func testDependencies(store *memory.Store) httpapi.Dependencies {
	ingestService := ingest.NewService(store, ingest.Config{})
	securityService := security.NewService(store)
	return httpapi.Dependencies{
		Readiness:    store,
		Dashboard:    store,
		Conversation: store,
		Infra:        store,
		Security:     securityService,
		Ingest:       ingestService,
		Audit:        store,
	}
}

func testLogger() *slog.Logger {
	return slog.New(slog.NewTextHandler(io.Discard, nil))
}

func validConversationEventJSON() string {
	return `{
		"schema_version": 1,
		"source": "openclaw",
		"event_id": "evt-router-1",
		"occurred_at": "2026-03-11T08:00:05Z",
		"account": {"external_id":"acct-router-1","email":"ops@example.com","status":"active"},
		"conversation": {"external_id":"conv-router-1","channel":"telegram","status":"completed","started_at":"2026-03-11T08:00:00Z"},
		"message": {"external_id":"msg-router-1","role":"user","content_masked":"hello","created_at":"2026-03-11T08:00:05Z"}
	}`
}

func validSecurityAnalysisJSON() string {
	return `{
		"schema_version": 1,
		"tfvars": {
			"openclaw_enable_public_ip": true,
			"ui_source_ranges": ["0.0.0.0/0"],
			"ssh_source_ranges": ["0.0.0.0/0"],
			"enable_project_oslogin": false,
			"wgeasy_password_secret": "projects/demo/secrets/plain-openclaw-password/versions/latest",
			"openclaw_openai_api_key_secret": "projects/demo/secrets/openai-api-token/versions/latest",
			"wg_port": 51820,
			"wgeasy_ui_port": 51821,
			"openclaw_gateway_port": 18789
		}
	}`
}

type invalidInputIngestWriter struct{}

func (invalidInputIngestWriter) IngestConversationEvent(context.Context, domain.ConversationEventInput) (domain.IngestResult, error) {
	return domain.IngestResult{}, fmt.Errorf("%w: invalid ingest payload", repository.ErrInvalidInput)
}

func (invalidInputIngestWriter) IngestInfraSnapshot(context.Context, domain.InfraSnapshotInput) (domain.IngestResult, error) {
	return domain.IngestResult{}, fmt.Errorf("%w: invalid ingest payload", repository.ErrInvalidInput)
}

func (invalidInputIngestWriter) IngestRequestAttempt(context.Context, domain.RequestAttemptEventInput) (domain.IngestResult, error) {
	return domain.IngestResult{}, fmt.Errorf("%w: invalid ingest payload", repository.ErrInvalidInput)
}

func (invalidInputIngestWriter) GetStatus(context.Context) (domain.IngestStatus, error) {
	return domain.IngestStatus{}, nil
}

type blockingAuditWriter struct {
	*memory.Store
	started chan struct{}
	release chan struct{}
}

func newBlockingAuditWriter(store *memory.Store) *blockingAuditWriter {
	return &blockingAuditWriter{
		Store:   store,
		started: make(chan struct{}),
		release: make(chan struct{}),
	}
}

func (w *blockingAuditWriter) InsertAuditEvent(ctx context.Context, event domain.AuditEvent) error {
	close(w.started)
	<-w.release
	return w.Store.InsertAuditEvent(ctx, event)
}

func waitForAuditEvents(t *testing.T, store *memory.Store, want int) []domain.AuditEvent {
	t.Helper()
	deadline := time.Now().Add(500 * time.Millisecond)
	for time.Now().Before(deadline) {
		events := store.AuditEvents()
		if len(events) >= want {
			return events
		}
		time.Sleep(10 * time.Millisecond)
	}
	events := store.AuditEvents()
	t.Fatalf("expected at least %d audit events, got %d", want, len(events))
	return nil
}
