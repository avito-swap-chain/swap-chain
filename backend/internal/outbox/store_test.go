package outbox

import (
	"context"
	"strings"
	"testing"
	"time"

	"go.uber.org/zap"

	"swap-chain/internal/events"
)

func TestStoreAndBroadcasterValidateInputs(t *testing.T) {
	if _, err := NewStore(nil); err == nil {
		t.Fatal("NewStore(nil) error = nil")
	}
	store := &Store{}
	if err := store.EnqueueTx(context.Background(), nil, PendingEvent{}); err == nil {
		t.Fatal("EnqueueTx(nil) error = nil")
	}
	if _, err := NewPostgreSQLBroadcaster(nil, "", nil, nil); err == nil {
		t.Fatal("NewPostgreSQLBroadcaster() error = nil")
	}

	broadcaster := &PostgreSQLBroadcaster{hub: events.NewHub(), logger: zap.NewNop()}
	err := broadcaster.Publish(context.Background(), Event{
		ID:           1,
		Type:         "chain.updated",
		EntityID:     "1",
		RecipientIDs: []int64{1},
		Data:         map[string]any{"large": strings.Repeat("x", maxNotifyPayload)},
		OccurredAt:   time.Now(),
	})
	if err == nil || !strings.Contains(err.Error(), "payload is too large") {
		t.Fatalf("oversized Publish() error = %v", err)
	}
}
