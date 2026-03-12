package config

import "testing"

func TestLoadFromEnvRequiresAdminToken(t *testing.T) {
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_ADMIN_TOKEN", "")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected missing admin token to fail")
	}
}

func TestLoadFromEnvRequiresDatabaseDSNWithoutFallback(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "test-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "false")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected missing database dsn to fail")
	}
}

func TestLoadFromEnvAllowsMemoryFallback(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "test-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("expected config to load, got error: %v", err)
	}
	if !cfg.AllowMemoryFallback {
		t.Fatal("expected AllowMemoryFallback to be true")
	}
}

func TestLoadFromEnvRejectsInvalidTimeoutEnv(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "test-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_READ_TIMEOUT_SEC", "abc")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid read timeout to fail")
	}
}

func TestLoadFromEnvRequiresIngestToken(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected missing ingest token to fail")
	}
}

func TestLoadFromEnvRejectsInvalidIngestRetryConfig(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_INGEST_RETRY_BASE_DELAY_MS", "5000")
	t.Setenv("OPS_API_INGEST_RETRY_MAX_DELAY_MS", "1000")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid ingest retry config to fail")
	}
}

func TestLoadFromEnvRejectsInvalidSecurityBodyLimit(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_SECURITY_MAX_BODY_BYTES", "0")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid security max body bytes to fail")
	}
}

func TestLoadFromEnvRejectsMatchingAdminAndIngestTokens(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "shared-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "shared-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected matching admin and ingest tokens to fail")
	}
}

func TestLoadFromEnvRejectsWhitespaceOnlyTokens(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "   ")
	t.Setenv("OPS_API_INGEST_TOKEN", "\t")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected whitespace-only tokens to fail")
	}
}

func TestLoadFromEnvAllowsOIDCWithoutAdminToken(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_OIDC_ISSUER", "https://issuer.example.com")
	t.Setenv("OPS_API_OIDC_AUDIENCE", "ops-api")
	t.Setenv("OPS_API_OIDC_JWKS_URL", "https://issuer.example.com/jwks")

	cfg, err := LoadFromEnv()
	if err != nil {
		t.Fatalf("expected OIDC-only config to load, got %v", err)
	}
	if !cfg.OIDC.Enabled() {
		t.Fatal("expected OIDC to be enabled")
	}
}

func TestLoadFromEnvRejectsBreakGlassWithoutOIDC(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_BREAK_GLASS_ENABLED", "true")
	t.Setenv("OPS_API_BREAK_GLASS_TOKEN", "break-glass")
	t.Setenv("OPS_API_BREAK_GLASS_ROLE", "admin")
	t.Setenv("OPS_API_BREAK_GLASS_ALLOWED_PATHS", "/v1/security")
	t.Setenv("OPS_API_BREAK_GLASS_EXPIRES_AT", "2026-03-12T12:00:00Z")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected break-glass without OIDC to fail")
	}
}

func TestLoadFromEnvRejectsTraceExporterOutsideAllowlist(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_TRACE_EXPORTER", "otlp")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected unsupported trace exporter to fail")
	}
}

func TestLoadFromEnvRejectsRetentionWindowBelowMinimum(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "admin-token")
	t.Setenv("OPS_API_INGEST_TOKEN", "ingest-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_RETENTION_ENABLED", "true")
	t.Setenv("OPS_API_RETENTION_MIN_WINDOW_HOURS", "48")
	t.Setenv("OPS_API_RETENTION_RAW_MESSAGE_HOURS", "24")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected too-small retention window to fail")
	}
}
