package items

import (
	"context"
	"database/sql"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"go.uber.org/zap"
)

func TestPostgresServiceLifecycleIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	userID := time.Now().UnixNano()
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	vector := make([]float32, 1024)
	vector[0] = 1
	vectorizer := &fakeVectorizer{vectors: [][]float32{vector, vector}}
	updated := make(chan struct{}, 1)
	service := NewPostgresService(database, vectorizer, func(_ int64, eventType, _ string, _ map[string]any) {
		if eventType == "item.status.updated" {
			updated <- struct{}{}
		}
	}, zap.NewNop())
	t.Cleanup(service.Close)

	created, err := service.Create(context.Background(), userID, CreateInput{
		OfferTitle:       "Городской велосипед",
		OfferDescription: "Исправен",
		WantDescription:  "Сноуборд",
		ImageURLs:        []string{"https://example.com/bike.jpg"},
	})
	if err != nil {
		t.Fatalf("Create() error = %v", err)
	}

	select {
	case <-updated:
	case <-time.After(3 * time.Second):
		t.Fatal("timed out waiting for MATCHING status")
	}

	item, err := service.Get(context.Background(), created.ID)
	if err != nil {
		t.Fatalf("Get() error = %v", err)
	}
	if item.Status != "MATCHING" {
		t.Fatalf("status = %q, want MATCHING", item.Status)
	}
	if len(item.ImageURLs) != 1 || item.ImageURLs[0] != "https://example.com/bike.jpg" {
		t.Fatalf("image URLs = %v", item.ImageURLs)
	}

	var embeddingsReady bool
	if err := database.QueryRowContext(context.Background(), `
		SELECT offer_embedding IS NOT NULL AND want_embedding IS NOT NULL
		FROM items WHERE id = $1`, created.ID).Scan(&embeddingsReady); err != nil {
		t.Fatalf("query embeddings: %v", err)
	}
	if !embeddingsReady {
		t.Fatal("embeddings were not persisted")
	}
}
