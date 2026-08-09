package items

import (
	"context"
	"sync"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestPostgresServiceTransitionsItemToMatching(t *testing.T) {
	repo := &fakeRepository{marked: make(chan Item, 1)}
	vectorizer := &fakeVectorizer{vectors: [][]float32{{1, 2}, {3, 4}}}
	events := make(chan string, 2)
	service := newPostgresService(repo, vectorizer, func(_ int64, eventType, _ string, _ map[string]any) {
		events <- eventType
	}, zap.NewNop())
	t.Cleanup(service.Close)

	created, err := service.Create(context.Background(), 7, CreateInput{
		OfferTitle:       "Велосипед",
		OfferDescription: "Городской",
		WantDescription:  "Сноуборд",
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}
	if created.Status != "ANALYZING" {
		t.Fatalf("created status = %q, want ANALYZING", created.Status)
	}

	if event := <-events; event != "item.created" {
		t.Fatalf("first event = %q, want item.created", event)
	}

	select {
	case updated := <-repo.marked:
		if updated.Status != "MATCHING" {
			t.Fatalf("updated status = %q, want MATCHING", updated.Status)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for analysis")
	}
	if event := <-events; event != "item.status.updated" {
		t.Fatalf("second event = %q, want item.status.updated", event)
	}
}

type fakeRepository struct {
	marked chan Item
}

func (r *fakeRepository) Create(_ context.Context, userID int64, input CreateInput) (Item, error) {
	now := time.Now().UTC()
	return Item{
		ID:               11,
		UserID:           userID,
		OfferTitle:       input.OfferTitle,
		OfferDescription: input.OfferDescription,
		WantDescription:  input.WantDescription,
		Status:           "ANALYZING",
		CreatedAt:        now,
		UpdatedAt:        now,
	}, nil
}

func (r *fakeRepository) Get(context.Context, int64) (Item, error) {
	return Item{}, ErrNotFound
}

func (r *fakeRepository) ListByUser(context.Context, int64, int64, int) ([]Item, *int64, error) {
	return nil, nil, nil
}

func (r *fakeRepository) MarkMatching(_ context.Context, itemID int64, _, _ []float32) (Item, error) {
	item := Item{ID: itemID, UserID: 7, Status: "MATCHING"}
	r.marked <- item
	return item, nil
}

type fakeVectorizer struct {
	mu      sync.Mutex
	vectors [][]float32
}

func (v *fakeVectorizer) Vectorize(context.Context, string) ([]float32, error) {
	v.mu.Lock()
	defer v.mu.Unlock()
	result := v.vectors[0]
	v.vectors = v.vectors[1:]
	return result, nil
}
