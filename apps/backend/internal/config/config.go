package config

import (
	"fmt"
	"os"
	"strconv"
	"strings"
	"time"
)

// Config holds API runtime configuration loaded from environment variables.
type Config struct {
	Addr                      string
	AdminToken                string
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

	cfg := Config{
		Addr:                      getEnv("OPS_API_ADDR", ":8080"),
		AdminToken:                strings.TrimSpace(os.Getenv("OPS_API_ADMIN_TOKEN")),
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
	}

	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("OPS_API_ADMIN_TOKEN is required")
	}
	if cfg.IngestToken == "" {
		return Config{}, fmt.Errorf("OPS_API_INGEST_TOKEN is required")
	}
	if cfg.IngestToken == cfg.AdminToken {
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
