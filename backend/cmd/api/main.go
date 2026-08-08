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

	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/config"
	"swap-chain/internal/infrastructure/postgres"
	httptransport "swap-chain/internal/transport/http"
	"swap-chain/matching/adapters"
	"swap-chain/matching/service"
	"swap-chain/shared/db"

	"go.uber.org/zap"
)

const (
	readHeaderTimeout = 5 * time.Second
	readTimeout       = 15 * time.Second
	writeTimeout      = 30 * time.Second
	idleTimeout       = 60 * time.Second
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
	matcher := service.NewMatching(
		logger,
		adapters.NewOllama(),
		queries,
		service.NewScoring(),
		service.MatchingConfig{
			SimilarItemsAmount:   cfg.SimilarItemsAmount,
			ChainLen:             cfg.ChainLength,
			PenaltyFactor:        cfg.PenaltyFactor,
			ChainRatingThreshold: cfg.ChainThreshold,
		},
	)
	finder := applicationmatching.NewFindCycles(matcher)
	handler := httptransport.NewHandler(database, finder, logger)

	server := &http.Server{
		Addr:              cfg.HTTPAddress,
		Handler:           handler.Routes(),
		ReadHeaderTimeout: readHeaderTimeout,
		ReadTimeout:       readTimeout,
		WriteTimeout:      writeTimeout,
		IdleTimeout:       idleTimeout,
	}

	serverErrors := make(chan error, 1)
	go func() {
		logger.Info("backend started", zap.String("address", cfg.HTTPAddress))
		serverErrors <- server.ListenAndServe()
	}()

	shutdownSignal, stopSignals := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stopSignals()

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
