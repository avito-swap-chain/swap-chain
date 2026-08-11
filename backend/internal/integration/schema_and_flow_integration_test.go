package integration_test

import (
	"context"
	"database/sql"
	"errors"
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
	"swap-chain/internal/events"
	"swap-chain/internal/outbox"
	adminmodel "swap-chain/modules/admin/model"
	adminrepository "swap-chain/modules/admin/repository"
	analyzemodel "swap-chain/modules/analyze/model"
	analyzerepository "swap-chain/modules/analyze/repository"
	matchingrepository "swap-chain/modules/matching/repository"
	matchingservice "swap-chain/modules/matching/service"
	metricsmodel "swap-chain/modules/metrics/model"
	metricsrepository "swap-chain/modules/metrics/repository"
	metricsservice "swap-chain/modules/metrics/service"
	reputationmodel "swap-chain/modules/reputation/model"
	reputationrepository "swap-chain/modules/reputation/repository"
	reputationservice "swap-chain/modules/reputation/service"
	"swap-chain/shared/db"
)

const migrationBeforeSchemaAlignment = 9

const migrationBeforeReputation = 14

const migrationBeforeChainRejections = 16

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
	if err := migrator.Migrate(migrationBeforeSchemaAlignment); err != nil {
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

func TestMatchingJobsMigrationBackfillsExistingMatchingItems(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(10); err != nil {
		t.Fatalf("apply first ten migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	var itemID int64
	if err := database.QueryRow(`
		WITH created_user AS (
			INSERT INTO users (username, phone)
			VALUES ('matching-job-backfill', '+79990008888')
			RETURNING id
		)
		INSERT INTO items (user_id, offer_title, status)
		SELECT id, 'Backfill matching item', 'MATCHING' FROM created_user
		RETURNING id`).Scan(&itemID); err != nil {
		t.Fatalf("create matching item before job migration: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close pre-job database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Up(); err != nil {
		t.Fatalf("apply matching jobs migration: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	var status string
	if err := database.QueryRow(`SELECT status FROM matching_jobs WHERE item_id = $1`, itemID).Scan(&status); err != nil {
		t.Fatalf("load backfilled matching job: %v", err)
	}
	if status != "PENDING" {
		t.Fatalf("matching job status = %q, want PENDING", status)
	}
}

func TestReputationAndOutboxRollbackPreservesExistingUsers(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(migrationBeforeReputation); err != nil {
		t.Fatalf("apply migrations through version 14: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	var userID int64
	if err := database.QueryRow(`
		INSERT INTO users (username, phone)
		VALUES ('reputation-upgrade-user', '+79990006666')
		RETURNING id`).Scan(&userID); err != nil {
		t.Fatalf("create pre-reputation user: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close version 14 database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Steps(2); err != nil {
		t.Fatalf("apply reputation and outbox migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	var aggregateCount int
	if err := database.QueryRow(`SELECT count(*) FROM user_reputation WHERE user_id = $1`, userID).Scan(&aggregateCount); err != nil {
		t.Fatalf("load backfilled reputation: %v", err)
	}
	if aggregateCount != 1 {
		t.Fatalf("backfilled reputation rows = %d, want 1", aggregateCount)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close upgraded database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Migrate(migrationBeforeReputation); err != nil {
		t.Fatalf("roll back reputation and outbox migrations: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	var username string
	if err := database.QueryRow(`SELECT username FROM users WHERE id = $1`, userID).Scan(&username); err != nil {
		t.Fatalf("load user after rollback: %v", err)
	}
	if username != "reputation-upgrade-user" {
		t.Fatalf("username after rollback = %q", username)
	}
	for _, table := range []string{"user_reviews", "user_reputation", "outbox_events"} {
		var exists bool
		if err := database.QueryRow(`SELECT to_regclass($1) IS NOT NULL`, table).Scan(&exists); err != nil {
			t.Fatalf("check table %s after rollback: %v", table, err)
		}
		if exists {
			t.Fatalf("table %s still exists after rollback", table)
		}
	}
}

func TestChainRejectionsUpgradeBackfillsAndRollsBack(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)

	migrator := newMigrator(t, databaseURL)
	if err := migrator.Steps(migrationBeforeChainRejections); err != nil {
		t.Fatalf("apply migrations through version 16: %v", err)
	}
	closeMigrator(t, migrator)

	database := openDatabase(t, databaseURL)
	var chainID int64
	if err := database.QueryRow(`
		INSERT INTO chains (status, cycle_key)
		VALUES ('REJECTED', 'rejection-backfill')
		RETURNING id`).Scan(&chainID); err != nil {
		t.Fatalf("create rejected chain before migration: %v", err)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close version 16 database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Steps(1); err != nil {
		t.Fatalf("apply chain rejection migration: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	var reason string
	if err := database.QueryRow(`SELECT reason FROM chain_rejections WHERE chain_id = $1`, chainID).Scan(&reason); err != nil {
		t.Fatalf("load backfilled rejection: %v", err)
	}
	if reason != "unknown" {
		t.Fatalf("backfilled rejection reason = %q, want unknown", reason)
	}
	if err := database.Close(); err != nil {
		t.Fatalf("close version 17 database: %v", err)
	}

	migrator = newMigrator(t, databaseURL)
	if err := migrator.Migrate(migrationBeforeChainRejections); err != nil {
		t.Fatalf("roll back chain rejection migration: %v", err)
	}
	closeMigrator(t, migrator)

	database = openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = database.Close() })
	var chainStatus string
	if err := database.QueryRow(`SELECT status::text FROM chains WHERE id = $1`, chainID).Scan(&chainStatus); err != nil {
		t.Fatalf("load chain after rollback: %v", err)
	}
	if chainStatus != "REJECTED" {
		t.Fatalf("chain status after rollback = %q, want REJECTED", chainStatus)
	}
	var rejectionTableExists bool
	if err := database.QueryRow(`SELECT to_regclass('chain_rejections') IS NOT NULL`).Scan(&rejectionTableExists); err != nil {
		t.Fatalf("check rejection table after rollback: %v", err)
	}
	if rejectionTableExists {
		t.Fatal("chain_rejections still exists after rollback")
	}
}

func TestCompletingAnalysisAtomicallyEnqueuesMatchingJob(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	categoryID := loadCategoryIDs(t, database, 1)[0]

	var itemID int64
	if err := database.QueryRowContext(ctx, `
		WITH created_user AS (
			INSERT INTO users (username, phone)
			VALUES ('analysis-job-integration', '+79990007777')
			RETURNING id
		)
		INSERT INTO items (user_id, offer_title, offer_description, want_description)
		SELECT id, 'Телефон', 'Описание телефона', 'Хочу велосипед' FROM created_user
		RETURNING id`).Scan(&itemID); err != nil {
		t.Fatalf("create analyzing item: %v", err)
	}

	repository, err := analyzerepository.NewPostgreSQLAnalysis(db.New(database))
	if err != nil {
		t.Fatalf("create analysis repository: %v", err)
	}
	updated, err := repository.CompleteItemAnalysis(ctx, analyzemodel.AnalysisResult{
		ItemID:          itemID,
		AnalysisVersion: 1,
		OfferCategoryID: categoryID,
		WantCategoryID:  categoryID,
		ParamRichness:   0.7,
		OfferEmbedding:  unitVector(0),
		WantEmbedding:   unitVector(1),
	})
	if err != nil {
		t.Fatalf("complete item analysis: %v", err)
	}
	if !updated {
		t.Fatal("complete item analysis did not update the item")
	}

	var itemStatus, jobStatus string
	if err := database.QueryRowContext(ctx, `
		SELECT item.status::text, job.status
		FROM items AS item
		JOIN matching_jobs AS job ON job.item_id = item.id
		WHERE item.id = $1`, itemID).Scan(&itemStatus, &jobStatus); err != nil {
		t.Fatalf("load analyzed item and matching job: %v", err)
	}
	if itemStatus != "MATCHING" || jobStatus != "PENDING" {
		t.Fatalf("item status = %q, job status = %q; want MATCHING/PENDING", itemStatus, jobStatus)
	}
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
	eventStore, err := outbox.NewStore(database)
	if err != nil {
		t.Fatalf("create chain outbox store: %v", err)
	}
	chainService := chains.NewPostgresServiceWithOutbox(database, nil, eventStore)
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
	outboxEvents, err := eventStore.Claim(ctx, "chain-test-worker", 10, time.Minute)
	if err != nil {
		t.Fatalf("claim chain outbox event: %v", err)
	}
	if len(outboxEvents) != 1 || outboxEvents[0].Type != "chain.created" || outboxEvents[0].EntityID != strconv.FormatInt(chain.ID, 10) {
		t.Fatalf("chain outbox events = %#v", outboxEvents)
	}
}

func TestCompletedChainReviewsUpdateReputation(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	chainID, userIDs, _ := seedDeliveryChain(t, database, adminmodel.ChainCompleted)

	repository, err := reputationrepository.NewPostgreSQL(database)
	if err != nil {
		t.Fatalf("create reputation repository: %v", err)
	}
	service, err := reputationservice.New(repository)
	if err != nil {
		t.Fatalf("create reputation service: %v", err)
	}

	results := make(chan error, 2)
	for _, input := range []struct {
		authorID int64
		rating   int
	}{
		{authorID: userIDs[0], rating: 5},
		{authorID: userIDs[2], rating: 3},
	} {
		go func() {
			_, createErr := service.CreateReview(ctx, input.authorID, chainID, reputationmodel.CreateInput{
				TargetUserID: userIDs[1],
				Rating:       input.rating,
			})
			results <- createErr
		}()
	}
	for range 2 {
		if err := <-results; err != nil {
			t.Fatalf("create concurrent review: %v", err)
		}
	}
	if _, err := service.CreateReview(ctx, userIDs[0], chainID, reputationmodel.CreateInput{TargetUserID: userIDs[1], Rating: 4}); !errors.Is(err, reputationmodel.ErrAlreadyExists) {
		t.Fatalf("duplicate review error = %v, want ErrAlreadyExists", err)
	}

	stats, err := service.Stats(ctx, userIDs[1])
	if err != nil {
		t.Fatalf("load reputation stats: %v", err)
	}
	if stats.Rating == nil || *stats.Rating != 4 || stats.ReviewsCount != 2 || stats.CompletedExchanges != 1 {
		t.Fatalf("reputation stats = %#v, want rating=4 reviews=2 completed=1", stats)
	}
	reviews, err := service.ListReviews(ctx, userIDs[1], 0, 20)
	if err != nil {
		t.Fatalf("list reviews: %v", err)
	}
	if len(reviews.Reviews) != 2 {
		t.Fatalf("review count = %d, want 2", len(reviews.Reviews))
	}
}

func TestProductFunnelMetricsUseCommittedChainState(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	_, userIDs, _ := seedDeliveryChain(t, database, adminmodel.ChainCompleted)

	var rejectedChainID int64
	if err := database.QueryRowContext(ctx, `
		INSERT INTO chains (status, cycle_key)
		VALUES ('REJECTED', $1)
		RETURNING id`, fmt.Sprintf("metrics-rejected:%d", time.Now().UnixNano())).Scan(&rejectedChainID); err != nil {
		t.Fatalf("create rejected metrics chain: %v", err)
	}
	if _, err := database.ExecContext(ctx, `
		INSERT INTO chain_rejections (chain_id, reason, actor_user_id)
		VALUES ($1, 'declined', $2)`, rejectedChainID, userIDs[0]); err != nil {
		t.Fatalf("record rejected metrics chain: %v", err)
	}
	if _, err := database.ExecContext(ctx, `UPDATE users SET role = 'ADMIN' WHERE id = $1`, userIDs[0]); err != nil {
		t.Fatalf("promote metrics user: %v", err)
	}

	repository, err := metricsrepository.NewPostgreSQL(database)
	if err != nil {
		t.Fatalf("create metrics repository: %v", err)
	}
	service, err := metricsservice.New(repository)
	if err != nil {
		t.Fatalf("create metrics service: %v", err)
	}
	if _, err := service.Funnel(ctx, userIDs[1]); !errors.Is(err, metricsmodel.ErrForbidden) {
		t.Fatalf("regular user metrics error = %v, want ErrForbidden", err)
	}

	snapshot, err := service.Funnel(ctx, userIDs[0])
	if err != nil {
		t.Fatalf("load product funnel: %v", err)
	}
	if snapshot.EligibleItems != 3 || snapshot.ItemsWithChain != 3 {
		t.Fatalf("item funnel = eligible %d, with chain %d; want 3 and 3", snapshot.EligibleItems, snapshot.ItemsWithChain)
	}
	if snapshot.ItemsWithChainRate == nil || *snapshot.ItemsWithChainRate != 1 {
		t.Fatalf("items with chain rate = %v, want 1", snapshot.ItemsWithChainRate)
	}
	if snapshot.AverageTimeToFirstChainSeconds == nil || *snapshot.AverageTimeToFirstChainSeconds < 0 {
		t.Fatalf("average time to first chain = %v, want non-negative value", snapshot.AverageTimeToFirstChainSeconds)
	}
	if snapshot.DecidedChains != 2 || snapshot.AcceptedChains != 1 || snapshot.CompletedChains != 1 {
		t.Fatalf("chain funnel = %#v", snapshot)
	}
	if snapshot.AcceptanceRate == nil || *snapshot.AcceptanceRate != 0.5 {
		t.Fatalf("acceptance rate = %v, want 0.5", snapshot.AcceptanceRate)
	}
	if snapshot.DeliveryCompletionRate == nil || *snapshot.DeliveryCompletionRate != 1 {
		t.Fatalf("delivery completion rate = %v, want 1", snapshot.DeliveryCompletionRate)
	}
	if len(snapshot.RejectionReasons) != 1 || snapshot.RejectionReasons[0].Reason != "declined" || snapshot.RejectionReasons[0].Count != 1 {
		t.Fatalf("rejection reasons = %#v", snapshot.RejectionReasons)
	}
}

func TestDeliveryTransitionCreatesClaimableOutboxEvent(t *testing.T) {
	database := newMigratedDatabase(t, 0)
	ctx := context.Background()
	chainID, _, deliveryIDs := seedDeliveryChain(t, database, adminmodel.ChainAccepted)

	store, err := outbox.NewStore(database)
	if err != nil {
		t.Fatalf("create outbox store: %v", err)
	}
	repository, err := adminrepository.NewPostgreSQLWithOutbox(database, store)
	if err != nil {
		t.Fatalf("create admin repository: %v", err)
	}
	var adminID int64
	if err := database.QueryRowContext(ctx, `SELECT id FROM users WHERE role = 'ADMIN' ORDER BY id LIMIT 1`).Scan(&adminID); err != nil {
		t.Fatalf("load admin: %v", err)
	}

	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[0], adminmodel.DeliveryAtPVZ); err != nil {
		t.Fatalf("transition delivery: %v", err)
	}
	if _, err := repository.TransitionDelivery(ctx, adminID, deliveryIDs[0], adminmodel.DeliveryAtPVZ); err != nil {
		t.Fatalf("repeat delivery transition: %v", err)
	}

	firstClaim, err := store.Claim(ctx, "worker-1", 10, time.Minute)
	if err != nil {
		t.Fatalf("first claim: %v", err)
	}
	secondClaim, err := store.Claim(ctx, "worker-2", 10, time.Minute)
	if err != nil {
		t.Fatalf("second claim: %v", err)
	}
	if len(firstClaim) != 1 || len(secondClaim) != 0 {
		t.Fatalf("claim sizes = %d/%d, want 1/0", len(firstClaim), len(secondClaim))
	}
	event := firstClaim[0]
	if event.Type != "chain.updated" || event.EntityID != strconv.FormatInt(chainID, 10) || len(event.RecipientIDs) != 3 {
		t.Fatalf("outbox event = %#v", event)
	}
	if err := store.MarkPublished(ctx, event.ID, "worker-1"); err != nil {
		t.Fatalf("mark published: %v", err)
	}
	remaining, err := store.Claim(ctx, "worker-2", 10, time.Minute)
	if err != nil {
		t.Fatalf("claim after publication: %v", err)
	}
	if len(remaining) != 0 {
		t.Fatalf("remaining events = %d, want 0", len(remaining))
	}
}

func TestPostgreSQLBroadcasterFansOutToBackendInstances(t *testing.T) {
	databaseURL, cleanup := newTestDatabase(t)
	t.Cleanup(cleanup)
	firstDatabase := openDatabase(t, databaseURL)
	secondDatabase := openDatabase(t, databaseURL)
	t.Cleanup(func() { _ = firstDatabase.Close() })
	t.Cleanup(func() { _ = secondDatabase.Close() })

	ctx, cancel := context.WithCancel(context.Background())
	t.Cleanup(cancel)
	firstHub := events.NewHub()
	secondHub := events.NewHub()
	first, err := outbox.NewPostgreSQLBroadcaster(firstDatabase, databaseURL, firstHub, zap.NewNop())
	if err != nil {
		t.Fatalf("create first broadcaster: %v", err)
	}
	second, err := outbox.NewPostgreSQLBroadcaster(secondDatabase, databaseURL, secondHub, zap.NewNop())
	if err != nil {
		_ = first.Close()
		t.Fatalf("create second broadcaster: %v", err)
	}
	t.Cleanup(func() { _ = first.Close() })
	t.Cleanup(func() { _ = second.Close() })
	go first.Run(ctx)
	go second.Run(ctx)

	firstSubscription := firstHub.Subscribe(ctx, 10)
	secondSubscription := secondHub.Subscribe(ctx, 10)
	want := outbox.Event{
		ID:           77,
		Type:         "chain.updated",
		EntityID:     "42",
		RecipientIDs: []int64{10},
		OccurredAt:   time.Now().UTC(),
	}
	if err := first.Publish(ctx, want); err != nil {
		t.Fatalf("publish realtime event: %v", err)
	}

	for instance, subscription := range map[string]<-chan events.Event{
		"first":  firstSubscription,
		"second": secondSubscription,
	} {
		select {
		case got := <-subscription:
			if got.ID != "outbox:77" || got.EntityID != want.EntityID {
				t.Fatalf("%s backend event = %#v", instance, got)
			}
		case <-time.After(3 * time.Second):
			t.Fatalf("%s backend did not receive realtime event", instance)
		}
	}
}

func seedDeliveryChain(t *testing.T, database *sql.DB, status string) (int64, []int64, []int64) {
	t.Helper()
	ctx := context.Background()
	suffix := time.Now().UnixNano()
	userIDs := make([]int64, 3)
	itemIDs := make([]int64, 3)
	for index := range userIDs {
		if err := database.QueryRowContext(ctx, `
			INSERT INTO users (username, phone)
			VALUES ($1, $2)
			RETURNING id`, fmt.Sprintf("delivery-user-%d-%d", suffix, index), fmt.Sprintf("+78%09d%d", suffix%1_000_000_000, index)).Scan(&userIDs[index]); err != nil {
			t.Fatalf("create delivery user %d: %v", index, err)
		}
		if err := database.QueryRowContext(ctx, `
			INSERT INTO items (user_id, offer_title, status)
			VALUES ($1, $2, 'LOCKED')
			RETURNING id`, userIDs[index], fmt.Sprintf("delivery-item-%d", index)).Scan(&itemIDs[index]); err != nil {
			t.Fatalf("create delivery item %d: %v", index, err)
		}
	}

	var chainID int64
	if err := database.QueryRowContext(ctx, `
		INSERT INTO chains (status, cycle_key)
		VALUES ($1::chain_status, $2)
		RETURNING id`, status, fmt.Sprintf("integration:%d", suffix)).Scan(&chainID); err != nil {
		t.Fatalf("create delivery chain: %v", err)
	}
	deliveryIDs := make([]int64, 3)
	for index := range userIDs {
		deliveryStatus := adminmodel.DeliveryAwaitingPVZ
		if status == adminmodel.ChainCompleted {
			deliveryStatus = adminmodel.DeliveryReceived
		}
		if err := database.QueryRowContext(ctx, `
			INSERT INTO chain_items (chain_id, item_id, user_id, next_item_id, status, delivery_status)
			VALUES ($1, $2, $3, $4, 'APPROVED', $5::delivery_status)
			RETURNING id`, chainID, itemIDs[index], userIDs[index], itemIDs[(index+1)%3], deliveryStatus).Scan(&deliveryIDs[index]); err != nil {
			t.Fatalf("create delivery leg %d: %v", index, err)
		}
	}
	return chainID, userIDs, deliveryIDs
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
		{table: "matching_jobs", name: "status"},
		{table: "matching_jobs", name: "attempts"},
		{table: "matching_jobs", name: "available_at"},
		{table: "matching_jobs", name: "locked_at"},
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
	if systemCategories != 10 {
		t.Fatalf("system category count = %d, want 10", systemCategories)
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
