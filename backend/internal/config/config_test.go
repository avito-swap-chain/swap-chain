package config

import (
	"strings"
	"testing"
)

func TestLoadDefaults(t *testing.T) {
	for _, name := range []string{
		"DATABASE_URL",
		"MIGRATIONS_URL",
		"HTTP_ADDR",
		"DB_CONNECT_TIMEOUT",
		"SHUTDOWN_TIMEOUT",
		"MATCHING_SIMILAR_ITEMS",
		"MATCHING_CHAIN_LENGTH",
		"MATCHING_PENALTY_FACTOR",
		"MATCHING_CHAIN_THRESHOLD",
	} {
		t.Setenv(name, "")
	}

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.DatabaseURL != defaultDatabaseURL {
		t.Fatalf("DatabaseURL = %q, want %q", cfg.DatabaseURL, defaultDatabaseURL)
	}
	if cfg.ChainLength != defaultChainLength {
		t.Fatalf("ChainLength = %d, want %d", cfg.ChainLength, defaultChainLength)
	}
}

func TestLoadRejectsInvalidChainLength(t *testing.T) {
	t.Setenv("MATCHING_CHAIN_LENGTH", "1")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "at least 2") {
		t.Fatalf("Load() error = %v, want minimum chain length error", err)
	}
}

func TestLoadReadsMatchingSettings(t *testing.T) {
	t.Setenv("MATCHING_SIMILAR_ITEMS", "12")
	t.Setenv("MATCHING_CHAIN_LENGTH", "3")
	t.Setenv("MATCHING_PENALTY_FACTOR", "0.4")
	t.Setenv("MATCHING_CHAIN_THRESHOLD", "0.5")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SimilarItemsAmount != 12 || cfg.ChainLength != 3 || cfg.PenaltyFactor != 0.4 || cfg.ChainThreshold != 0.5 {
		t.Fatalf("unexpected matching config: %+v", cfg)
	}
}

func TestLoadMigrationIgnoresUnrelatedInvalidSettings(t *testing.T) {
	t.Setenv("DATABASE_URL", "postgres://example")
	t.Setenv("MIGRATIONS_URL", "file://testdata")
	t.Setenv("MATCHING_CHAIN_LENGTH", "invalid")

	cfg := LoadMigration()
	if cfg.DatabaseURL != "postgres://example" || cfg.MigrationsURL != "file://testdata" {
		t.Fatalf("unexpected migration config: %+v", cfg)
	}
}
