package config

import (
	"fmt"
	"math"
	"os"
	"strconv"
	"strings"
	"time"
)

type OIDCConfig struct {
	Issuer       string
	Audience     string
	JWKSURL      string
	RolesClaim   string
	SubjectClaim string
	ClockSkew    time.Duration
}

func (c OIDCConfig) Enabled() bool {
	return c.Issuer != "" || c.Audience != "" || c.JWKSURL != ""
}

type BreakGlassConfig struct {
	Enabled      bool
	Token        string
	Role         string
	ExpiresAt    time.Time
	AllowedPaths []string
	Reason       string
	Approver     string
}

type TraceConfig struct {
	Exporter    string
	SampleRate  float64
	ServiceName string
}

type RetentionConfig struct {
	Enabled             bool
	DryRun              bool
	Interval            time.Duration
	MaxRowsPerRun       int
	MinWindow           time.Duration
	RawMessageWindow    time.Duration
	AuditEventWindow    time.Duration
	InfraSnapshotWindow time.Duration
	IngestEventWindow   time.Duration
}

// Config holds API runtime configuration loaded from environment variables.
type Config struct {
	Addr                      string
	AdminToken                string
	AdminTokenCompatibility   bool
	IngestToken               string
	AllowMemoryFallback       bool
	DatabaseDSN               string
	DatabaseDriver            string
	ReadTimeout               time.Duration
	WriteTimeout              time.Duration
	IngestMaxBodyBytes        int64
	SecurityMaxBodyBytes      int64
	IngestRetryBaseDelay      time.Duration
	IngestRetryMaxDelay       time.Duration
	IngestRetryMaxAttempts    int
	IngestRetryWorkerInterval time.Duration
	IngestRetryBatchSize      int
	OIDC                      OIDCConfig
	BreakGlass                BreakGlassConfig
	Trace                     TraceConfig
	Retention                 RetentionConfig
}

// LoadFromEnv builds Config from environment variables.
func LoadFromEnv() (Config, error) {
	allowMemoryFallback, err := getEnvBool("OPS_API_ALLOW_MEMORY_FALLBACK", false)
	if err != nil {
		return Config{}, err
	}
	readTimeoutSec, err := getEnvInt("OPS_API_READ_TIMEOUT_SEC", 10)
	if err != nil {
		return Config{}, err
	}
	writeTimeoutSec, err := getEnvInt("OPS_API_WRITE_TIMEOUT_SEC", 10)
	if err != nil {
		return Config{}, err
	}
	ingestMaxBodyBytes, err := getEnvInt64("OPS_API_INGEST_MAX_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	securityMaxBodyBytes, err := getEnvInt64("OPS_API_SECURITY_MAX_BODY_BYTES", 1<<20)
	if err != nil {
		return Config{}, err
	}
	ingestRetryBaseDelayMS, err := getEnvInt("OPS_API_INGEST_RETRY_BASE_DELAY_MS", 1000)
	if err != nil {
		return Config{}, err
	}
	ingestRetryMaxDelayMS, err := getEnvInt("OPS_API_INGEST_RETRY_MAX_DELAY_MS", 30000)
	if err != nil {
		return Config{}, err
	}
	ingestRetryMaxAttempts, err := getEnvInt("OPS_API_INGEST_RETRY_MAX_ATTEMPTS", 5)
	if err != nil {
		return Config{}, err
	}
	ingestRetryWorkerIntervalMS, err := getEnvInt("OPS_API_INGEST_RETRY_WORKER_INTERVAL_MS", 1000)
	if err != nil {
		return Config{}, err
	}
	ingestRetryBatchSize, err := getEnvInt("OPS_API_INGEST_RETRY_BATCH_SIZE", 20)
	if err != nil {
		return Config{}, err
	}
	adminTokenCompatibility, err := getEnvBool("OPS_API_ADMIN_TOKEN_COMPATIBILITY", false)
	if err != nil {
		return Config{}, err
	}
	oidcClockSkewSec, err := getEnvInt("OPS_API_OIDC_CLOCK_SKEW_SEC", 60)
	if err != nil {
		return Config{}, err
	}
	breakGlassEnabled, err := getEnvBool("OPS_API_BREAK_GLASS_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	traceSampleRate, err := getEnvFloat("OPS_API_TRACE_SAMPLE_RATE", 0.1)
	if err != nil {
		return Config{}, err
	}
	retentionEnabled, err := getEnvBool("OPS_API_RETENTION_ENABLED", false)
	if err != nil {
		return Config{}, err
	}
	retentionDryRun, err := getEnvBool("OPS_API_RETENTION_DRY_RUN", true)
	if err != nil {
		return Config{}, err
	}
	retentionIntervalSec, err := getEnvInt("OPS_API_RETENTION_INTERVAL_SEC", 3600)
	if err != nil {
		return Config{}, err
	}
	retentionMaxRows, err := getEnvInt("OPS_API_RETENTION_MAX_ROWS_PER_RUN", 500)
	if err != nil {
		return Config{}, err
	}
	retentionMinWindowHours, err := getEnvInt("OPS_API_RETENTION_MIN_WINDOW_HOURS", 24)
	if err != nil {
		return Config{}, err
	}
	retentionRawMessageHours, err := getEnvInt("OPS_API_RETENTION_RAW_MESSAGE_HOURS", 168)
	if err != nil {
		return Config{}, err
	}
	retentionAuditHours, err := getEnvInt("OPS_API_RETENTION_AUDIT_EVENT_HOURS", 720)
	if err != nil {
		return Config{}, err
	}
	retentionInfraHours, err := getEnvInt("OPS_API_RETENTION_INFRA_SNAPSHOT_HOURS", 720)
	if err != nil {
		return Config{}, err
	}
	retentionIngestHours, err := getEnvInt("OPS_API_RETENTION_INGEST_EVENT_HOURS", 336)
	if err != nil {
		return Config{}, err
	}

	oidc := OIDCConfig{
		Issuer:       strings.TrimSpace(os.Getenv("OPS_API_OIDC_ISSUER")),
		Audience:     strings.TrimSpace(os.Getenv("OPS_API_OIDC_AUDIENCE")),
		JWKSURL:      strings.TrimSpace(os.Getenv("OPS_API_OIDC_JWKS_URL")),
		RolesClaim:   getEnv("OPS_API_OIDC_ROLES_CLAIM", "roles"),
		SubjectClaim: getEnv("OPS_API_OIDC_SUBJECT_CLAIM", "sub"),
		ClockSkew:    time.Duration(oidcClockSkewSec) * time.Second,
	}

	breakGlass := BreakGlassConfig{
		Enabled:      breakGlassEnabled,
		Token:        strings.TrimSpace(os.Getenv("OPS_API_BREAK_GLASS_TOKEN")),
		Role:         strings.TrimSpace(getEnv("OPS_API_BREAK_GLASS_ROLE", "admin")),
		Reason:       strings.TrimSpace(os.Getenv("OPS_API_BREAK_GLASS_REASON")),
		Approver:     strings.TrimSpace(os.Getenv("OPS_API_BREAK_GLASS_APPROVER")),
		AllowedPaths: splitCSV(os.Getenv("OPS_API_BREAK_GLASS_ALLOWED_PATHS")),
	}
	if raw := strings.TrimSpace(os.Getenv("OPS_API_BREAK_GLASS_EXPIRES_AT")); raw != "" {
		parsed, parseErr := time.Parse(time.RFC3339, raw)
		if parseErr != nil {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_EXPIRES_AT must be RFC3339")
		}
		breakGlass.ExpiresAt = parsed.UTC()
	}

	cfg := Config{
		Addr:                      getEnv("OPS_API_ADDR", ":8080"),
		AdminToken:                strings.TrimSpace(os.Getenv("OPS_API_ADMIN_TOKEN")),
		AdminTokenCompatibility:   adminTokenCompatibility,
		IngestToken:               strings.TrimSpace(os.Getenv("OPS_API_INGEST_TOKEN")),
		AllowMemoryFallback:       allowMemoryFallback,
		DatabaseDSN:               strings.TrimSpace(os.Getenv("OPS_API_DB_DSN")),
		DatabaseDriver:            getEnv("OPS_API_DB_DRIVER", "postgres"),
		ReadTimeout:               time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout:              time.Duration(writeTimeoutSec) * time.Second,
		IngestMaxBodyBytes:        ingestMaxBodyBytes,
		SecurityMaxBodyBytes:      securityMaxBodyBytes,
		IngestRetryBaseDelay:      time.Duration(ingestRetryBaseDelayMS) * time.Millisecond,
		IngestRetryMaxDelay:       time.Duration(ingestRetryMaxDelayMS) * time.Millisecond,
		IngestRetryMaxAttempts:    ingestRetryMaxAttempts,
		IngestRetryWorkerInterval: time.Duration(ingestRetryWorkerIntervalMS) * time.Millisecond,
		IngestRetryBatchSize:      ingestRetryBatchSize,
		OIDC:                      oidc,
		BreakGlass:                breakGlass,
		Trace: TraceConfig{
			Exporter:    getEnv("OPS_API_TRACE_EXPORTER", "stdout"),
			SampleRate:  traceSampleRate,
			ServiceName: getEnv("OPS_API_TRACE_SERVICE_NAME", "ops-api"),
		},
		Retention: RetentionConfig{
			Enabled:             retentionEnabled,
			DryRun:              retentionDryRun,
			Interval:            time.Duration(retentionIntervalSec) * time.Second,
			MaxRowsPerRun:       retentionMaxRows,
			MinWindow:           time.Duration(retentionMinWindowHours) * time.Hour,
			RawMessageWindow:    time.Duration(retentionRawMessageHours) * time.Hour,
			AuditEventWindow:    time.Duration(retentionAuditHours) * time.Hour,
			InfraSnapshotWindow: time.Duration(retentionInfraHours) * time.Hour,
			IngestEventWindow:   time.Duration(retentionIngestHours) * time.Hour,
		},
	}

	if cfg.IngestToken == "" {
		return Config{}, fmt.Errorf("OPS_API_INGEST_TOKEN is required")
	}
	if cfg.AdminToken == "" && !cfg.OIDC.Enabled() {
		return Config{}, fmt.Errorf("OPS_API_ADMIN_TOKEN is required unless OIDC is configured")
	}
	if cfg.IngestToken == cfg.AdminToken && cfg.AdminToken != "" {
		return Config{}, fmt.Errorf("OPS_API_INGEST_TOKEN must differ from OPS_API_ADMIN_TOKEN")
	}
	if cfg.DatabaseDSN == "" && !cfg.AllowMemoryFallback {
		return Config{}, fmt.Errorf("OPS_API_DB_DSN is required unless OPS_API_ALLOW_MEMORY_FALLBACK=true")
	}
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 {
		return Config{}, fmt.Errorf("timeouts must be greater than zero")
	}
	if cfg.IngestMaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("OPS_API_INGEST_MAX_BODY_BYTES must be greater than zero")
	}
	if cfg.SecurityMaxBodyBytes <= 0 {
		return Config{}, fmt.Errorf("OPS_API_SECURITY_MAX_BODY_BYTES must be greater than zero")
	}
	if cfg.IngestRetryBaseDelay <= 0 || cfg.IngestRetryMaxDelay <= 0 {
		return Config{}, fmt.Errorf("ingest retry delays must be greater than zero")
	}
	if cfg.IngestRetryBaseDelay > cfg.IngestRetryMaxDelay {
		return Config{}, fmt.Errorf("OPS_API_INGEST_RETRY_BASE_DELAY_MS must be less than or equal to OPS_API_INGEST_RETRY_MAX_DELAY_MS")
	}
	if cfg.IngestRetryMaxAttempts < 1 {
		return Config{}, fmt.Errorf("OPS_API_INGEST_RETRY_MAX_ATTEMPTS must be greater than zero")
	}
	if cfg.IngestRetryWorkerInterval <= 0 {
		return Config{}, fmt.Errorf("OPS_API_INGEST_RETRY_WORKER_INTERVAL_MS must be greater than zero")
	}
	if cfg.IngestRetryBatchSize < 1 {
		return Config{}, fmt.Errorf("OPS_API_INGEST_RETRY_BATCH_SIZE must be greater than zero")
	}
	if cfg.OIDC.Enabled() {
		switch {
		case cfg.OIDC.Issuer == "":
			return Config{}, fmt.Errorf("OPS_API_OIDC_ISSUER is required when OIDC is configured")
		case cfg.OIDC.Audience == "":
			return Config{}, fmt.Errorf("OPS_API_OIDC_AUDIENCE is required when OIDC is configured")
		case cfg.OIDC.JWKSURL == "":
			return Config{}, fmt.Errorf("OPS_API_OIDC_JWKS_URL is required when OIDC is configured")
		}
		if cfg.OIDC.ClockSkew < 0 {
			return Config{}, fmt.Errorf("OPS_API_OIDC_CLOCK_SKEW_SEC must be greater than or equal to zero")
		}
		if strings.TrimSpace(cfg.OIDC.RolesClaim) == "" || strings.TrimSpace(cfg.OIDC.SubjectClaim) == "" {
			return Config{}, fmt.Errorf("OIDC claim names must not be empty")
		}
	}
	if cfg.AdminTokenCompatibility && cfg.OIDC.Enabled() && cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("OPS_API_ADMIN_TOKEN is required when OPS_API_ADMIN_TOKEN_COMPATIBILITY=true")
	}
	if cfg.BreakGlass.Enabled {
		if !cfg.OIDC.Enabled() {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_ENABLED requires OIDC configuration")
		}
		if cfg.BreakGlass.Token == "" {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_TOKEN is required when break-glass is enabled")
		}
		if cfg.BreakGlass.Token == cfg.IngestToken || (cfg.AdminToken != "" && cfg.BreakGlass.Token == cfg.AdminToken) {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_TOKEN must differ from other service tokens")
		}
		if cfg.BreakGlass.ExpiresAt.IsZero() {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_EXPIRES_AT is required when break-glass is enabled")
		}
		if len(cfg.BreakGlass.AllowedPaths) == 0 {
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_ALLOWED_PATHS is required when break-glass is enabled")
		}
		switch cfg.BreakGlass.Role {
		case "admin", "viewer", "auditor":
		default:
			return Config{}, fmt.Errorf("OPS_API_BREAK_GLASS_ROLE must be one of: admin,viewer,auditor")
		}
	}
	switch cfg.Trace.Exporter {
	case "none", "stdout":
	default:
		return Config{}, fmt.Errorf("OPS_API_TRACE_EXPORTER must be one of: none,stdout")
	}
	if math.IsNaN(cfg.Trace.SampleRate) || cfg.Trace.SampleRate < 0 || cfg.Trace.SampleRate > 1 {
		return Config{}, fmt.Errorf("OPS_API_TRACE_SAMPLE_RATE must be between 0 and 1")
	}
	if strings.TrimSpace(cfg.Trace.ServiceName) == "" {
		return Config{}, fmt.Errorf("OPS_API_TRACE_SERVICE_NAME must not be empty")
	}
	if cfg.Retention.Enabled {
		if cfg.Retention.Interval <= 0 {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_INTERVAL_SEC must be greater than zero")
		}
		if cfg.Retention.MaxRowsPerRun < 1 {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_MAX_ROWS_PER_RUN must be greater than zero")
		}
		if cfg.Retention.MinWindow <= 0 {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_MIN_WINDOW_HOURS must be greater than zero")
		}
		if cfg.Retention.RawMessageWindow < cfg.Retention.MinWindow {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_RAW_MESSAGE_HOURS must be greater than or equal to OPS_API_RETENTION_MIN_WINDOW_HOURS")
		}
		if cfg.Retention.AuditEventWindow < cfg.Retention.MinWindow {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_AUDIT_EVENT_HOURS must be greater than or equal to OPS_API_RETENTION_MIN_WINDOW_HOURS")
		}
		if cfg.Retention.InfraSnapshotWindow < cfg.Retention.MinWindow {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_INFRA_SNAPSHOT_HOURS must be greater than or equal to OPS_API_RETENTION_MIN_WINDOW_HOURS")
		}
		if cfg.Retention.IngestEventWindow < cfg.Retention.MinWindow {
			return Config{}, fmt.Errorf("OPS_API_RETENTION_INGEST_EVENT_HOURS must be greater than or equal to OPS_API_RETENTION_MIN_WINDOW_HOURS")
		}
	}

	return cfg, nil
}

func getEnv(key, fallback string) string {
	if v, ok := os.LookupEnv(key); ok && v != "" {
		return v
	}
	return fallback
}

func getEnvInt(key string, fallback int) (int, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	parsed, err := strconv.Atoi(v)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func getEnvInt64(key string, fallback int64) (int64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseInt(v, 10, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be an integer", key)
	}
	return parsed, nil
}

func getEnvBool(key string, fallback bool) (bool, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(v)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean", key)
	}
	return parsed, nil
}

func getEnvFloat(key string, fallback float64) (float64, error) {
	v, ok := os.LookupEnv(key)
	if !ok || v == "" {
		return fallback, nil
	}
	parsed, err := strconv.ParseFloat(v, 64)
	if err != nil {
		return 0, fmt.Errorf("%s must be a float", key)
	}
	return parsed, nil
}

func splitCSV(v string) []string {
	parts := strings.Split(v, ",")
	out := make([]string, 0, len(parts))
	for _, part := range parts {
		trimmed := strings.TrimSpace(part)
		if trimmed == "" {
			continue
		}
		out = append(out, trimmed)
	}
	return out
}
