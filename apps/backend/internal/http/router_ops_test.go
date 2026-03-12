package httpapi_test

import (
	"bytes"
	"crypto"
	"crypto/rand"
	"crypto/rsa"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"log/slog"
	"math/big"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/config"
	httpapi "github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/http"
	"github.com/choegeun-won/terraform-gcp-wireguard-openclaw/apps/backend/internal/repository/memory"
)

func TestOIDCViewerCanAccessReadOnlyRoute(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	h := httpapi.NewRouter(provider.Config(config.Config{
		IngestToken: "ingest-token",
	}), testDependencies(store), testLogger())

	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	req.Header.Set("Authorization", "Bearer "+provider.Token(t, "viewer-1", time.Now().UTC().Add(time.Hour), []string{"viewer"}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
}

func TestOIDCViewerCannotAccessAdminOnlyRoute(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	h := httpapi.NewRouter(provider.Config(config.Config{
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
	}), testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer "+provider.Token(t, "viewer-1", time.Now().UTC().Add(time.Hour), []string{"viewer"}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusForbidden, rr.Code, rr.Body.String())
	}
}

func TestOIDCRejectsExpiredToken(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	h := httpapi.NewRouter(provider.Config(config.Config{
		IngestToken: "ingest-token",
	}), testDependencies(store), testLogger())

	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	req.Header.Set("Authorization", "Bearer "+provider.Token(t, "viewer-1", time.Now().UTC().Add(-time.Minute), []string{"viewer"}))
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestBreakGlassDisabledRejectsLegacyAdminTokenWhenOIDCEnabled(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	cfg := provider.Config(config.Config{
		AdminToken:  "admin-token",
		IngestToken: "ingest-token",
	})
	h := httpapi.NewRouter(cfg, testDependencies(store), testLogger())

	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)
	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusUnauthorized, rr.Code, rr.Body.String())
	}
}

func TestBreakGlassEnabledAllowsConfiguredPathAndWritesAudit(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	cfg := provider.Config(config.Config{
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
		BreakGlass: config.BreakGlassConfig{
			Enabled:      true,
			Token:        "break-glass",
			Role:         "admin",
			ExpiresAt:    time.Now().UTC().Add(time.Hour),
			AllowedPaths: []string{"/v1/security"},
			Reason:       "idp outage",
			Approver:     "oncall",
		},
	})
	h := httpapi.NewRouter(cfg, testDependencies(store), testLogger())

	req := httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer break-glass")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}

	events := waitForAuditEvents(t, store, 2)
	found := false
	for _, event := range events {
		if event.Action == "auth.break_glass" {
			found = true
			break
		}
	}
	if !found {
		t.Fatal("expected break-glass audit event to be recorded")
	}
}

func TestMetricsRouteExposesAlertSignals(t *testing.T) {
	provider := newOIDCTestProvider(t)
	defer provider.Close()

	store := memory.NewStore()
	h := httpapi.NewRouter(provider.Config(config.Config{
		IngestToken:          "ingest-token",
		SecurityMaxBodyBytes: 1 << 20,
		Trace: config.TraceConfig{
			Exporter:    "none",
			SampleRate:  0,
			ServiceName: "ops-api",
		},
	}), testDependencies(store), testLogger())

	viewerToken := provider.Token(t, "viewer-1", time.Now().UTC().Add(time.Hour), []string{"viewer"})
	from := time.Now().UTC().Add(-time.Hour).Format(time.RFC3339)
	to := time.Now().UTC().Format(time.RFC3339)

	req := httptest.NewRequest(http.MethodGet, "/v1/dashboard/summary?from="+from+"&to="+to, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusOK {
		t.Fatalf("expected summary status %d, got %d", http.StatusOK, rr.Code)
	}

	req = httptest.NewRequest(http.MethodPost, "/v1/security/analyze-tfvars", strings.NewReader(validSecurityAnalysisJSON()))
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)
	if rr.Code != http.StatusForbidden {
		t.Fatalf("expected analyze status %d, got %d", http.StatusForbidden, rr.Code)
	}

	req = httptest.NewRequest(http.MethodGet, "/metrics", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	rr = httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusOK {
		t.Fatalf("expected metrics status %d, got %d body=%s", http.StatusOK, rr.Code, rr.Body.String())
	}
	body := rr.Body.String()
	for _, want := range []string{
		"ops_api_service_up 1",
		"ops_api_http_requests_total",
		"ops_api_http_request_errors_total",
		"ops_api_ingest_queue_depth",
		"ops_api_ingest_oldest_retry_age_seconds",
	} {
		if !strings.Contains(body, want) {
			t.Fatalf("expected metrics output to contain %q, got %s", want, body)
		}
	}
}

func TestObservabilityLogsStructuredFieldsAndTraceSpan(t *testing.T) {
	store := memory.NewStore()
	var logBuffer bytes.Buffer
	logger := slog.New(slog.NewJSONHandler(&logBuffer, nil))
	h := httpapi.NewRouter(config.Config{
		AdminToken:  "admin-token",
		IngestToken: "ingest-token",
		Trace: config.TraceConfig{
			Exporter:    "stdout",
			SampleRate:  1,
			ServiceName: "ops-api",
		},
	}, testDependencies(store), logger)

	req := httptest.NewRequest(http.MethodGet, "/v1/conversations/0", nil)
	req.Header.Set("Authorization", "Bearer admin-token")
	rr := httptest.NewRecorder()
	h.ServeHTTP(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Fatalf("expected status %d, got %d", http.StatusBadRequest, rr.Code)
	}

	output := logBuffer.String()
	for _, want := range []string{
		`"msg":"trace span"`,
		`"msg":"http request completed"`,
		`"request_id":`,
		`"principal_id":"admin"`,
		`"path":"/v1/conversations/0"`,
		`"status":400`,
		`"error_code":"bad_request"`,
	} {
		if !strings.Contains(output, want) {
			t.Fatalf("expected structured log output to contain %q, got %s", want, output)
		}
	}
}

type oidcTestProvider struct {
	server     *httptest.Server
	privateKey *rsa.PrivateKey
	kid        string
	issuer     string
	audience   string
}

func newOIDCTestProvider(t *testing.T) *oidcTestProvider {
	t.Helper()

	privateKey, err := rsa.GenerateKey(rand.Reader, 2048)
	if err != nil {
		t.Fatalf("failed to generate test key: %v", err)
	}

	provider := &oidcTestProvider{
		privateKey: privateKey,
		kid:        "test-key",
		audience:   "ops-api",
	}
	provider.server = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/jwks.json" {
			http.NotFound(w, r)
			return
		}
		n := base64.RawURLEncoding.EncodeToString(privateKey.PublicKey.N.Bytes())
		e := base64.RawURLEncoding.EncodeToString(big.NewInt(int64(privateKey.PublicKey.E)).Bytes())
		_ = json.NewEncoder(w).Encode(map[string]any{
			"keys": []map[string]any{{
				"kty": "RSA",
				"kid": provider.kid,
				"alg": "RS256",
				"use": "sig",
				"n":   n,
				"e":   e,
			}},
		})
	}))
	provider.issuer = provider.server.URL
	return provider
}

func (p *oidcTestProvider) Close() {
	if p.server != nil {
		p.server.Close()
	}
}

func (p *oidcTestProvider) Config(cfg config.Config) config.Config {
	cfg.OIDC = config.OIDCConfig{
		Issuer:       p.issuer,
		Audience:     p.audience,
		JWKSURL:      p.server.URL + "/jwks.json",
		RolesClaim:   "roles",
		SubjectClaim: "sub",
		ClockSkew:    time.Minute,
	}
	if cfg.Trace.Exporter == "" {
		cfg.Trace = config.TraceConfig{
			Exporter:    "none",
			SampleRate:  0,
			ServiceName: "ops-api",
		}
	}
	return cfg
}

func (p *oidcTestProvider) Token(t *testing.T, subject string, expiresAt time.Time, roles []string) string {
	t.Helper()

	header := map[string]any{
		"alg": "RS256",
		"kid": p.kid,
		"typ": "JWT",
	}
	claims := map[string]any{
		"iss":   p.issuer,
		"aud":   p.audience,
		"sub":   subject,
		"email": subject + "@example.com",
		"roles": roles,
		"iat":   time.Now().UTC().Add(-time.Minute).Unix(),
		"nbf":   time.Now().UTC().Add(-time.Minute).Unix(),
		"exp":   expiresAt.Unix(),
	}
	unsigned := encodeJWTPart(t, header) + "." + encodeJWTPart(t, claims)
	sum := sha256.Sum256([]byte(unsigned))
	signature, err := rsa.SignPKCS1v15(rand.Reader, p.privateKey, crypto.SHA256, sum[:])
	if err != nil {
		t.Fatalf("failed to sign token: %v", err)
	}
	return unsigned + "." + base64.RawURLEncoding.EncodeToString(signature)
}

func encodeJWTPart(t *testing.T, payload any) string {
	t.Helper()
	raw, err := json.Marshal(payload)
	if err != nil {
		t.Fatalf("failed to marshal JWT payload: %v", err)
	}
	return base64.RawURLEncoding.EncodeToString(raw)
}
