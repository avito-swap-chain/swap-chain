// Package config loads and validates runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultDatabaseURL        = "postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable"
	defaultMigrationsURL      = "file://migrations"
	defaultHTTPAddress        = ":8080"
	defaultDBConnectTimeout   = 10 * time.Second
	defaultShutdownTimeout    = 10 * time.Second
	defaultSimilarItemsAmount = 20
	defaultChainLength        = 4
	defaultPenaltyFactor      = 0.25
	defaultChainThreshold     = 0.30
)

// Config contains runtime settings loaded from environment variables.
type Config struct {
	DatabaseURL        string
	MigrationsURL      string
	HTTPAddress        string
	DBConnectTimeout   time.Duration
	ShutdownTimeout    time.Duration
	SimilarItemsAmount int
	ChainLength        int
	PenaltyFactor      float64
	ChainThreshold     float64
}

// Migration contains the settings required by the migration command only.
type Migration struct {
	DatabaseURL   string
	MigrationsURL string
}

// LoadMigration reads migration settings without validating unrelated API options.
func LoadMigration() Migration {
	return Migration{
		DatabaseURL:   envOrDefault("DATABASE_URL", defaultDatabaseURL),
		MigrationsURL: envOrDefault("MIGRATIONS_URL", defaultMigrationsURL),
	}
}

// Load reads and validates application configuration from the environment.
func Load() (Config, error) {
	cfg := Config{
		DatabaseURL:   envOrDefault("DATABASE_URL", defaultDatabaseURL),
		MigrationsURL: envOrDefault("MIGRATIONS_URL", defaultMigrationsURL),
		HTTPAddress:   envOrDefault("HTTP_ADDR", defaultHTTPAddress),
	}

	var err error
	if cfg.DBConnectTimeout, err = durationFromEnv("DB_CONNECT_TIMEOUT", defaultDBConnectTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationFromEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SimilarItemsAmount, err = positiveIntFromEnv("MATCHING_SIMILAR_ITEMS", defaultSimilarItemsAmount); err != nil {
		return Config{}, err
	}
	if cfg.ChainLength, err = positiveIntFromEnv("MATCHING_CHAIN_LENGTH", defaultChainLength); err != nil {
		return Config{}, err
	}
	if cfg.ChainLength < 2 {
		return Config{}, fmt.Errorf("MATCHING_CHAIN_LENGTH must be at least 2")
	}
	if cfg.PenaltyFactor, err = nonNegativeFloatFromEnv("MATCHING_PENALTY_FACTOR", defaultPenaltyFactor); err != nil {
		return Config{}, err
	}
	if cfg.ChainThreshold, err = boundedFloatFromEnv("MATCHING_CHAIN_THRESHOLD", defaultChainThreshold, 0, 1); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func envOrDefault(name, fallback string) string {
	if value := os.Getenv(name); value != "" {
		return value
	}
	return fallback
}

func durationFromEnv(name string, fallback time.Duration) (time.Duration, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := time.ParseDuration(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive duration: %q", name, value)
	}
	return parsed, nil
}

func positiveIntFromEnv(name string, fallback int) (int, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.Atoi(value)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer: %q", name, value)
	}
	return parsed, nil
}

func nonNegativeFloatFromEnv(name string, fallback float64) (float64, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseFloat(value, 64)
	if err != nil || parsed < 0 {
		return 0, fmt.Errorf("%s must be a non-negative number: %q", name, value)
	}
	return parsed, nil
}

func boundedFloatFromEnv(name string, fallback, minValue, maxValue float64) (float64, error) {
	parsed, err := nonNegativeFloatFromEnv(name, fallback)
	if err != nil {
		return 0, err
	}
	if parsed < minValue || parsed > maxValue {
		return 0, fmt.Errorf("%s must be between %.0f and %.0f: %q", name, minValue, maxValue, os.Getenv(name))
	}
	return parsed, nil
}
