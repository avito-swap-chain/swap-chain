// Package outbox durably bridges committed database changes to realtime delivery.
package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/lib/pq"
)

// PendingEvent describes an event written in the same transaction as its state change.
type PendingEvent struct {
	DeduplicationKey string
	Type             string
	EntityID         string
	RecipientIDs     []int64
	Data             map[string]any
}

// Event is a claimed durable event ready for publishing.
type Event struct {
	ID           int64
	Type         string
	EntityID     string
	RecipientIDs []int64
	Data         map[string]any
	OccurredAt   time.Time
}

// Store persists, claims and acknowledges outbox events.
type Store struct {
	database *sql.DB
}

func NewStore(database *sql.DB) (*Store, error) {
	if database == nil {
		return nil, errors.New("outbox store: database is required")
	}
	return &Store{database: database}, nil
}

// EnqueueTx inserts one event using the caller's transaction. Duplicate keys are idempotent.
func (*Store) EnqueueTx(ctx context.Context, tx *sql.Tx, event PendingEvent) error {
	if tx == nil {
		return errors.New("enqueue outbox event: transaction is required")
	}
	if event.DeduplicationKey == "" || event.Type == "" || event.EntityID == "" || len(event.RecipientIDs) == 0 {
		return errors.New("enqueue outbox event: key, type, entity and recipients are required")
	}
	payload, err := json.Marshal(event.Data)
	if err != nil {
		return fmt.Errorf("marshal outbox payload: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO outbox_events (deduplication_key, event_type, entity_id, recipient_ids, payload)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (deduplication_key) DO NOTHING`,
		event.DeduplicationKey, event.Type, event.EntityID, pq.Array(event.RecipientIDs), string(payload),
	); err != nil {
		return fmt.Errorf("insert outbox event: %w", err)
	}
	return nil
}

// Claim leases a batch so multiple backend instances can safely compete for work.
func (s *Store) Claim(ctx context.Context, workerID string, limit int, lease time.Duration) ([]Event, error) {
	rows, err := s.database.QueryContext(ctx, `
		WITH candidates AS (
			SELECT id
			FROM outbox_events
			WHERE published_at IS NULL
			  AND available_at <= now()
			  AND (locked_until IS NULL OR locked_until < now())
			ORDER BY id
			FOR UPDATE SKIP LOCKED
			LIMIT $1
		)
		UPDATE outbox_events event
		SET locked_until = now() + ($2::bigint * interval '1 millisecond'),
		    locked_by = $3,
		    attempts = attempts + 1
		FROM candidates
		WHERE event.id = candidates.id
		RETURNING event.id, event.event_type, event.entity_id, event.recipient_ids,
		          event.payload, event.occurred_at`, limit, lease.Milliseconds(), workerID)
	if err != nil {
		return nil, fmt.Errorf("claim outbox events: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]Event, 0, limit)
	for rows.Next() {
		var event Event
		var payload []byte
		if err := rows.Scan(&event.ID, &event.Type, &event.EntityID, pq.Array(&event.RecipientIDs), &payload, &event.OccurredAt); err != nil {
			return nil, fmt.Errorf("scan outbox event: %w", err)
		}
		if len(payload) > 0 && string(payload) != "null" {
			if err := json.Unmarshal(payload, &event.Data); err != nil {
				return nil, fmt.Errorf("decode outbox event %d: %w", event.ID, err)
			}
		}
		result = append(result, event)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate outbox events: %w", err)
	}
	return result, nil
}

func (s *Store) MarkPublished(ctx context.Context, eventID int64, workerID string) error {
	result, err := s.database.ExecContext(ctx, `
		UPDATE outbox_events
		SET published_at = now(), locked_until = NULL, locked_by = NULL, last_error = NULL
		WHERE id = $1 AND locked_by = $2 AND published_at IS NULL`, eventID, workerID)
	if err != nil {
		return fmt.Errorf("mark outbox event published: %w", err)
	}
	return requireOneRow(result, "mark outbox event published")
}

func (s *Store) Retry(ctx context.Context, eventID int64, workerID, message string, delay time.Duration) error {
	result, err := s.database.ExecContext(ctx, `
		UPDATE outbox_events
		SET available_at = now() + ($3::bigint * interval '1 millisecond'),
		    locked_until = NULL,
		    locked_by = NULL,
		    last_error = left($4, 2000)
		WHERE id = $1 AND locked_by = $2 AND published_at IS NULL`, eventID, workerID, delay.Milliseconds(), message)
	if err != nil {
		return fmt.Errorf("retry outbox event: %w", err)
	}
	return requireOneRow(result, "retry outbox event")
}

func requireOneRow(result sql.Result, operation string) error {
	affected, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("%s affected rows: %w", operation, err)
	}
	if affected != 1 {
		return fmt.Errorf("%s: lease is no longer owned", operation)
	}
	return nil
}
