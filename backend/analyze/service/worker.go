package service

import (
	"context"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"

	"swap-chain/shared/db"
)

type StaleAnalysisRepository interface {
	ClaimStaleAnalyzingItems(ctx context.Context, arg db.ClaimStaleAnalyzingItemsParams) ([]int64, error)
}

type ItemAnalyzer interface {
	AnalyzeItem(ctx context.Context, itemID int64) error
}

type AnalysisRecoveryWorkerConfig struct {
	PollInterval time.Duration
	StaleAfter   time.Duration
	BatchSize    int32
}

func DefaultAnalysisRecoveryWorkerConfig() AnalysisRecoveryWorkerConfig {
	return AnalysisRecoveryWorkerConfig{
		PollInterval: 30 * time.Second,
		StaleAfter:   5 * time.Minute,
		BatchSize:    100,
	}
}

// AnalysisRecoveryWorker повторно запускает полный анализ зависших ANALYZING-вещей.
type AnalysisRecoveryWorker struct {
	repo     StaleAnalysisRepository
	analyzer ItemAnalyzer
	logger   *zap.Logger
	cfg      AnalysisRecoveryWorkerConfig
	now      func() time.Time
}

func NewAnalysisRecoveryWorker(
	repo StaleAnalysisRepository,
	analyzer ItemAnalyzer,
	logger *zap.Logger,
	cfg AnalysisRecoveryWorkerConfig,
) (*AnalysisRecoveryWorker, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("analysis recovery worker: repository is required")
	case analyzer == nil:
		return nil, fmt.Errorf("analysis recovery worker: analyzer is required")
	case logger == nil:
		return nil, fmt.Errorf("analysis recovery worker: logger is required")
	}

	if cfg.PollInterval <= 0 {
		return nil, fmt.Errorf("analysis recovery worker: poll interval must be positive")
	}
	if cfg.StaleAfter <= 0 {
		return nil, fmt.Errorf("analysis recovery worker: stale duration must be positive")
	}
	if cfg.BatchSize <= 0 {
		return nil, fmt.Errorf("analysis recovery worker: batch size must be positive")
	}

	return &AnalysisRecoveryWorker{
		repo:     repo,
		analyzer: analyzer,
		logger:   logger,
		cfg:      cfg,
		now:      time.Now,
	}, nil
}

func (w *AnalysisRecoveryWorker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	w.recoverAndLog(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("analysis recovery worker stopped")
			return
		case <-ticker.C:
			w.recoverAndLog(ctx)
		}
	}
}

func (w *AnalysisRecoveryWorker) recoverAndLog(ctx context.Context) {
	if err := w.recoverOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("analysis recovery iteration failed", zap.Error(err))
	}
}

func (w *AnalysisRecoveryWorker) recoverOnce(ctx context.Context) error {
	staleBefore := w.now().Add(-w.cfg.StaleAfter)
	itemIDs, err := w.repo.ClaimStaleAnalyzingItems(ctx, db.ClaimStaleAnalyzingItemsParams{
		StaleBefore: staleBefore,
		BatchSize:   w.cfg.BatchSize,
	})
	if err != nil {
		return fmt.Errorf("claim stale analyzing items: %w", err)
	}
	if len(itemIDs) == 0 {
		return nil
	}

	w.logger.Info("claimed stale items for analysis recovery", zap.Int("count", len(itemIDs)))

	var recoveryErrors []error
	for _, itemID := range itemIDs {
		if err := ctx.Err(); err != nil {
			recoveryErrors = append(recoveryErrors, err)
			break
		}

		if err := w.analyzer.AnalyzeItem(ctx, itemID); err != nil {
			itemErr := fmt.Errorf("recover item %d analysis: %w", itemID, err)
			recoveryErrors = append(recoveryErrors, itemErr)
			w.logger.Error("failed to recover item analysis", zap.Int64("item_id", itemID), zap.Error(err))
			continue
		}

		w.logger.Info("item analysis recovered", zap.Int64("item_id", itemID))
	}

	return errors.Join(recoveryErrors...)
}
