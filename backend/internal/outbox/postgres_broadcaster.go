package outbox

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"strconv"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"

	"swap-chain/internal/events"
)

const (
	realtimeChannel    = "swap_chain_realtime"
	maxNotifyPayload   = 7900
	listenerMinBackoff = time.Second
	listenerMaxBackoff = 30 * time.Second
)

type realtimeEnvelope struct {
	ID           int64          `json:"id"`
	Type         string         `json:"type"`
	EntityID     string         `json:"entityId"`
	RecipientIDs []int64        `json:"recipientIds"`
	Data         map[string]any `json:"data,omitempty"`
	OccurredAt   time.Time      `json:"occurredAt"`
}

// PostgreSQLBroadcaster fans one claimed outbox event out to every backend instance.
type PostgreSQLBroadcaster struct {
	database *sql.DB
	listener *pq.Listener
	hub      *events.Hub
	logger   *zap.Logger
}

func NewPostgreSQLBroadcaster(database *sql.DB, databaseURL string, hub *events.Hub, logger *zap.Logger) (*PostgreSQLBroadcaster, error) {
	if database == nil || databaseURL == "" || hub == nil || logger == nil {
		return nil, errors.New("postgres broadcaster: database, URL, hub and logger are required")
	}
	listener := pq.NewListener(databaseURL, listenerMinBackoff, listenerMaxBackoff, func(event pq.ListenerEventType, err error) {
		if err != nil {
			logger.Warn("postgres realtime listener state changed", zap.Int("event", int(event)), zap.Error(err))
		}
	})
	if err := listener.Listen(realtimeChannel); err != nil {
		_ = listener.Close()
		return nil, fmt.Errorf("listen for realtime events: %w", err)
	}
	return &PostgreSQLBroadcaster{database: database, listener: listener, hub: hub, logger: logger}, nil
}

// Publish notifies all listening backend instances. PostgreSQL delivers it after this statement commits.
func (b *PostgreSQLBroadcaster) Publish(ctx context.Context, event Event) error {
	payload, err := json.Marshal(realtimeEnvelope(event))
	if err != nil {
		return fmt.Errorf("marshal realtime event: %w", err)
	}
	if len(payload) > maxNotifyPayload {
		return fmt.Errorf("realtime event %d payload is too large: %d bytes", event.ID, len(payload))
	}
	if _, err := b.database.ExecContext(ctx, `SELECT pg_notify($1, $2)`, realtimeChannel, string(payload)); err != nil {
		return fmt.Errorf("notify realtime event: %w", err)
	}
	return nil
}

// Run forwards database notifications to subscribers connected to this backend instance.
func (b *PostgreSQLBroadcaster) Run(ctx context.Context) {
	for {
		select {
		case <-ctx.Done():
			return
		case notification, open := <-b.listener.Notify:
			if !open {
				return
			}
			if notification == nil {
				continue
			}
			var envelope realtimeEnvelope
			if err := json.Unmarshal([]byte(notification.Extra), &envelope); err != nil {
				b.logger.Error("decode postgres realtime event", zap.Error(err))
				continue
			}
			b.hub.Publish(envelope.RecipientIDs, events.Event{
				ID:         "outbox:" + strconv.FormatInt(envelope.ID, 10),
				Type:       envelope.Type,
				EntityID:   envelope.EntityID,
				OccurredAt: envelope.OccurredAt,
				Data:       envelope.Data,
			})
		}
	}
}

func (b *PostgreSQLBroadcaster) Close() error {
	if err := b.listener.UnlistenAll(); err != nil {
		_ = b.listener.Close()
		return fmt.Errorf("unlisten realtime events: %w", err)
	}
	if err := b.listener.Close(); err != nil {
		return fmt.Errorf("close realtime listener: %w", err)
	}
	return nil
}
