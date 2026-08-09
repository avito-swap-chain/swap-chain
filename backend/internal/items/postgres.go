package items

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/lib/pq"
	"github.com/pgvector/pgvector-go"
	"go.uber.org/zap"
)

const (
	analysisQueueSize = 64
	analysisTimeout   = 60 * time.Second
)

type vectorizer interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

type repository interface {
	Create(ctx context.Context, userID int64, input CreateInput) (Item, error)
	Get(ctx context.Context, itemID int64) (Item, error)
	ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error)
	MarkMatching(ctx context.Context, itemID int64, offerEmbedding, wantEmbedding []float32) (Item, error)
}

// PublishEvent sends an item lifecycle event after its database change commits.
type PublishEvent func(userID int64, eventType, entityID string, data map[string]any)

// PostgresService stores items and owns the background analysis worker.
type PostgresService struct {
	repo       repository
	vectorizer vectorizer
	publish    PublishEvent
	logger     *zap.Logger
	jobs       chan Item
	ctx        context.Context
	cancel     context.CancelFunc
	wg         sync.WaitGroup
}

// NewPostgresService creates a PostgreSQL-backed item service and starts analysis.
func NewPostgresService(database *sql.DB, vectorizer vectorizer, publish PublishEvent, logger *zap.Logger) *PostgresService {
	return newPostgresService(&postgresRepository{database: database}, vectorizer, publish, logger)
}

func newPostgresService(repo repository, vectorizer vectorizer, publish PublishEvent, logger *zap.Logger) *PostgresService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &PostgresService{
		repo:       repo,
		vectorizer: vectorizer,
		publish:    publish,
		logger:     logger,
		jobs:       make(chan Item, analysisQueueSize),
		ctx:        ctx,
		cancel:     cancel,
	}
	service.wg.Add(1)
	go service.runWorker()
	return service
}

// Close stops the analysis worker and waits for its current request to finish.
func (s *PostgresService) Close() {
	s.cancel()
	s.wg.Wait()
}

// Create persists an ANALYZING item and schedules both embeddings.
func (s *PostgresService) Create(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	input = normalize(input)
	if err := validate(input); err != nil {
		return Item{}, err
	}

	item, err := s.repo.Create(ctx, userID, input)
	if err != nil {
		return Item{}, err
	}

	s.notify(item, "item.created")
	select {
	case s.jobs <- item:
		return item, nil
	case <-s.ctx.Done():
		return Item{}, errors.New("item analysis service is stopped")
	}
}

// Get returns one persisted item by ID.
func (s *PostgresService) Get(ctx context.Context, itemID int64) (Item, error) {
	return s.repo.Get(ctx, itemID)
}

// ListByUser returns a stable ID-ordered page owned by one user.
func (s *PostgresService) ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error) {
	return s.repo.ListByUser(ctx, userID, afterID, limit)
}

func (s *PostgresService) runWorker() {
	defer s.wg.Done()
	for {
		select {
		case <-s.ctx.Done():
			return
		case item := <-s.jobs:
			s.analyze(item)
		}
	}
}

func (s *PostgresService) analyze(item Item) {
	ctx, cancel := context.WithTimeout(s.ctx, analysisTimeout)
	defer cancel()

	offerText := strings.TrimSpace(item.OfferTitle + ". " + item.OfferDescription)
	offerEmbedding, err := s.vectorizer.Vectorize(ctx, offerText)
	if err != nil {
		s.logger.Error("vectorize item offer", zap.Int64("item_id", item.ID), zap.Error(err))
		return
	}
	wantEmbedding, err := s.vectorizer.Vectorize(ctx, item.WantDescription)
	if err != nil {
		s.logger.Error("vectorize item want", zap.Int64("item_id", item.ID), zap.Error(err))
		return
	}

	updated, err := s.repo.MarkMatching(ctx, item.ID, offerEmbedding, wantEmbedding)
	if err != nil {
		s.logger.Error("complete item analysis", zap.Int64("item_id", item.ID), zap.Error(err))
		return
	}
	s.notify(updated, "item.status.updated")
}

func (s *PostgresService) notify(item Item, eventType string) {
	if s.publish == nil {
		return
	}
	s.publish(item.UserID, eventType, strconv.FormatInt(item.ID, 10), map[string]any{"status": item.Status})
}

type postgresRepository struct {
	database *sql.DB
}

func (r *postgresRepository) Create(ctx context.Context, userID int64, input CreateInput) (Item, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, fmt.Errorf("begin create item: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	row := tx.QueryRowContext(ctx, `
		INSERT INTO items (user_id, offer_title, offer_description, want_description, image_urls)
		VALUES ($1, $2, $3, $4, $5)
		RETURNING id, user_id, offer_title, offer_description, want_description,
		          image_urls, status::text, created_at, updated_at`,
		userID, input.OfferTitle, input.OfferDescription, input.WantDescription, pq.Array(input.ImageURLs),
	)
	item, err := scanItem(row)
	if err != nil {
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("commit create item: %w", err)
	}
	return item, nil
}

func (r *postgresRepository) Get(ctx context.Context, itemID int64) (Item, error) {
	item, err := scanItem(r.database.QueryRowContext(ctx, `
		SELECT id, user_id, offer_title, offer_description, want_description,
		       image_urls, status::text, created_at, updated_at
		FROM items WHERE id = $1`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("get item: %w", err)
	}
	return item, nil
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT id, user_id, offer_title, offer_description, want_description,
		       image_urls, status::text, created_at, updated_at
		FROM items
		WHERE user_id = $1 AND id > $2
		ORDER BY id LIMIT $3`, userID, afterID, limit+1)
	if err != nil {
		return nil, nil, fmt.Errorf("list items: %w", err)
	}
	defer func() { _ = rows.Close() }()

	result := make([]Item, 0, limit+1)
	for rows.Next() {
		item, scanErr := scanItem(rows)
		if scanErr != nil {
			return nil, nil, fmt.Errorf("scan listed item: %w", scanErr)
		}
		result = append(result, item)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate items: %w", err)
	}

	var next *int64
	if len(result) > limit {
		value := result[limit-1].ID
		next = &value
		result = result[:limit]
	}
	return result, next, nil
}

func (r *postgresRepository) MarkMatching(ctx context.Context, itemID int64, offerEmbedding, wantEmbedding []float32) (Item, error) {
	item, err := scanItem(r.database.QueryRowContext(ctx, `
		UPDATE items
		SET offer_embedding = $2, want_embedding = $3, status = 'MATCHING', updated_at = now()
		WHERE id = $1 AND status = 'ANALYZING'
		RETURNING id, user_id, offer_title, offer_description, want_description,
		          image_urls, status::text, created_at, updated_at`,
		itemID, pgvector.NewVector(offerEmbedding), pgvector.NewVector(wantEmbedding),
	))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("mark item matching: %w", err)
	}
	return item, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var item Item
	var imageURLs pq.StringArray
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.OfferTitle,
		&item.OfferDescription,
		&item.WantDescription,
		&imageURLs,
		&item.Status,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	item.ImageURLs = append([]string(nil), imageURLs...)
	return item, err
}
