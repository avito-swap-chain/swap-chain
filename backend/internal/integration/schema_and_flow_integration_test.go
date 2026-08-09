package integration_test

import (
	"context"
	"database/sql"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
	"time"

	"github.com/golang-migrate/migrate/v4"
	_ "github.com/golang-migrate/migrate/v4/database/postgres"
	_ "github.com/golang-migrate/migrate/v4/source/file"
	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"

	analyzeservice "swap-chain/analyze/service"
	applicationmatching "swap-chain/internal/application/matching"
	"swap-chain/internal/chains"
	"swap-chain/internal/items"
	"swap-chain/internal/users"
	matchingrepository "swap-chain/matching/repository"
	matchingservice "swap-chain/matching/service"
	"swap-chain/shared/db"
)

const migrationFive = 5

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

func TestMigrationSixUpgradesAndRollsBackExistingSchema(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(migrationFive); err != nil {
		t.Fatalf("apply first five migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	assertColumn(t, database, "items", "offer_embedding", true)
	assertColumn(t, database, "items", "offer_embedding_local", false)
	if _, err := database.Exec(`
		INSERT INTO categories (name, embedding)
		VALUES ('Legacy category', array_fill(0::real, ARRAY[1024])::vector)`); err != nil {
		t.Fatalf("insert version-five category: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close version-five database: %v", err)
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

	var retainedCategoryID int32
	if err := database.QueryRow(`
		SELECT id
		FROM categories
		WHERE is_system = TRUE
		ORDER BY id
		LIMIT 1`).Scan(&retainedCategoryID); err != nil {
		t.Fatalf("load system category for rollback: %v", err)
	}
	if _, err := database.Exec(`
		UPDATE categories
		SET embedding_local = array_fill(0::real, ARRAY[1024])::vector
		WHERE id = $1`, retainedCategoryID); err != nil {
		t.Fatalf("prepare referenced category embedding: %v", err)
	}
	var retainedItemID int64
	if err := database.QueryRow(`
		WITH created_user AS (
			INSERT INTO users (username, phone)
			VALUES ('rollback-category-integration', '+79990008888')
			RETURNING id
		)
		INSERT INTO items (
			user_id, offer_title, offer_description, want_description,
			offer_category_id, want_category_id
		)
		SELECT id, 'Rollback category check', 'Описание', 'Пожелание', $1, $1
		FROM created_user
		RETURNING id`, retainedCategoryID).Scan(&retainedItemID); err != nil {
		t.Fatalf("insert item referencing system category: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Steps(-1); err != nil {
		t.Fatalf("roll back migration six: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	assertColumn(t, database, "items", "offer_embedding", true)
	assertColumn(t, database, "items", "offer_embedding_local", false)
	assertColumn(t, database, "categories", "embedding", true)
	assertColumn(t, database, "categories", "embedding_local", false)
	assertColumn(t, database, "items", "image_amount", false)

	var offerCategoryID, wantCategoryID int32
	if err := database.QueryRow(`
		SELECT offer_category_id, want_category_id
		FROM items
		WHERE id = $1`, retainedItemID).Scan(&offerCategoryID, &wantCategoryID); err != nil {
		t.Fatalf("load rolled-back item categories: %v", err)
	}
	if offerCategoryID != retainedCategoryID || wantCategoryID != retainedCategoryID {
		t.Fatalf(
			"rolled-back item categories = (%d, %d), want (%d, %d)",
			offerCategoryID,
			wantCategoryID,
			retainedCategoryID,
			retainedCategoryID,
		)
	}
	var retainedCategoryName string
	if err := database.QueryRow(`SELECT name FROM categories WHERE id = $1`, retainedCategoryID).Scan(&retainedCategoryName); err != nil {
		t.Fatalf("load retained category after rollback: %v", err)
	}
	if retainedCategoryName == "" {
		t.Fatal("retained category name is empty after rollback")
	}
}

func TestThreeItemsProduceExpectedExchangeChain(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()

	categoryIDs := loadCategoryIDs(t, database, 3)
	vectors := [][]float32{unitVector(0), unitVector(1), unitVector(2)}
	specs := map[string]analysisSpec{
		"Книга по архитектуре": {
			offerCategoryID: categoryIDs[0], wantCategoryID: categoryIDs[1],
			offerVector: vectors[0], wantVector: vectors[1],
		},
		"Настольная лампа": {
			offerCategoryID: categoryIDs[1], wantCategoryID: categoryIDs[2],
			offerVector: vectors[1], wantVector: vectors[2],
		},
		"Механическая клавиатура": {
			offerCategoryID: categoryIDs[2], wantCategoryID: categoryIDs[0],
			offerVector: vectors[2], wantVector: vectors[0],
		},
	}

	userService := users.NewPostgresService(database)
	userIDs := make([]int64, 0, 3)
	for index, name := range []string{"Алиса", "Борис", "Вера"} {
		created, err := userService.Create(ctx, users.CreateInput{
			Username: name + "-integration",
			Phone:    fmt.Sprintf("+7999000%04d", index+1),
		})
		if err != nil {
			t.Fatalf("create user %s: %v", name, err)
		}
		userIDs = append(userIDs, created.ID)
	}

	itemService := items.NewPostgresService(database, databaseAnalyzer{database: database, specs: specs}, nil, zap.NewNop())
	t.Cleanup(itemService.Close)
	itemInputs := []items.CreateInput{
		{OfferTitle: "Книга по архитектуре", OfferDescription: strings.Repeat("Описание книги. ", 8), WantDescription: "Настольная лампа"},
		{OfferTitle: "Настольная лампа", OfferDescription: strings.Repeat("Описание лампы. ", 8), WantDescription: "Механическая клавиатура"},
		{OfferTitle: "Механическая клавиатура", OfferDescription: strings.Repeat("Описание клавиатуры. ", 8), WantDescription: "Книга по архитектуре"},
	}
	itemIDs := make([]int64, 0, len(itemInputs))
	for index, input := range itemInputs {
		created, err := itemService.Create(ctx, userIDs[index], input)
		if err != nil {
			t.Fatalf("create item %q: %v", input.OfferTitle, err)
		}
		itemIDs = append(itemIDs, created.ID)
	}
	waitForMatchingItems(t, database, itemIDs)

	queries := db.New(database)
	repository, err := matchingrepository.NewPostgreSQLMatching(queries)
	if err != nil {
		t.Fatalf("create matching repository: %v", err)
	}
	matcher, err := matchingservice.NewMatching(zap.NewNop(), repository, matchingservice.NewScoring(), matchingservice.MatchingConfig{
		SimilarItemsAmount:     20,
		CompatibilityThreshold: 0.5,
		ChainLen:               3,
		PenaltyFactor:          0.25,
		ChainRatingThreshold:   0.3,
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

	wantTargets := map[int64]int64{
		itemIDs[0]: itemIDs[1],
		itemIDs[1]: itemIDs[2],
		itemIDs[2]: itemIDs[0],
	}
	edges := make([]chains.Edge, 0, len(cycles[0]))
	for _, edge := range cycles[0] {
		sourceID := int64(edge.SourceID)
		targetID := int64(edge.TargetID)
		if wantTargets[sourceID] != targetID {
			t.Fatalf("unexpected edge %d -> %d; want %d", sourceID, targetID, wantTargets[sourceID])
		}
		edges = append(edges, chains.Edge{SourceItemID: sourceID, TargetItemID: targetID})
	}

	chain, err := chainService.Create(ctx, userIDs[0], chains.CreateInput{Edges: edges})
	if err != nil {
		t.Fatalf("create chain: %v", err)
	}
	if chain.Status != chains.StatusPending || len(chain.Participants) != 3 {
		t.Fatalf("created chain = %#v", chain)
	}
	for _, participant := range chain.Participants {
		if wantTargets[participant.GiveItem.ID] != participant.ReceiveItem.ID {
			t.Fatalf("participant %d gives %d and receives %d", participant.User.ID, participant.GiveItem.ID, participant.ReceiveItem.ID)
		}
	}
}

type analysisSpec struct {
	offerCategoryID int32
	wantCategoryID  int32
	offerVector     []float32
	wantVector      []float32
}

type databaseAnalyzer struct {
	database *sql.DB
	specs    map[string]analysisSpec
}

func (analyzer databaseAnalyzer) AnalyzeItem(ctx context.Context, itemID int64) error {
	var title string
	if err := analyzer.database.QueryRowContext(ctx, `SELECT offer_title FROM items WHERE id = $1`, itemID).Scan(&title); err != nil {
		return fmt.Errorf("load item title: %w", err)
	}
	spec, ok := analyzer.specs[title]
	if !ok {
		return fmt.Errorf("analysis spec for %q is missing", title)
	}
	result, err := analyzer.database.ExecContext(ctx, `
		UPDATE items
		SET offer_category_id = $2,
		    want_category_id = $3,
		    param_richness = 1,
		    quality_score = 1,
		    offer_embedding_local = $4,
		    want_embedding_local = $5,
		    status = 'MATCHING',
		    updated_at = now(),
		    last_status_updated_at = now()
		WHERE id = $1 AND status = 'ANALYZING'`,
		itemID,
		spec.offerCategoryID,
		spec.wantCategoryID,
		pgvector.NewVector(spec.offerVector),
		pgvector.NewVector(spec.wantVector),
	)
	if err != nil {
		return fmt.Errorf("complete deterministic analysis: %w", err)
	}
	updated, err := result.RowsAffected()
	if err != nil {
		return fmt.Errorf("read deterministic analysis result: %w", err)
	}
	if updated != 1 {
		return analyzeservice.ErrAnalysisStateChanged
	}
	return nil
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
	for _, column := range []struct {
		table string
		name  string
	}{
		{table: "items", name: "offer_embedding_local"},
		{table: "items", name: "want_embedding_local"},
		{table: "items", name: "image_amount"},
		{table: "categories", name: "embedding_local"},
	} {
		assertColumn(t, database, column.table, column.name, true)
	}
	for _, removed := range []struct {
		table string
		name  string
	}{
		{table: "items", name: "offer_embedding"},
		{table: "items", name: "want_embedding"},
		{table: "categories", name: "embedding"},
	} {
		assertColumn(t, database, removed.table, removed.name, false)
	}

	var categoryNames string
	if err := database.QueryRow(`
		SELECT string_agg(name, '|' ORDER BY id)
		FROM categories
		WHERE is_system = TRUE`).Scan(&categoryNames); err != nil {
		t.Fatalf("load seeded categories: %v", err)
	}
	wantCategoryNames := strings.Join([]string{
		"Электроника",
		"Бытовая техника",
		"Дом и дача",
		"Спорт и отдых",
		"Книги",
		"Хобби и творчество",
		"Одежда и аксессуары",
		"Детские товары",
		"Транспорт",
		"Другое",
	}, "|")
	if categoryNames != wantCategoryNames {
		t.Fatalf("seeded categories = %q, want %q", categoryNames, wantCategoryNames)
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
	rows, err := database.Query(`SELECT id FROM categories ORDER BY id LIMIT $1`, count)
	if err != nil {
		t.Fatalf("load categories: %v", err)
	}
	defer func() {
		if closeErr := rows.Close(); closeErr != nil {
			t.Errorf("close categories: %v", closeErr)
		}
	}()
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

func waitForMatchingItems(t *testing.T, database *sql.DB, itemIDs []int64) {
	t.Helper()
	deadline := time.Now().Add(5 * time.Second)
	for time.Now().Before(deadline) {
		var count int
		err := database.QueryRow(`SELECT count(*) FROM items WHERE id = ANY($1) AND status = 'MATCHING'`, pq.Array(itemIDs)).Scan(&count)
		if err != nil {
			t.Fatalf("count analyzed items: %v", err)
		}
		if count == len(itemIDs) {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("timed out waiting for analyzed items")
}

func unitVector(index int) []float32 {
	vector := make([]float32, 1024)
	vector[index] = 1
	return vector
}
