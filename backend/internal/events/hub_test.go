package events

import (
	"context"
	"testing"
	"time"
)

func TestHubDeliversOnlyToTargetUser(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub()
	first := hub.Subscribe(ctx, 1)
	second := hub.Subscribe(ctx, 2)

	hub.PublishToUser(1, "chain.updated", "42", nil)

	select {
	case event := <-first:
		if event.EntityID != "42" {
			t.Fatalf("entity ID = %q, want 42", event.EntityID)
		}
	case <-time.After(time.Second):
		t.Fatal("target user did not receive event")
	}

	select {
	case event := <-second:
		t.Fatalf("another user received event: %#v", event)
	case <-time.After(25 * time.Millisecond):
	}
}

func TestHubDeliversOneEventToExplicitRecipients(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	hub := NewHub()
	first := hub.Subscribe(ctx, 1)
	second := hub.Subscribe(ctx, 2)

	published := hub.PublishToUsers([]int64{1, 2, 2}, "chain.updated", "42", nil)

	for user, channel := range map[int64]<-chan Event{1: first, 2: second} {
		select {
		case event := <-channel:
			if event.ID != published.ID {
				t.Fatalf("user %d event ID = %q, want %q", user, event.ID, published.ID)
			}
		case <-time.After(time.Second):
			t.Fatalf("user %d did not receive event", user)
		}
	}
}
