package outbox

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"

	"go.uber.org/zap"
)

type Publisher interface {
	Publish(ctx context.Context, event Event) error
}

type workerStore interface {
	Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Event, error)
	MarkPublished(ctx context.Context, eventID int64, workerID string) error
	Retry(ctx context.Context, eventID int64, workerID, message string, delay time.Duration) error
}

type WorkerConfig struct {
	PollInterval time.Duration
	RetryDelay   time.Duration
	Lease        time.Duration
	BatchSize    int
}

func DefaultWorkerConfig() WorkerConfig {
	return WorkerConfig{
		PollInterval: 500 * time.Millisecond,
		RetryDelay:   2 * time.Second,
		Lease:        30 * time.Second,
		BatchSize:    50,
	}
}

type Worker struct {
	store     workerStore
	publisher Publisher
	logger    *zap.Logger
	config    WorkerConfig
	workerID  string
}

func NewWorker(store *Store, publisher Publisher, logger *zap.Logger, config WorkerConfig) (*Worker, error) {
	if store == nil || publisher == nil || logger == nil {
		return nil, fmt.Errorf("outbox worker: store, publisher and logger are required")
	}
	if config.PollInterval <= 0 || config.RetryDelay <= 0 || config.Lease <= 0 || config.BatchSize <= 0 {
		return nil, fmt.Errorf("outbox worker: invalid configuration")
	}
	identifier := make([]byte, 16)
	if _, err := rand.Read(identifier); err != nil {
		return nil, fmt.Errorf("outbox worker identifier: %w", err)
	}
	return &Worker{store: store, publisher: publisher, logger: logger, config: config, workerID: hex.EncodeToString(identifier)}, nil
}

// Run dispatches committed events until the context is canceled.
func (w *Worker) Run(ctx context.Context) {
	ticker := time.NewTicker(w.config.PollInterval)
	defer ticker.Stop()

	for {
		if err := w.dispatch(ctx); err != nil && ctx.Err() == nil {
			w.logger.Error("dispatch outbox events", zap.Error(err))
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (w *Worker) dispatch(ctx context.Context) error {
	eventsToPublish, err := w.store.Claim(ctx, w.workerID, w.config.BatchSize, w.config.Lease)
	if err != nil {
		return err
	}
	for _, pending := range eventsToPublish {
		if err := w.publisher.Publish(ctx, pending); err != nil {
			if retryErr := w.store.Retry(ctx, pending.ID, w.workerID, err.Error(), w.config.RetryDelay); retryErr != nil {
				return fmt.Errorf("publish outbox event %d and schedule retry: %w", pending.ID, errors.Join(err, retryErr))
			}
			continue
		}
		if err := w.store.MarkPublished(ctx, pending.ID, w.workerID); err != nil {
			if retryErr := w.store.Retry(ctx, pending.ID, w.workerID, err.Error(), w.config.RetryDelay); retryErr != nil {
				return fmt.Errorf("ack outbox event %d and schedule retry: %w", pending.ID, errors.Join(err, retryErr))
			}
			return fmt.Errorf("ack outbox event %d: %w", pending.ID, err)
		}
	}
	return nil
}
