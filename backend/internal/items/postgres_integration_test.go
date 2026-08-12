package items

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	_ "github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
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
	phone := fmt.Sprintf("+7%010d", userID%10_000_000_000)
	if _, err := database.ExecContext(context.Background(), `INSERT INTO users (id, username, phone) VALUES ($1, $2, $3)`, userID, fmt.Sprintf("items-test-%d", userID), phone); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM users WHERE id = $1`, userID)
	})

	vector := make([]float32, 1024)
	vector[0] = 1
	analyzer := fakeAnalyzer{analyze: func(ctx context.Context, itemID int64) error {
		_, err := database.ExecContext(ctx, `
			UPDATE items
			SET offer_embedding_local = $2, want_embedding_local = $2, status = 'MATCHING',
			    last_status_updated_at = now(), updated_at = now()
			WHERE id = $1 AND status = 'ANALYZING'`, itemID, pgvector.NewVector(vector))
		return err
	}}
	updated := make(chan struct{}, 1)
	service := NewPostgresService(database, analyzer, func(_ int64, eventType, _ string, _ map[string]any) {
		if eventType == "item.status.updated" {
			updated <- struct{}{}
		}
	}, zap.NewNop(), time.Minute)
	t.Cleanup(service.Close)

	created, err := service.Create(context.Background(), userID, CreateInput{
		OfferTitle:       "Городской велосипед",
		OfferDescription: "Исправен",
		Wishes: []string{"want"},
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
		SELECT offer_embedding_local IS NOT NULL AND want_embedding_local IS NOT NULL
		FROM items WHERE id = $1`, created.ID).Scan(&embeddingsReady); err != nil {
		t.Fatalf("query embeddings: %v", err)
	}
	if !embeddingsReady {
		t.Fatal("embeddings were not persisted")
	}
}
