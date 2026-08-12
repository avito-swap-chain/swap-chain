// Package main starts the swap-chain HTTP API.
package main

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/categories"
	"swap-chain/internal/chains"
	"swap-chain/internal/config"
	"swap-chain/internal/events"
	"swap-chain/internal/httpapi"
	"swap-chain/internal/httpserver"
	"swap-chain/internal/infrastructure/postgres"
	"swap-chain/internal/items"
	"swap-chain/internal/media"
	"swap-chain/internal/outbox"
	"swap-chain/internal/session"
	"swap-chain/internal/users"
	adminmodel "swap-chain/modules/admin/model"
	adminrepository "swap-chain/modules/admin/repository"
	adminservice "swap-chain/modules/admin/service"
	"swap-chain/modules/analyze/adapters"
	analyzerepository "swap-chain/modules/analyze/repository"
	analyzeservice "swap-chain/modules/analyze/service"
	chatmodel "swap-chain/modules/chat/model"
	chatrepository "swap-chain/modules/chat/repository"
	chatservice "swap-chain/modules/chat/service"
	matchingrepository "swap-chain/modules/matching/repository"
	"swap-chain/modules/matching/service"
	metricsrepository "swap-chain/modules/metrics/repository"
	metricsservice "swap-chain/modules/metrics/service"
	notificationrepository "swap-chain/modules/notifications/repository"
	notificationservice "swap-chain/modules/notifications/service"
	reputationrepository "swap-chain/modules/reputation/repository"
	reputationservice "swap-chain/modules/reputation/service"
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

	outboxStore, err := outbox.NewStore(database)
	if err != nil {
		return fmt.Errorf("create outbox store: %w", err)
	}
	realtimeBroadcaster, err := outbox.NewPostgreSQLBroadcaster(database, cfg.DatabaseURL, eventHub, logger)
	if err != nil {
		return fmt.Errorf("create realtime broadcaster: %w", err)
	}
	defer func() {
		if err := realtimeBroadcaster.Close(); err != nil {
			logger.Warn("close realtime broadcaster", zap.Error(err))
		}
	}()
	outboxWorker, err := outbox.NewWorker(outboxStore, realtimeBroadcaster, logger, outbox.DefaultWorkerConfig())
	if err != nil {
		return fmt.Errorf("create outbox worker: %w", err)
	}

	sessions := session.NewManager(cfg.SessionTTL, cfg.CookieSecure)

	ollamaConfig := adapters.DefaultOllamaConfig()
	ollamaConfig.BaseURL = cfg.OllamaBaseURL
	ollamaConfig.ChatModel = cfg.OllamaChatModel
	ollamaConfig.EmbeddingsModel = cfg.OllamaEmbeddingsModel
	ollamaConfig.EmbeddingsDimensions = cfg.OllamaEmbeddingsDimensions
	ollamaConfig.Timeout = cfg.OllamaTimeout
	ollamaClient, err := adapters.NewOllama(ollamaConfig)
	if err != nil {
		return fmt.Errorf("create ollama client: %w", err)
	}
	var enricher adapters.Enricher = ollamaClient
	var embedder adapters.Embedder = ollamaClient
	var visionAdapter analyzeservice.VisionAdapter
	var visionService httpapi.VisionService

	var gigaChatClient *adapters.GigaChat
	if cfg.GigaChatAuthKey != "" {
		gigaChatConfig := adapters.DefaultGigaChatConfig(cfg.GigaChatAuthKey)
		gigaChatConfig.OnCleanupError = func(err error) {
			logger.Warn("failed to remove temporary GigaChat file", zap.Error(err))
		}
		gcClient, err := adapters.NewGigaChat(gigaChatConfig)
		if err != nil {
			return fmt.Errorf("create gigachat client: %w", err)
		}
		gigaChatClient = gcClient

		enricher, err = adapters.NewFallbackClient(gigaChatClient, enricher)
		if err != nil {
			return fmt.Errorf("create LLM fallback client: %w", err)
		}
		visionAdapter = gigaChatClient
	}

	var openRouterClient *adapters.OpenRouter
	if cfg.OpenRouterAPIKey != "" {
		openRouterConfig := adapters.DefaultOpenRouterConfig(cfg.OpenRouterAPIKey)
		if cfg.OpenRouterModel != "" {
			openRouterConfig.Model = cfg.OpenRouterModel
		}
		openRouterConfig.Dimensions = cfg.OllamaEmbeddingsDimensions
		orClient, err := adapters.NewOpenRouter(openRouterConfig)
		if err != nil {
			return fmt.Errorf("create openrouter client: %w", err)
		}
		openRouterClient = orClient


		enricher, err = adapters.NewFallbackClient(openRouterClient, enricher)
		if err != nil {
			return fmt.Errorf("create openrouter enricher fallback: %w", err)
		}
		if visionAdapter != nil {
			visionAdapter, _ = adapters.NewVisionFallbackClient(openRouterClient, visionAdapter)
		} else {
			visionAdapter = openRouterClient
		}
	}

	if cfg.VoyageAPIKey != "" {
		voyageConfig := adapters.DefaultVoyageConfig(cfg.VoyageAPIKey)
		voyageClient, err := adapters.NewVoyage(voyageConfig)
		if err != nil {
			return fmt.Errorf("create voyage client: %w", err)
		}
		embedder, err = adapters.NewFallbackEmbedder(voyageClient, embedder)
		if err != nil {
			return fmt.Errorf("create voyage embedder fallback: %w", err)
		}
	}

	if visionAdapter != nil {
		vision, vsErr := analyzeservice.NewVision(visionAdapter)
		if vsErr != nil {
			return fmt.Errorf("create vision service: %w", vsErr)
		}
		visionService = vision
		logger.Info("vision photo recognition enabled")
	} else {
		logger.Info("vision photo recognition disabled (no API keys set)")
	}

	vectorizer, err := analyzeservice.NewVectorizer(enricher, embedder)
	if err != nil {
		return fmt.Errorf("create vectorizer: %w", err)
	}
	analysisRepo, err := analyzerepository.NewPostgreSQLAnalysis(queries)
	if err != nil {
		return fmt.Errorf("create analysis repo: %w", err)
	}
	categoryBootstrap, err := analyzeservice.NewCategoryBootstrap(analysisRepo, embedder)
	if err != nil {
		return fmt.Errorf("create category bootstrap: %w", err)
	}
	bootstrapCtx, cancelBootstrap := context.WithTimeout(context.Background(), cfg.AnalysisBootstrapTimeout)
	if err := ollamaClient.Ready(bootstrapCtx); err != nil {
		cancelBootstrap()
		return fmt.Errorf("wait for ollama models: %w", err)
	}
	if err := categoryBootstrap.Bootstrap(bootstrapCtx); err != nil {
		cancelBootstrap()
		return fmt.Errorf("bootstrap category embeddings: %w", err)
	}
	cancelBootstrap()

	tagging, err := analyzeservice.NewTagging(analysisRepo, analyzeservice.TaggingConfig{
		SimilarityThreshold: cfg.CategorySimilarityThreshold,
		ConfidenceMargin:    cfg.CategoryConfidenceMargin,
		UndefinedCategoryID: cfg.UndefinedCategoryID,
	})
	if err != nil {
		return fmt.Errorf("create tagging: %w", err)
	}
	scoring, err := analyzeservice.NewScoring(enricher)
	if err != nil {
		return fmt.Errorf("create scoring: %w", err)
	}
	analysis, err := analyzeservice.NewAnalysis(analysisRepo, scoring, tagging, vectorizer)
	if err != nil {
		return fmt.Errorf("create analysis: %w", err)
	}
	itemService := items.NewPostgresService(database, analysis, func(userID int64, eventType, entityID string, data map[string]any) {
		eventHub.PublishToUser(userID, eventType, entityID, data)
	}, logger, cfg.AnalysisTimeout)
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
			SimilarItemsAmount:              cfg.SimilarItemsAmount,
			CompatibilityThreshold:          cfg.CompatibilityThreshold,
			UndefinedCategoryID:             cfg.UndefinedCategoryID,
			UndefinedCompatibilityThreshold: cfg.UndefinedCompatibilityThreshold,
			ChainLen:                        cfg.ChainLength,
			PenaltyFactor:                   cfg.PenaltyFactor,
			ChainRatingThreshold:            cfg.ChainThreshold,
			Debug:                           cfg.MatchingDebug,
		},
	)
	if err != nil {
		return fmt.Errorf("create matching: %w", err)
	}

	notificationRepo, err := notificationrepository.NewPostgres(database)
	if err != nil {
		return fmt.Errorf("create notification repository: %w", err)
	}
	notificationService, err := notificationservice.New(notificationRepo)
	if err != nil {
		return fmt.Errorf("create notification service: %w", err)
	}
	notificationProducer := notificationservice.NewProducer(notificationService, logger, func(userID int64, eventType, entityID string, data map[string]any) {
		eventHub.PublishToUser(userID, eventType, entityID, data)
	})

	chainService := chains.NewPostgresServiceWithOutbox(database, func(userIDs []int64, eventType, entityID string, data map[string]any) {
		chainID, _ := strconv.ParseInt(entityID, 10, 64)
		ctx := context.Background()
		switch eventType {
		case "chain.created":
			notificationProducer.NotifyChainCreated(ctx, userIDs, chainID)
		case "chain.updated":
			notificationProducer.NotifyChainUpdated(ctx, userIDs, chainID)
		case "chain.accepted":
			notificationProducer.NotifyChainAccepted(ctx, userIDs, chainID)
		case "chain.rejected":
			reason := ""
			if v, ok := data["reason"]; ok {
				if r, ok2 := v.(string); ok2 {
					reason = r
				}
			}
			notificationProducer.NotifyChainRejected(ctx, userIDs, chainID, reason)
		}
	}, outboxStore)

	finder := applicationmatching.NewFindCycles(matcher, chainService)
	matchingJobs, err := postgres.NewMatchingJobs(database)
	if err != nil {
		return fmt.Errorf("create matching jobs repository: %w", err)
	}
	materializer, err := applicationmatching.NewMaterializer(itemService, finder, chainService)
	if err != nil {
		return fmt.Errorf("create matching materializer: %w", err)
	}
	matchingWorker, err := applicationmatching.NewWorker(
		matchingJobs,
		materializer,
		logger,
		applicationmatching.DefaultWorkerConfig(),
	)
	if err != nil {
		return fmt.Errorf("create matching worker: %w", err)
	}

	userService := users.NewPostgresService(database)

	reputationRepo, err := reputationrepository.NewPostgreSQL(database)
	if err != nil {
		return fmt.Errorf("create reputation repository: %w", err)
	}
	reputationModule, err := reputationservice.New(reputationRepo)
	if err != nil {
		return fmt.Errorf("create reputation service: %w", err)
	}
	metricsRepo, err := metricsrepository.NewPostgreSQL(database)
	if err != nil {
		return fmt.Errorf("create metrics repository: %w", err)
	}
	metricsModule, err := metricsservice.New(metricsRepo)
	if err != nil {
		return fmt.Errorf("create metrics service: %w", err)
	}
	adminRepo, err := adminrepository.NewPostgreSQLWithOutbox(database, outboxStore)
	if err != nil {
		return fmt.Errorf("create admin repository: %w", err)
	}
	adminModule, err := adminservice.NewWithCallback(adminRepo, func(ctx context.Context, delivery adminmodel.Delivery) {
		switch delivery.Status {
		case adminmodel.DeliveryAtPVZ:
			notificationProducer.NotifyDeliveryAtPVZ(ctx, delivery.RecipientID, delivery.ItemTitle, delivery.ItemID)
		case adminmodel.DeliveryInTransit:
			notificationProducer.NotifyDeliveryInTransit(ctx, delivery.RecipientID, delivery.ItemTitle, delivery.ItemID)
		}
	})
	if err != nil {
		return fmt.Errorf("create admin service: %w", err)
	}
	chatRepo, err := chatrepository.NewPostgreSQL(database)
	if err != nil {
		return fmt.Errorf("create chat repository: %w", err)
	}
	chatModule, err := chatservice.NewWithCallback(chatRepo, func(ctx context.Context, message chatmodel.Message) {
		eventHub.PublishToUser(message.Recipient.ID, "chat.message.created", strconv.FormatInt(message.ChainID, 10), nil)
		notificationProducer.NotifyChatMessage(ctx, message.Recipient.ID, message.Sender.Username, message.ChainID)
	})
	if err != nil {
		return fmt.Errorf("create chat service: %w", err)
	}

	categoriesService := categories.NewPostgresService(queries, cfg.UndefinedCategoryID)

	handler := httpapi.NewHandler(
		database,
		finder,
		logger,
		itemService,
		mediaService,
		chainService,
		categoriesService,
		eventHub,
		sessions,
		userService,
		adminModule,
		chatModule,
		notificationService,
		reputationModule,
		metricsModule,
		visionService,
		ollamaClient,
		categoryBootstrap,
	)

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
	recoveryWorker, err := analyzeservice.NewAnalysisRecoveryWorker(analysisRepo, analysis, logger, analyzeservice.AnalysisRecoveryWorkerConfig{
		PollInterval: cfg.AnalysisPollInterval,
		StaleAfter:   cfg.AnalysisStaleAfter,
		BatchSize:    int32(cfg.AnalysisBatchSize),
	})
	if err != nil {
		return fmt.Errorf("create analysis recovery worker: %w", err)
	}
	go recoveryWorker.Start(shutdownSignal)
	if !cfg.DisableMatchingWorker {
		go matchingWorker.Start(shutdownSignal)
	}
	go realtimeBroadcaster.Run(shutdownSignal)
	go outboxWorker.Run(shutdownSignal)
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
