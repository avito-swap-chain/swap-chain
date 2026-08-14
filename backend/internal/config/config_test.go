package config

import (
	"strings"
	"testing"
	"time"
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
		"MATCHING_COMPATIBILITY_THRESHOLD",
		"MATCHING_DEBUG",
		"UNDEFINED_CATEGORY_ID",
		"ANALYSIS_CATEGORY_SIMILARITY_THRESHOLD",
		"ANALYSIS_CATEGORY_CONFIDENCE_MARGIN",
		"ANALYSIS_CATEGORY_MODEL",
		"ANALYSIS_CATEGORY_MODEL_TIMEOUT",
		"ANALYSIS_RECOVERY_POLL_INTERVAL",
		"ANALYSIS_TIMEOUT",
		"ANALYSIS_STALE_AFTER",
		"ANALYSIS_RECOVERY_BATCH_SIZE",
		"ANALYSIS_BOOTSTRAP_TIMEOUT",
		"CORS_ALLOWED_ORIGIN",
		"SESSION_TTL",
		"COOKIE_SECURE",
		"OLLAMA_BASE_URL",
		"OLLAMA_CHAT_MODEL",
		"OLLAMA_EMBEDDINGS_MODEL",
		"OLLAMA_TIMEOUT",
		"GIGACHAT_AUTH_KEY",
		"MINIO_ENDPOINT",
		"MINIO_ACCESS_KEY",
		"MINIO_SECRET_KEY",
		"MINIO_BUCKET",
		"MINIO_USE_SSL",
		"MEDIA_MAX_UPLOAD_BYTES",
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
	if cfg.OllamaBaseURL != defaultOllamaBaseURL || cfg.OllamaChatModel != defaultOllamaChatModel ||
		cfg.OllamaEmbeddingsModel != defaultOllamaEmbedModel || cfg.OllamaTimeout != defaultOllamaTimeout {
		t.Fatalf("unexpected Ollama defaults: %+v", cfg)
	}
	if cfg.MinIOEndpoint != defaultMinIOEndpoint || cfg.MinIOBucket != defaultMinIOBucket || cfg.MediaMaxUploadBytes != defaultMediaMaxBytes {
		t.Fatalf("unexpected media defaults: %+v", cfg)
	}
	if cfg.CategoryModel != defaultCategoryModel || cfg.CategoryModelTimeout != defaultCategoryModelTimeout {
		t.Fatalf("unexpected category model defaults: %+v", cfg)
	}
}

func TestLoadReadsGigaChatAuthKey(t *testing.T) {
	t.Setenv("GIGACHAT_AUTH_KEY", "auth-key")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.GigaChatAuthKey != "auth-key" {
		t.Fatalf("GigaChatAuthKey = %q, want auth-key", cfg.GigaChatAuthKey)
	}
}

func TestLoadReadsOllamaSettings(t *testing.T) {
	t.Setenv("OLLAMA_BASE_URL", "http://ollama:11434")
	t.Setenv("OLLAMA_CHAT_MODEL", "chat-model")
	t.Setenv("OLLAMA_EMBEDDINGS_MODEL", "embedding-model")
	t.Setenv("OLLAMA_TIMEOUT", "4m")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.OllamaBaseURL != "http://ollama:11434" || cfg.OllamaChatModel != "chat-model" ||
		cfg.OllamaEmbeddingsModel != "embedding-model" || cfg.OllamaTimeout != 4*time.Minute {
		t.Fatalf("unexpected Ollama config: %+v", cfg)
	}
}

func TestLoadReadsSessionSettings(t *testing.T) {
	t.Setenv("CORS_ALLOWED_ORIGIN", "http://localhost:3000")
	t.Setenv("SESSION_TTL", "2h")
	t.Setenv("COOKIE_SECURE", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CORSAllowedOrigin != "http://localhost:3000" || cfg.SessionTTL != 2*time.Hour || !cfg.CookieSecure {
		t.Fatalf("unexpected session config: %+v", cfg)
	}
}

func TestLoadReadsMediaSettings(t *testing.T) {
	t.Setenv("MINIO_ENDPOINT", "minio:9000")
	t.Setenv("MINIO_ACCESS_KEY", "access")
	t.Setenv("MINIO_SECRET_KEY", "secret")
	t.Setenv("MINIO_BUCKET", "images")
	t.Setenv("MINIO_USE_SSL", "true")
	t.Setenv("MEDIA_MAX_UPLOAD_BYTES", "2048")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.MinIOEndpoint != "minio:9000" || cfg.MinIOAccessKey != "access" || cfg.MinIOSecretKey != "secret" || cfg.MinIOBucket != "images" || !cfg.MinIOUseSSL || cfg.MediaMaxUploadBytes != 2048 {
		t.Fatalf("unexpected media config: %+v", cfg)
	}
}

func TestLoadRejectsInvalidChainLength(t *testing.T) {
	for _, value := range []string{"1", "4"} {
		t.Run(value, func(t *testing.T) {
			t.Setenv("MATCHING_CHAIN_LENGTH", value)

			_, err := Load()
			if err == nil || !strings.Contains(err.Error(), "MATCHING_CHAIN_LENGTH must be between 2 and 3") {
				t.Fatalf("Load() error = %v, want chain length error", err)
			}
		})
	}
}

func TestLoadReadsMatchingSettings(t *testing.T) {
	t.Setenv("MATCHING_SIMILAR_ITEMS", "12")
	t.Setenv("MATCHING_CHAIN_LENGTH", "3")
	t.Setenv("MATCHING_PENALTY_FACTOR", "0.4")
	t.Setenv("MATCHING_CHAIN_THRESHOLD", "0.5")
	t.Setenv("MATCHING_COMPATIBILITY_THRESHOLD", "0.6")
	t.Setenv("UNDEFINED_CATEGORY_ID", "47")
	t.Setenv("MATCHING_DEBUG", "true")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}

	if cfg.SimilarItemsAmount != 12 || cfg.ChainLength != 3 || cfg.PenaltyFactor != 0.4 || cfg.ChainThreshold != 0.5 ||
		cfg.CompatibilityThreshold != 0.6 || cfg.UndefinedCategoryID != 47 || !cfg.MatchingDebug {
		t.Fatalf("unexpected matching config: %+v", cfg)
	}
}

func TestLoadReadsAnalysisSettings(t *testing.T) {
	t.Setenv("ANALYSIS_CATEGORY_SIMILARITY_THRESHOLD", "0.7")
	t.Setenv("ANALYSIS_CATEGORY_CONFIDENCE_MARGIN", "0.1")
	t.Setenv("ANALYSIS_CATEGORY_MODEL", "google/gemini-2.5-flash")
	t.Setenv("ANALYSIS_CATEGORY_MODEL_TIMEOUT", "20s")
	t.Setenv("ANALYSIS_RECOVERY_POLL_INTERVAL", "15s")
	t.Setenv("ANALYSIS_TIMEOUT", "90s")
	t.Setenv("ANALYSIS_STALE_AFTER", "2m")
	t.Setenv("ANALYSIS_RECOVERY_BATCH_SIZE", "25")
	t.Setenv("ANALYSIS_BOOTSTRAP_TIMEOUT", "90s")

	cfg, err := Load()
	if err != nil {
		t.Fatalf("Load() error = %v", err)
	}
	if cfg.CategorySimilarityThreshold != 0.7 || cfg.CategoryConfidenceMargin != 0.1 ||
		cfg.AnalysisPollInterval != 15*time.Second || cfg.AnalysisTimeout != 90*time.Second ||
		cfg.AnalysisStaleAfter != 2*time.Minute || cfg.AnalysisBatchSize != 25 ||
		cfg.AnalysisBootstrapTimeout != 90*time.Second || cfg.CategoryModel != "google/gemini-2.5-flash" ||
		cfg.CategoryModelTimeout != 20*time.Second {
		t.Fatalf("unexpected analysis config: %+v", cfg)
	}
}

func TestLoadRejectsStaleAnalysisWindowNotGreaterThanTimeout(t *testing.T) {
	t.Setenv("ANALYSIS_TIMEOUT", "5m")
	t.Setenv("ANALYSIS_STALE_AFTER", "5m")

	_, err := Load()
	if err == nil || !strings.Contains(err.Error(), "ANALYSIS_STALE_AFTER must be greater than ANALYSIS_TIMEOUT") {
		t.Fatalf("Load() error = %v, want analysis timeout ordering error", err)
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
