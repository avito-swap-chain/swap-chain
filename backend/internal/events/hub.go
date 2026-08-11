// Package events provides user-scoped in-process realtime event delivery.
package events

import (
	"context"
	"strconv"
	"sync"
	"sync/atomic"
	"time"
)

// Event is a transport notification about a committed state change.
type Event struct {
	ID         string         `json:"id"`
	Type       string         `json:"type"`
	EntityID   string         `json:"entityId,omitempty"`
	OccurredAt time.Time      `json:"occurredAt"`
	Data       map[string]any `json:"data,omitempty"`
}

// Hub routes events to subscribers of explicit user IDs.
type Hub struct {
	mu          sync.RWMutex
	subscribers map[int64]map[chan Event]struct{}
	nextID      atomic.Uint64
}

// NewHub creates an empty user-scoped event hub.
func NewHub() *Hub {
	return &Hub{subscribers: make(map[int64]map[chan Event]struct{})}
}

// PublishToUser publishes one event to all active streams of a user.
func (h *Hub) PublishToUser(userID int64, eventType, entityID string, data map[string]any) Event {
	return h.PublishToUsers([]int64{userID}, eventType, entityID, data)
}

// PublishToUsers publishes one event to an explicit deduplicated recipient set.
func (h *Hub) PublishToUsers(userIDs []int64, eventType, entityID string, data map[string]any) Event {
	event := Event{
		ID:         strconv.FormatUint(h.nextID.Add(1), 10),
		Type:       eventType,
		EntityID:   entityID,
		OccurredAt: time.Now().UTC(),
		Data:       data,
	}
	h.Publish(userIDs, event)
	return event
}

// Publish routes a preconstructed event, preserving a durable outbox ID and timestamp.
func (h *Hub) Publish(userIDs []int64, event Event) {
	h.mu.RLock()
	defer h.mu.RUnlock()
	delivered := make(map[int64]struct{}, len(userIDs))
	for _, userID := range userIDs {
		if _, duplicate := delivered[userID]; duplicate {
			continue
		}
		delivered[userID] = struct{}{}

		for subscriber := range h.subscribers[userID] {
			select {
			case subscriber <- event:
			default:
			}
		}
	}
}

// Subscribe registers a stream for one user until the context is canceled.
func (h *Hub) Subscribe(ctx context.Context, userID int64) <-chan Event {
	channel := make(chan Event, 16)
	h.mu.Lock()
	if h.subscribers[userID] == nil {
		h.subscribers[userID] = make(map[chan Event]struct{})
	}
	h.subscribers[userID][channel] = struct{}{}
	h.mu.Unlock()

	go func() {
		<-ctx.Done()
		h.mu.Lock()
		delete(h.subscribers[userID], channel)
		if len(h.subscribers[userID]) == 0 {
			delete(h.subscribers, userID)
		}
		close(channel)
		h.mu.Unlock()
	}()

	return channel
}
