// Package config loads and validates runtime configuration.
package config

import (
	"fmt"
	"os"
	"strconv"
	"time"
)

const (
	defaultDatabaseURL            = "postgres://swap_chain:swap_chain@127.0.0.1:5432/swap_chain?sslmode=disable"
	defaultMigrationsURL          = "file://migrations"
	defaultHTTPAddress            = ":8080"
	defaultDBConnectTimeout       = 10 * time.Second
	defaultShutdownTimeout        = 10 * time.Second
	defaultSimilarItemsAmount     = 20
	defaultChainLength            = 3
	defaultPenaltyFactor          = 0.25
	defaultChainThreshold         = 0.30
	defaultCompatibilityThreshold = 0.50
	defaultUndefinedCategoryID    = 47
	defaultUndefinedThreshold     = 0.60
	defaultCategorySimilarity     = 0.65
	defaultCategoryMargin         = 0.05
	defaultCategoryModel          = "google/gemini-2.5-flash"
	defaultCategoryModelTimeout   = 30 * time.Second
	defaultAnalysisPoll           = 30 * time.Second
	defaultAnalysisTimeout        = 5 * time.Minute
	defaultAnalysisStale          = 6 * time.Minute
	defaultAnalysisBatch          = 100
	defaultAnalysisBootstrap      = 5 * time.Minute
	defaultAnalysisConcurrency    = 3
	defaultAdminMaxListLimit      = 100
	defaultChatMaxListLimit       = 100
	defaultChatMaxWait            = 25 * time.Second
	defaultCORSAllowedOrigin      = "http://localhost:5173"
	defaultSessionTTL             = 24 * time.Hour
	defaultOllamaBaseURL          = "http://localhost:11434"
	defaultOllamaChatModel        = "llama3.1"
	defaultOllamaEmbedModel       = "qwen3-embedding:4b"
	defaultOllamaEmbedDimensions  = 1024
	defaultOllamaTimeout          = 4 * time.Minute
	defaultMinIOEndpoint          = "localhost:9000"
	defaultMinIOAccessKey         = "minioadmin"
	defaultMinIOSecretKey         = "minioadmin"
	defaultMinIOBucket            = "swap-chain-media"
	defaultMediaMaxBytes          = 10 << 20
)

// Config contains runtime settings loaded from environment variables.
type Config struct {
	DatabaseURL                     string
	MigrationsURL                   string
	HTTPAddress                     string
	DBConnectTimeout                time.Duration
	ShutdownTimeout                 time.Duration
	SimilarItemsAmount              int
	ChainLength                     int
	PenaltyFactor                   float64
	ChainThreshold                  float64
	CompatibilityThreshold          float64
	UndefinedCategoryID             int32
	UndefinedCompatibilityThreshold float64
	MatchingDebug                   bool
	DisableMatchingWorker           bool
	CategorySimilarityThreshold     float64
	CategoryConfidenceMargin        float64
	CategoryModel                   string
	CategoryModelTimeout            time.Duration
	AnalysisPollInterval            time.Duration
	AnalysisTimeout                 time.Duration
	AnalysisStaleAfter              time.Duration
	AnalysisBatchSize               int
	AnalysisBootstrapTimeout        time.Duration
	AnalysisConcurrency             int
	AdminMaxListLimit               int
	ChatMaxListLimit                int
	ChatMaxWait                     time.Duration
	CORSAllowedOrigin               string
	SessionTTL                      time.Duration
	CookieSecure                    bool
	OllamaBaseURL                   string
	OllamaChatModel                 string
	OllamaEmbeddingsModel           string
	OllamaEmbeddingsDimensions      int
	OllamaTimeout                   time.Duration
	OpenRouterAPIKey                string
	OpenRouterModel                 string
	VoyageAPIKey                    string
	VoyageModel                     string
	GigaChatAuthKey                 string
	MinIOEndpoint                   string
	MinIOAccessKey                  string
	MinIOSecretKey                  string
	MinIOBucket                     string
	MinIOUseSSL                     bool
	MediaMaxUploadBytes             int64
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
		DatabaseURL:           envOrDefault("DATABASE_URL", defaultDatabaseURL),
		MigrationsURL:         envOrDefault("MIGRATIONS_URL", defaultMigrationsURL),
		HTTPAddress:           envOrDefault("HTTP_ADDR", defaultHTTPAddress),
		CORSAllowedOrigin:     envOrDefault("CORS_ALLOWED_ORIGIN", defaultCORSAllowedOrigin),
		OllamaBaseURL:         envOrDefault("OLLAMA_BASE_URL", defaultOllamaBaseURL),
		OllamaChatModel:       envOrDefault("OLLAMA_CHAT_MODEL", defaultOllamaChatModel),
		OllamaEmbeddingsModel: envOrDefault("OLLAMA_EMBEDDINGS_MODEL", defaultOllamaEmbedModel),
		MinIOEndpoint:         envOrDefault("MINIO_ENDPOINT", defaultMinIOEndpoint),
		MinIOAccessKey:        envOrDefault("MINIO_ACCESS_KEY", defaultMinIOAccessKey),
		MinIOSecretKey:        envOrDefault("MINIO_SECRET_KEY", defaultMinIOSecretKey),
		MinIOBucket:           envOrDefault("MINIO_BUCKET", defaultMinIOBucket),
	}

	var err error
	if cfg.DBConnectTimeout, err = durationFromEnv("DB_CONNECT_TIMEOUT", defaultDBConnectTimeout); err != nil {
		return Config{}, err
	}
	if cfg.ShutdownTimeout, err = durationFromEnv("SHUTDOWN_TIMEOUT", defaultShutdownTimeout); err != nil {
		return Config{}, err
	}
	if cfg.SessionTTL, err = durationFromEnv("SESSION_TTL", defaultSessionTTL); err != nil {
		return Config{}, err
	}
	if cfg.OllamaTimeout, err = durationFromEnv("OLLAMA_TIMEOUT", defaultOllamaTimeout); err != nil {
		return Config{}, err
	}
	cfg.OpenRouterAPIKey = os.Getenv("OPENROUTER_API_KEY")
	cfg.OpenRouterModel = os.Getenv("OPENROUTER_MODEL")
	cfg.CategoryModel = envOrDefault("ANALYSIS_CATEGORY_MODEL", defaultCategoryModel)
	cfg.VoyageAPIKey = os.Getenv("VOYAGE_API_KEY")
	cfg.VoyageModel = os.Getenv("VOYAGE_MODEL")
	cfg.GigaChatAuthKey = os.Getenv("GIGACHAT_AUTH_KEY")
	if cfg.CookieSecure, err = boolFromEnv("COOKIE_SECURE", false); err != nil {
		return Config{}, err
	}
	if cfg.MinIOUseSSL, err = boolFromEnv("MINIO_USE_SSL", false); err != nil {
		return Config{}, err
	}
	if cfg.MediaMaxUploadBytes, err = positiveInt64FromEnv("MEDIA_MAX_UPLOAD_BYTES", defaultMediaMaxBytes); err != nil {
		return Config{}, err
	}
	if cfg.SimilarItemsAmount, err = positiveIntFromEnv("MATCHING_SIMILAR_ITEMS", defaultSimilarItemsAmount); err != nil {
		return Config{}, err
	}
	if cfg.ChainLength, err = positiveIntFromEnv("MATCHING_CHAIN_LENGTH", defaultChainLength); err != nil {
		return Config{}, err
	}
	if cfg.ChainLength < 2 || cfg.ChainLength > 3 {
		return Config{}, fmt.Errorf("MATCHING_CHAIN_LENGTH must be between 2 and 3")
	}
	if cfg.PenaltyFactor, err = nonNegativeFloatFromEnv("MATCHING_PENALTY_FACTOR", defaultPenaltyFactor); err != nil {
		return Config{}, err
	}
	if cfg.ChainThreshold, err = boundedFloatFromEnv("MATCHING_CHAIN_THRESHOLD", defaultChainThreshold, 0, 1); err != nil {
		return Config{}, err
	}
	if cfg.CompatibilityThreshold, err = boundedFloatFromEnv("MATCHING_COMPATIBILITY_THRESHOLD", defaultCompatibilityThreshold, 0, 1); err != nil {
		return Config{}, err
	}
	if cfg.UndefinedCategoryID, err = positiveInt32FromEnv("UNDEFINED_CATEGORY_ID", defaultUndefinedCategoryID); err != nil {
		return Config{}, err
	}
	if cfg.UndefinedCompatibilityThreshold, err = boundedFloatFromEnv("MATCHING_UNDEFINED_COMPATIBILITY_THRESHOLD", defaultUndefinedThreshold, 0, 1); err != nil {
		return Config{}, err
	}
	if cfg.UndefinedCompatibilityThreshold <= cfg.CompatibilityThreshold {
		return Config{}, fmt.Errorf("MATCHING_UNDEFINED_COMPATIBILITY_THRESHOLD must be greater than MATCHING_COMPATIBILITY_THRESHOLD")
	}
	if cfg.MatchingDebug, err = boolFromEnv("MATCHING_DEBUG", false); err != nil {
		return Config{}, err
	}
	if cfg.DisableMatchingWorker, err = boolFromEnv("DISABLE_MATCHING_WORKER", false); err != nil {
		return Config{}, err
	}
	if cfg.CategorySimilarityThreshold, err = boundedFloatFromEnv("ANALYSIS_CATEGORY_SIMILARITY_THRESHOLD", defaultCategorySimilarity, 0, 1); err != nil {
		return Config{}, err
	}
	if cfg.CategoryConfidenceMargin, err = boundedFloatFromEnv("ANALYSIS_CATEGORY_CONFIDENCE_MARGIN", defaultCategoryMargin, 0, 1); err != nil {
		return Config{}, err
	}
	if cfg.CategoryModelTimeout, err = durationFromEnv("ANALYSIS_CATEGORY_MODEL_TIMEOUT", defaultCategoryModelTimeout); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisPollInterval, err = durationFromEnv("ANALYSIS_RECOVERY_POLL_INTERVAL", defaultAnalysisPoll); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisTimeout, err = durationFromEnv("ANALYSIS_TIMEOUT", defaultAnalysisTimeout); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisStaleAfter, err = durationFromEnv("ANALYSIS_STALE_AFTER", defaultAnalysisStale); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisStaleAfter <= cfg.AnalysisTimeout {
		return Config{}, fmt.Errorf("ANALYSIS_STALE_AFTER must be greater than ANALYSIS_TIMEOUT")
	}
	if cfg.AnalysisBatchSize, err = positiveIntFromEnv("ANALYSIS_RECOVERY_BATCH_SIZE", defaultAnalysisBatch); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisBootstrapTimeout, err = durationFromEnv("ANALYSIS_BOOTSTRAP_TIMEOUT", defaultAnalysisBootstrap); err != nil {
		return Config{}, err
	}
	if cfg.AnalysisConcurrency, err = positiveIntFromEnv("ANALYSIS_CONCURRENCY", defaultAnalysisConcurrency); err != nil {
		return Config{}, err
	}
	if cfg.AdminMaxListLimit, err = positiveIntFromEnv("ADMIN_MAX_LIST_LIMIT", defaultAdminMaxListLimit); err != nil {
		return Config{}, err
	}
	if cfg.ChatMaxListLimit, err = positiveIntFromEnv("CHAT_MAX_LIST_LIMIT", defaultChatMaxListLimit); err != nil {
		return Config{}, err
	}
	if cfg.ChatMaxWait, err = durationFromEnv("CHAT_MAX_WAIT", defaultChatMaxWait); err != nil {
		return Config{}, err
	}
	if cfg.OllamaEmbeddingsDimensions, err = positiveIntFromEnv("OLLAMA_EMBEDDINGS_DIMENSIONS", defaultOllamaEmbedDimensions); err != nil {
		return Config{}, err
	}

	return cfg, nil
}

func boolFromEnv(name string, fallback bool) (bool, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseBool(value)
	if err != nil {
		return false, fmt.Errorf("%s must be a boolean: %q", name, value)
	}
	return parsed, nil
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

func positiveInt64FromEnv(name string, fallback int64) (int64, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 64)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive integer: %q", name, value)
	}
	return parsed, nil
}

func positiveInt32FromEnv(name string, fallback int32) (int32, error) {
	value := os.Getenv(name)
	if value == "" {
		return fallback, nil
	}

	parsed, err := strconv.ParseInt(value, 10, 32)
	if err != nil || parsed <= 0 {
		return 0, fmt.Errorf("%s must be a positive 32-bit integer: %q", name, value)
	}
	return int32(parsed), nil
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
