package adapters

import (
	"context"
	"errors"
	"testing"
)

func TestGigaChatGenerationSlotHonorsContext(t *testing.T) {
	client := &GigaChat{generationSlot: make(chan struct{}, 1)}
	release, err := client.acquireGenerationSlot(context.Background())
	if err != nil {
		t.Fatalf("acquireGenerationSlot() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.acquireGenerationSlot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireGenerationSlot() error = %v, want context canceled", err)
	}

	release()
	secondRelease, err := client.acquireGenerationSlot(context.Background())
	if err != nil {
		t.Fatalf("acquireGenerationSlot() after release error = %v", err)
	}
	secondRelease()
}
