package items

import (
	"context"
	"testing"
	"time"

	"go.uber.org/zap"
)

func TestPostgresServiceTransitionsItemToMatching(t *testing.T) {
	repo := &fakeRepository{analyzed: make(chan int64, 1)}
	analyzer := fakeAnalyzer{analyze: func(_ context.Context, itemID int64) error {
		repo.analyzed <- itemID
		return nil
	}}
	events := make(chan string, 2)
	service := newPostgresService(repo, analyzer, func(_ int64, eventType, _ string, _ map[string]any) {
		events <- eventType
	}, zap.NewNop(), time.Minute)
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
	case itemID := <-repo.analyzed:
		if itemID != created.ID {
			t.Fatalf("analyzed item = %d, want %d", itemID, created.ID)
		}
	case <-time.After(time.Second):
		t.Fatal("timed out waiting for analysis")
	}
	if event := <-events; event != "item.status.updated" {
		t.Fatalf("second event = %q, want item.status.updated", event)
	}
}

type fakeRepository struct {
	analyzed chan int64
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

func (r *fakeRepository) Get(_ context.Context, itemID int64) (Item, error) {
	return Item{ID: itemID, UserID: 7, Status: "MATCHING"}, nil
}

func (r *fakeRepository) ListByUser(context.Context, int64, int64, int) ([]Item, *int64, error) {
	return nil, nil, nil
}

type fakeAnalyzer struct {
	analyze func(context.Context, int64) error
}

func (a fakeAnalyzer) AnalyzeItem(ctx context.Context, itemID int64) error {
	return a.analyze(ctx, itemID)
}
