package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"
	"time"

	"swap-chain/shared/db"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
)

func TestMatchingRepositoryFindsCrossOwnerCandidateIntegration(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}

	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	stamp := time.Now().UnixNano()
	ownerID := insertMatchingTestUser(t, database, fmt.Sprintf("+7%010d", stamp%10_000_000_000), stamp)
	candidateOwnerID := insertMatchingTestUser(t, database, fmt.Sprintf("+7%010d", (stamp+1)%10_000_000_000), stamp+1)
	t.Cleanup(func() {
		_, _ = database.ExecContext(context.Background(), `DELETE FROM users WHERE id = ANY($1)`, pq.Array([]int64{ownerID, candidateOwnerID}))
	})

	vector := make([]float32, 1024)
	vector[0] = 1
	var categoryID int32
	if err := database.QueryRowContext(context.Background(), `SELECT id FROM categories ORDER BY id LIMIT 1`).Scan(&categoryID); err != nil {
		t.Fatalf("load matching category: %v", err)
	}
	sourceID := insertMatchingTestItem(t, database, ownerID, "Источник", categoryID, vector)
	_ = insertMatchingTestItem(t, database, ownerID, "Тот же владелец", categoryID, vector)
	candidateID := insertMatchingTestItem(t, database, candidateOwnerID, "Кандидат", categoryID, vector)

	repository, err := NewPostgreSQLMatching(db.New(database))
	if err != nil {
		t.Fatalf("NewPostgreSQLMatching() error = %v", err)
	}
	if err := repository.ValidateSourceItem(context.Background(), sourceID); err != nil {
		t.Fatalf("ValidateSourceItem() error = %v", err)
	}
	matches, err := repository.FindSimilarItems(context.Background(), sourceID, 1, 10)
	if err != nil {
		t.Fatalf("FindSimilarItems() error = %v", err)
	}
	if len(matches) != 1 || matches[0].TargetItem.ID != candidateID {
		t.Fatalf("FindSimilarItems() = %+v, want only candidate %d", matches, candidateID)
	}
}

func insertMatchingTestUser(t *testing.T, database *sql.DB, phone string, suffix int64) int64 {
	t.Helper()
	var id int64
	if err := database.QueryRowContext(context.Background(), `
		INSERT INTO users (username, phone) VALUES ($1, $2) RETURNING id`,
		fmt.Sprintf("matching-test-%d", suffix), phone,
	).Scan(&id); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	return id
}

func insertMatchingTestItem(t *testing.T, database *sql.DB, userID int64, title string, categoryID int32, vector []float32) int64 {
	t.Helper()
	var id int64
	if err := database.QueryRowContext(context.Background(), `
		INSERT INTO items (
			user_id, offer_title, offer_description, want_description,
			offer_category_id, want_category_id,
			offer_embedding_local, want_embedding_local, status, param_richness
		) VALUES ($1, $2, 'Описание', 'Желаемая вещь', $3, $3, $4, $4, 'MATCHING', 0.8)
		RETURNING id`, userID, title, categoryID, pgvector.NewVector(vector),
	).Scan(&id); err != nil {
		t.Fatalf("insert item: %v", err)
	}
	return id
}
