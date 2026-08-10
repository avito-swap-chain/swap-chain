package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"

	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/chains"
	matchingrepository "swap-chain/modules/matching/repository"
	matchingservice "swap-chain/modules/matching/service"
	"swap-chain/shared/db"
)

const migrationBeforeSchemaAlignment = 9

func TestMigrationsCreateAnalyzeAndMatchingSchema(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	assertAnalyzeAndMatchingSchema(t, database)

	var imageAmount int
	if err := database.QueryRow(`
		WITH created_user AS (
			INSERT INTO users (username, phone)
			VALUES ('image-amount-integration', '+79990009999')
			RETURNING id
		)
		INSERT INTO items (user_id, offer_title, image_urls)
		SELECT id, 'Image amount check', ARRAY['first.jpg', 'second.jpg'] FROM created_user
		RETURNING image_amount`).Scan(&imageAmount); err != nil {
		t.Fatalf("insert item with images: %v", err)
	}
	if imageAmount != 2 {
		t.Fatalf("image_amount = %d, want 2", imageAmount)
	}
}

func TestSchemaAlignmentUpgradesAndRollsBackExistingSchema(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(migrationBeforeSchemaAlignment); err != nil {
		t.Fatalf("apply first nine migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	assertColumn(t, database, "items", "offer_embedding", true)
	assertColumn(t, database, "items", "offer_embedding_local", true)
	if _, err := database.Exec(`
		INSERT INTO categories (name, embedding, embedding_local)
		VALUES (
			'Legacy category',
			array_fill(0::real, ARRAY[1024])::vector,
			array_fill(0::real, ARRAY[1024])::vector
		)`); err != nil {
		t.Fatalf("insert pre-alignment category: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close pre-alignment database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Up(); err != nil {
		t.Fatalf("upgrade migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	assertAnalyzeAndMatchingSchema(t, database)
	var legacyEmbeddingPreserved bool
	if err := database.QueryRow(`
		SELECT embedding_local IS NOT NULL
		FROM categories
		WHERE name = 'Legacy category'`).Scan(&legacyEmbeddingPreserved); err != nil {
		t.Fatalf("load upgraded category: %v", err)
	}
	if !legacyEmbeddingPreserved {
		t.Fatal("legacy category embedding was not preserved")
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Steps(-1); err != nil {
		t.Fatalf("roll back schema alignment: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	assertColumn(t, database, "items", "offer_embedding", true)
	assertColumn(t, database, "items", "offer_embedding_local", true)
	assertColumn(t, database, "categories", "embedding", true)
	assertColumn(t, database, "categories", "embedding_local", true)
	assertColumn(t, database, "categories", "is_system", false)
}

func TestThreeItemsProduceExpectedExchangeChain(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	categoryIDs := loadCategoryIDs(t, database, 3)
	vectors := [][]float32{unitVector(0), unitVector(1), unitVector(2)}

	userIDs := make([]int64, 0, 3)
	for index, name := range []string{"Алиса", "Борис", "Вера"} {
		var userID int64
		if err := database.QueryRowContext(ctx, `
			INSERT INTO users (username, phone)
			VALUES ($1, $2)
			RETURNING id`, name+"-integration", fmt.Sprintf("+7999000%04d", index+1)).Scan(&userID); err != nil {
			t.Fatalf("create user %s: %v", name, err)
		}
		userIDs = append(userIDs, userID)
	}

	itemIDs := make([]int64, 0, 3)
	for index, title := range []string{"Книга по архитектуре", "Настольная лампа", "Механическая клавиатура"} {
		var itemID int64
		if err := database.QueryRowContext(ctx, `
			INSERT INTO items (
				user_id, offer_title, offer_description, want_description,
				offer_category_id, want_category_id,
				offer_embedding_local, want_embedding_local,
				status, param_richness, quality_score
			)
			VALUES ($1, $2, 'Описание', 'Пожелание', $3, $4, $5, $6, 'MATCHING', 1, 1)
			RETURNING id`,
			userIDs[index],
			title,
			categoryIDs[index],
			categoryIDs[(index+1)%3],
			pgvector.NewVector(vectors[index]),
			pgvector.NewVector(vectors[(index+1)%3]),
		).Scan(&itemID); err != nil {
			t.Fatalf("create item %q: %v", title, err)
		}
		itemIDs = append(itemIDs, itemID)
	}

	repository, err := matchingrepository.NewPostgreSQLMatching(db.New(database))
	if err != nil {
		t.Fatalf("create matching repository: %v", err)
	}
	matcher, err := matchingservice.NewMatching(zap.NewNop(), repository, matchingservice.NewScoring(), matchingservice.MatchingConfig{
		SimilarItemsAmount:              20,
		CompatibilityThreshold:          0.5,
		UndefinedCategoryID:             47,
		UndefinedCompatibilityThreshold: 0.6,
		ChainLen:                        3,
		PenaltyFactor:                   0.25,
		ChainRatingThreshold:            0.3,
	})
	if err != nil {
		t.Fatalf("create matcher: %v", err)
	}
	chainService := chains.NewPostgresService(database, nil)
	finder := applicationmatching.NewFindCycles(matcher, chainService)
	cycles, err := finder.Execute(ctx, itemIDs[0])
	if err != nil {
		t.Fatalf("find cycles: %v", err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycle count = %d, want 1: %#v", len(cycles), cycles)
	}

	wantTargets := map[int64]int64{itemIDs[0]: itemIDs[1], itemIDs[1]: itemIDs[2], itemIDs[2]: itemIDs[0]}
	edges := make([]chains.Edge, 0, len(cycles[0]))
	for _, edge := range cycles[0] {
		if wantTargets[edge.SourceID] != edge.TargetID {
			t.Fatalf("unexpected edge %d -> %d; want %d", edge.SourceID, edge.TargetID, wantTargets[edge.SourceID])
		}
		edges = append(edges, chains.Edge{SourceItemID: edge.SourceID, TargetItemID: edge.TargetID})
	}

	chain, err := chainService.Create(ctx, userIDs[0], chains.CreateInput{Edges: edges})
	if err != nil {
		t.Fatalf("create chain: %v", err)
	}
	if chain.Status != chains.StatusPending || len(chain.Participants) != 3 {
		t.Fatalf("created chain = %#v", chain)
	}
}

func newMigratedDatabase(t *testing.T, steps int) *sql.DB {
	t.Helper()
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)
	migrator := newMigrator(t, databaseURL)
	var err error
	if steps == 0 {
		err = migrator.Up()
	} else {
		err = migrator.Steps(steps)
	}
	if err != nil {
		t.Fatalf("apply migrations: %v", err)
	}
	closeMigrator(t, migrator)
	database := openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	return database
}

func newTestDatabase(t *testing.T) (string, func()) {
	t.Helper()
	adminURL := os.Getenv("TEST_DATABASE_URL")
	if adminURL == "" {
		t.Skip("TEST_DATABASE_URL is not set")
	}
	parsed, err := url.Parse(adminURL)
	if err != nil {
		t.Fatalf("parse TEST_DATABASE_URL: %v", err)
	}
	admin := openDatabase(t, adminURL)
	name := "swap_chain_integration_" + strconv.FormatInt(time.Now().UnixNano(), 10)
	if _, err := admin.Exec(`CREATE DATABASE ` + pq.QuoteIdentifier(name)); err != nil {
		_ = admin.Close()
		t.Fatalf("create test database: %v", err)
	}
	testURL := *parsed
	testURL.Path = "/" + name
	cleanup := func() {
		_, _ = admin.Exec(`SELECT pg_terminate_backend(pid) FROM pg_stat_activity WHERE datname = $1`, name)
		if _, dropErr := admin.Exec(`DROP DATABASE IF EXISTS ` + pq.QuoteIdentifier(name)); dropErr != nil {
			t.Errorf("drop test database %s: %v", name, dropErr)
		}
		if closeErr := admin.Close(); closeErr != nil {
			t.Errorf("close admin database: %v", closeErr)
		}
	}
	return testURL.String(), cleanup
}

func newMigrator(t *testing.T, databaseURL string) *migrate.Migrate {
	t.Helper()
	migrationsPath, err := filepath.Abs(filepath.Join("..", "..", "migrations"))
	if err != nil {
		t.Fatalf("resolve migrations path: %v", err)
	}
	migrator, err := migrate.New("file://"+filepath.ToSlash(migrationsPath), databaseURL)
	if err != nil {
		t.Fatalf("create migrator: %v", err)
	}
	return migrator
}

func closeMigrator(t *testing.T, migrator *migrate.Migrate) {
	t.Helper()
	sourceErr, databaseErr := migrator.Close()
	if sourceErr != nil || databaseErr != nil {
		t.Fatalf("close migrator: source=%v database=%v", sourceErr, databaseErr)
	}
}

func openDatabase(t *testing.T, databaseURL string) *sql.DB {
	t.Helper()
	database, err := sql.Open("postgres", databaseURL)
	if err != nil {
		t.Fatalf("open database: %v", err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := database.PingContext(ctx); err != nil {
		_ = database.Close()
		t.Fatalf("ping database: %v", err)
	}
	return database
}

func assertAnalyzeAndMatchingSchema(t *testing.T, database *sql.DB) {
	t.Helper()
	for _, column := range []struct{ table, name string }{
		{table: "items", name: "offer_embedding_local"},
		{table: "items", name: "want_embedding_local"},
		{table: "items", name: "image_amount"},
		{table: "categories", name: "embedding_local"},
		{table: "categories", name: "is_system"},
	} {
		assertColumn(t, database, column.table, column.name, true)
	}
	for _, column := range []struct{ table, name string }{
		{table: "items", name: "offer_embedding"},
		{table: "items", name: "want_embedding"},
		{table: "categories", name: "embedding"},
	} {
		assertColumn(t, database, column.table, column.name, false)
	}

	var systemCategories int
	if err := database.QueryRow(`SELECT count(*) FROM categories WHERE is_system = TRUE`).Scan(&systemCategories); err != nil {
		t.Fatalf("count system categories: %v", err)
	}
	if systemCategories != 11 {
		t.Fatalf("system category count = %d, want 11", systemCategories)
	}
}

func assertColumn(t *testing.T, database *sql.DB, table, column string, want bool) {
	t.Helper()
	var exists bool
	if err := database.QueryRow(`
		SELECT EXISTS (
			SELECT 1 FROM information_schema.columns
			WHERE table_schema = 'public' AND table_name = $1 AND column_name = $2
		)`, table, column).Scan(&exists); err != nil {
		t.Fatalf("check column %s.%s: %v", table, column, err)
	}
	if exists != want {
		t.Fatalf("column %s.%s exists = %t, want %t", table, column, exists, want)
	}
}

func loadCategoryIDs(t *testing.T, database *sql.DB, count int) []int32 {
	t.Helper()
	rows, err := database.Query(`SELECT id FROM categories WHERE id <> 47 ORDER BY id LIMIT $1`, count)
	if err != nil {
		t.Fatalf("load categories: %v", err)
	}
	defer func() { _ = rows.Close() }()
	ids := make([]int32, 0, count)
	for rows.Next() {
		var id int32
		if err := rows.Scan(&id); err != nil {
			t.Fatalf("scan category: %v", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate categories: %v", err)
	}
	if len(ids) != count {
		t.Fatalf("category IDs = %v, want %d", ids, count)
	}
	return ids
}

func unitVector(index int) []float32 {
	vector := make([]float32, 1024)
	vector[index] = 1
	return vector
}
