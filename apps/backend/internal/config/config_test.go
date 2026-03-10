package config

import "testing"

func TestLoadFromEnvRequiresAdminToken(t *testing.T) {
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_ADMIN_TOKEN", "")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected missing admin token to fail")
	}
}

func TestLoadFromEnvRequiresDatabaseDSNWithoutFallback(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "test-token")
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "false")
	t.Setenv("OPS_API_DB_DSN", "")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected missing database dsn to fail")
	}
}

func TestLoadFromEnvAllowsMemoryFallback(t *testing.T) {
	t.Setenv("OPS_API_ADMIN_TOKEN", "test-token")
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
	t.Setenv("OPS_API_ALLOW_MEMORY_FALLBACK", "true")
	t.Setenv("OPS_API_DB_DSN", "")
	t.Setenv("OPS_API_READ_TIMEOUT_SEC", "abc")

	_, err := LoadFromEnv()
	if err == nil {
		t.Fatal("expected invalid read timeout to fail")
	}
}
