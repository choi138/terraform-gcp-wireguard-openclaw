package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

// Config holds API runtime configuration loaded from environment variables.
type Config struct {
	Addr                string
	AdminToken          string
	AllowMemoryFallback bool
	DatabaseDSN         string
	DatabaseDriver      string
	ReadTimeout         time.Duration
	WriteTimeout        time.Duration
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

	cfg := Config{
		Addr:                getEnv("OPS_API_ADDR", ":8080"),
		AdminToken:          os.Getenv("OPS_API_ADMIN_TOKEN"),
		AllowMemoryFallback: allowMemoryFallback,
		DatabaseDSN:         os.Getenv("OPS_API_DB_DSN"),
		DatabaseDriver:      getEnv("OPS_API_DB_DRIVER", "postgres"),
		ReadTimeout:         time.Duration(readTimeoutSec) * time.Second,
		WriteTimeout:        time.Duration(writeTimeoutSec) * time.Second,
	}

	if cfg.AdminToken == "" {
		return Config{}, fmt.Errorf("OPS_API_ADMIN_TOKEN is required")
	}
	if cfg.DatabaseDSN == "" && !cfg.AllowMemoryFallback {
		return Config{}, fmt.Errorf("OPS_API_DB_DSN is required unless OPS_API_ALLOW_MEMORY_FALLBACK=true")
	}
	if cfg.ReadTimeout <= 0 || cfg.WriteTimeout <= 0 {
		return Config{}, fmt.Errorf("timeouts must be greater than zero")
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
