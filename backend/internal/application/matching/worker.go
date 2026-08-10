package matching

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"go.uber.org/zap"
)

const maxMatchingJobErrorLength = 2000

// Job is a leased unit of automatic matching work.
type Job struct {
	ItemID   int64
	Attempts int
}

type JobRepository interface {
	Claim(ctx context.Context, staleBefore time.Time, batchSize int) ([]Job, error)
	Complete(ctx context.Context, itemID int64) (bool, error)
	Retry(ctx context.Context, itemID int64, availableAt time.Time, lastError string) (bool, error)
}

type JobMaterializer interface {
	Materialize(ctx context.Context, itemID int64) (int, error)
}

type WorkerConfig struct {
	PollInterval   time.Duration
	LeaseDuration  time.Duration
	AttemptTimeout time.Duration
	RetryBaseDelay time.Duration
	RetryMaxDelay  time.Duration
	BatchSize      int
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval:   5 * time.Second,
		LeaseDuration:  2 * time.Minute,
		AttemptTimeout: time.Minute,
		RetryBaseDelay: 5 * time.Second,
		RetryMaxDelay:  time.Minute,
		BatchSize:      20,
	}
}

// Worker leases durable jobs, materializes cycles and retries transient
// failures. A database lease makes a process crash recoverable on the next run.
type Worker struct {
	repo         JobRepository
	materializer JobMaterializer
	logger       *zap.Logger
	cfg          WorkerConfig
	now          func() time.Time
}

func NewWorker(
	repo JobRepository,
	materializer JobMaterializer,
	logger *zap.Logger,
	cfg WorkerConfig,
) (*Worker, error) {
	switch {
	case repo == nil:
		return nil, fmt.Errorf("matching worker: job repository is required")
	case materializer == nil:
		return nil, fmt.Errorf("matching worker: materializer is required")
	case logger == nil:
		return nil, fmt.Errorf("matching worker: logger is required")
	case cfg.PollInterval <= 0:
		return nil, fmt.Errorf("matching worker: poll interval must be positive")
	case cfg.LeaseDuration <= cfg.AttemptTimeout:
		return nil, fmt.Errorf("matching worker: lease duration must exceed attempt timeout")
	case cfg.AttemptTimeout <= 0:
		return nil, fmt.Errorf("matching worker: attempt timeout must be positive")
	case cfg.RetryBaseDelay <= 0:
		return nil, fmt.Errorf("matching worker: retry base delay must be positive")
	case cfg.RetryMaxDelay < cfg.RetryBaseDelay:
		return nil, fmt.Errorf("matching worker: retry max delay must not be less than base delay")
	case cfg.BatchSize <= 0:
		return nil, fmt.Errorf("matching worker: batch size must be positive")
	default:
		return &Worker{repo: repo, materializer: materializer, logger: logger, cfg: cfg, now: time.Now}, nil
	}
}

func (w *Worker) Start(ctx context.Context) {
	ticker := time.NewTicker(w.cfg.PollInterval)
	defer ticker.Stop()

	w.runAndLog(ctx)
	for {
		select {
		case <-ctx.Done():
			w.logger.Info("matching worker stopped")
			return
		case <-ticker.C:
			w.runAndLog(ctx)
		}
	}
}

func (w *Worker) runAndLog(ctx context.Context) {
	if err := w.runOnce(ctx); err != nil && !errors.Is(err, context.Canceled) {
		w.logger.Error("matching worker iteration failed", zap.Error(err))
	}
}

func (w *Worker) runOnce(ctx context.Context) error {
	jobs, err := w.repo.Claim(ctx, w.now().Add(-w.cfg.LeaseDuration), w.cfg.BatchSize)
	if err != nil {
		return fmt.Errorf("claim matching jobs: %w", err)
	}
	if len(jobs) == 0 {
		return nil
	}

	w.logger.Info("claimed matching jobs", zap.Int("count", len(jobs)))
	var iterationErrors []error
	for _, job := range jobs {
		if err := ctx.Err(); err != nil {
			iterationErrors = append(iterationErrors, err)
			break
		}

		attemptCtx, cancel := context.WithTimeout(ctx, w.cfg.AttemptTimeout)
		created, materializeErr := w.materializer.Materialize(attemptCtx, job.ItemID)
		cancel()
		if materializeErr != nil {
			retryAt := w.now().Add(w.retryDelay(job.Attempts))
			updated, retryErr := w.repo.Retry(
				ctx,
				job.ItemID,
				retryAt,
				truncateMatchingJobError(materializeErr.Error()),
			)
			if retryErr != nil {
				iterationErrors = append(iterationErrors, fmt.Errorf("retry matching job for item %d: %w", job.ItemID, retryErr))
			} else if !updated {
				w.logger.Warn("matching job state changed before retry", zap.Int64("item_id", job.ItemID))
			}
			w.logger.Error(
				"matching job failed",
				zap.Int64("item_id", job.ItemID),
				zap.Int("attempt", job.Attempts),
				zap.Time("retry_at", retryAt),
				zap.Error(materializeErr),
			)
			continue
		}

		updated, completeErr := w.repo.Complete(ctx, job.ItemID)
		if completeErr != nil {
			iterationErrors = append(iterationErrors, fmt.Errorf("complete matching job for item %d: %w", job.ItemID, completeErr))
			continue
		}
		if !updated {
			w.logger.Warn("matching job state changed before completion", zap.Int64("item_id", job.ItemID))
			continue
		}
		w.logger.Info("matching job completed", zap.Int64("item_id", job.ItemID), zap.Int("created_chains", created))
	}

	return errors.Join(iterationErrors...)
}

func (w *Worker) retryDelay(attempt int) time.Duration {
	delay := w.cfg.RetryBaseDelay
	for current := 1; current < attempt && delay < w.cfg.RetryMaxDelay; current++ {
		if delay > w.cfg.RetryMaxDelay/2 {
			return w.cfg.RetryMaxDelay
		}
		delay *= 2
	}
	if delay > w.cfg.RetryMaxDelay {
		return w.cfg.RetryMaxDelay
	}
	return delay
}

func truncateMatchingJobError(message string) string {
	message = strings.TrimSpace(message)
	runes := []rune(message)
	if len(runes) <= maxMatchingJobErrorLength {
		return message
	}
	return string(runes[:maxMatchingJobErrorLength])
}
