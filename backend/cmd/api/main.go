// Package main starts the swap-chain HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"swap-chain/analyze/adapters"
	analyzeservice "swap-chain/analyze/service"
	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/chains"
	"swap-chain/internal/config"
	"swap-chain/internal/events"
	"swap-chain/internal/httpapi"
	"swap-chain/internal/httpserver"
	"swap-chain/internal/infrastructure/postgres"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	matchingrepository "swap-chain/matching/repository"
	"swap-chain/matching/service"
	"swap-chain/shared/db"

	"go.uber.org/zap"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	idleTimeout       = 60 * time.Second
	chainExpiryPeriod = time.Minute
	chainExpiryBatch  = 100
)

func main() {
	logger, err := zap.NewProduction()
	if err != nil {
		fmt.Fprintf(os.Stderr, "create logger: %v\n", err)
		os.Exit(1)
	}
	defer func() {
		_ = logger.Sync()
	}()

	if err := run(logger); err != nil {
		logger.Fatal("backend stopped", zap.Error(err))
	}
}

func run(logger *zap.Logger) error {
	cfg, err := config.Load()
	if err != nil {
		return fmt.Errorf("load config: %w", err)
	}

	connectCtx, cancelConnect := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	defer cancelConnect()

	database, err := postgres.Open(connectCtx, cfg.DatabaseURL)
	if err != nil {
		return err
	}
	defer func() {
		if closeErr := database.Close(); closeErr != nil {
			logger.Error("close database", zap.Error(closeErr))
		}
	}()

	queries := db.New(database)
	storageCtx, cancelStorage := context.WithTimeout(context.Background(), cfg.DBConnectTimeout)
	mediaStorage, err := media.NewMinIOStorage(storageCtx, media.MinIOConfig{
		Endpoint:  cfg.MinIOEndpoint,
		AccessKey: cfg.MinIOAccessKey,
		SecretKey: cfg.MinIOSecretKey,
		Bucket:    cfg.MinIOBucket,
		UseSSL:    cfg.MinIOUseSSL,
	})
	cancelStorage()
	if err != nil {
		return fmt.Errorf("initialize media storage: %w", err)
	}
	mediaService := media.NewService(mediaStorage, cfg.MediaMaxUploadBytes)
	eventHub := events.NewHub()
	sessions := session.NewManager(cfg.SessionTTL, cfg.CookieSecure)
	ollamaConfig := adapters.DefaultOllamaConfig()
	ollamaConfig.BaseURL = cfg.OllamaBaseURL
	ollamaConfig.ChatModel = cfg.OllamaChatModel
	ollamaConfig.EmbeddingsModel = cfg.OllamaEmbeddingsModel
	llmClient, err := adapters.NewOllama(ollamaConfig)
	if err != nil {
		return fmt.Errorf("create ollama client: %w", err)
	}
	vectorizer, err := analyzeservice.NewVectorizer(llmClient, llmClient)
	if err != nil {
		return fmt.Errorf("create vectorizer: %w", err)
	}
	tagging, err := analyzeservice.NewTagging(vectorizer, queries, analyzeservice.TaggingConfig{
		SimilarityThreshold: cfg.CategorySimilarityThreshold,
		ConfidenceMargin:    cfg.CategoryConfidenceMargin,
	})
	if err != nil {
		return fmt.Errorf("create tagging: %w", err)
	}
	scoring, err := analyzeservice.NewScoring(llmClient)
	if err != nil {
		return fmt.Errorf("create scoring: %w", err)
	}
	analysis, err := analyzeservice.NewAnalysis(queries, scoring, tagging, vectorizer)
	if err != nil {
		return fmt.Errorf("create analysis: %w", err)
	}
	itemService := items.NewPostgresService(database, analysis, func(userID int64, eventType, entityID string, data map[string]any) {
		eventHub.PublishToUser(userID, eventType, entityID, data)
	}, logger)
	defer itemService.Close()
	matchingRepo, err := matchingrepository.NewPostgreSQLMatching(queries)
	if err != nil {
		return fmt.Errorf("create matching repo: %w", err)
	}
	matcher, err := service.NewMatching(
		logger,
		matchingRepo,
		service.NewScoring(),
		service.MatchingConfig{
			SimilarItemsAmount:     cfg.SimilarItemsAmount,
			CompatibilityThreshold: cfg.CompatibilityThreshold,
			ChainLen:               cfg.ChainLength,
			PenaltyFactor:          cfg.PenaltyFactor,
			ChainRatingThreshold:   cfg.ChainThreshold,
		},
	)
	if err != nil {
		return fmt.Errorf("create matching: %w", err)
	}
	chainService := chains.NewPostgresService(database, func(userIDs []int64, eventType, entityID string, data map[string]any) {
		eventHub.PublishToUsers(userIDs, eventType, entityID, data)
	})
	finder := applicationmatching.NewFindCycles(matcher, chainService)
	userService := users.NewPostgresService(database)
	handler := httpapi.NewHandler(database, finder, logger, itemService, mediaService, chainService, eventHub, sessions, userService)
	router, err := httpserver.New(logger, handler, sessions, cfg.CORSAllowedOrigin, cfg.MediaMaxUploadBytes)
	if err != nil {
		return fmt.Errorf("create HTTP router: %w", err)
	}

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           router,
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		IdleTimeout:       idleTimeout,
	}
	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()
	recoveryWorker, err := analyzeservice.NewAnalysisRecoveryWorker(queries, analysis, logger, analyzeservice.AnalysisRecoveryWorkerConfig{
		PollInterval: cfg.AnalysisPollInterval,
		StaleAfter:   cfg.AnalysisStaleAfter,
		BatchSize:    int32(cfg.AnalysisBatchSize),
	})
	if err != nil {
		return fmt.Errorf("create analysis recovery worker: %w", err)
	}
	go recoveryWorker.Start(shutdownSignal)
	go runChainExpiry(shutdownSignal, chainService, logger)

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("backend started", zap.String("address", cfg.HTTPAddress))
		serverErrors <- server.ListenAndServe()
	}()

	select {
	case err := <-serverErrors:
		if !errors.Is(err, http.ErrServerClosed) {
			return fmt.Errorf("serve HTTP: %w", err)
		}
		return nil
	case <-shutdownSignal.Done():
		logger.Info("shutting down backend")
	}

	shutdownCtx, cancelShutdown := context.WithTimeout(context.Background(), cfg.ShutdownTimeout)
	defer cancelShutdown()
	if err := server.Shutdown(shutdownCtx); err != nil {
		return fmt.Errorf("shutdown HTTP server: %w", err)
	}

	return nil
}

func runChainExpiry(ctx context.Context, service chains.Service, logger *zap.Logger) {
	ticker := time.NewTicker(chainExpiryPeriod)
	defer ticker.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			expired, err := service.ExpirePending(ctx, chainExpiryBatch)
			if err != nil {
				logger.Error("expire pending chains", zap.Error(err))
				continue
			}
			if expired > 0 {
				logger.Info("expired pending chains", zap.Int("count", expired))
			}
		}
	}
}
