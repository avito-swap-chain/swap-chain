package events

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"time"
)

// NewStream encodes events for one user as a server-sent event stream.
func NewStream(ctx context.Context, hub *Hub, userID int64) io.Reader {
	reader, writer := io.Pipe()
	go writeStream(ctx, hub, userID, writer)
	return reader
}

func writeStream(ctx context.Context, hub *Hub, userID int64, writer *io.PipeWriter) {
	defer func() {
		_ = writer.Close()
	}()

	write := func(value string) error {
		_, err := io.WriteString(writer, value)
		return err
	}

	if err := write("retry: 3000\n\n"); err != nil {
		return
	}

	connected := Event{
		ID:         "connected",
		Type:       "stream.connected",
		OccurredAt: time.Now().UTC(),
	}
	if err := writeEvent(write, connected); err != nil {
		return
	}

	events := hub.Subscribe(ctx, userID)
	heartbeat := time.NewTicker(15 * time.Second)
	defer heartbeat.Stop()

	for {
		select {
		case <-ctx.Done():
			return
		case event, open := <-events:
			if !open {
				return
			}
			if err := writeEvent(write, event); err != nil {
				return
			}
		case <-heartbeat.C:
			if err := write(": keep-alive\n\n"); err != nil {
				return
			}
		}
	}
}

func writeEvent(write func(string) error, event Event) error {
	payload, err := json.Marshal(event)
	if err != nil {
		return fmt.Errorf("marshal SSE event: %w", err)
	}
	return write(fmt.Sprintf("id: %s\nevent: %s\ndata: %s\n\n", event.ID, event.Type, payload))
}
