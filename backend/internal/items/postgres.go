package items

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strconv"
	"sync"
	"time"

	"github.com/lib/pq"
	"go.uber.org/zap"
)

const (
	analysisQueueSize = 64
)

type analyzer interface {
	AnalyzeItem(ctx context.Context, itemID int64) error
}

type repository interface {
	Create(ctx context.Context, userID int64, input CreateInput) (Item, error)
	Update(ctx context.Context, userID, itemID int64, input UpdateInput) (updateResult, error)
	ResolveCategories(ctx context.Context, userID, itemID int64, input CategoryDecisionInput) (Item, error)
	Get(ctx context.Context, itemID int64) (Item, error)
	ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error)
}

type chainRejection struct {
	ChainID int64
	UserIDs []int64
}

type updateResult struct {
	Item       Item
	Rejections []chainRejection
}

// PublishEvent sends an item lifecycle event after its database change commits.
type PublishEvent func(userID int64, eventType, entityID string, data map[string]any)

// PostgresService stores items and owns the background analysis worker.
type PostgresService struct {
	repo     repository
	analyzer analyzer
	publish  PublishEvent
	logger   *zap.Logger
	timeout  time.Duration
	jobs     chan int64
	ctx      context.Context
	cancel   context.CancelFunc
	wg       sync.WaitGroup
}

// NewPostgresService creates a PostgreSQL-backed item service and starts analysis.
func NewPostgresService(database *sql.DB, analyzer analyzer, publish PublishEvent, logger *zap.Logger, analysisTimeout time.Duration) *PostgresService {
	return newPostgresService(&postgresRepository{database: database}, analyzer, publish, logger, analysisTimeout)
}

func newPostgresService(repo repository, analyzer analyzer, publish PublishEvent, logger *zap.Logger, analysisTimeout time.Duration) *PostgresService {
	ctx, cancel := context.WithCancel(context.Background())
	service := &PostgresService{
		repo:     repo,
		analyzer: analyzer,
		publish:  publish,
		logger:   logger,
		timeout:  analysisTimeout,
		jobs:     make(chan int64, analysisQueueSize),
		ctx:      ctx,
		cancel:   cancel,
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
	case s.jobs <- item.ID:
		return item, nil
	case <-s.ctx.Done():
		return Item{}, errors.New("item analysis service is stopped")
	}
}

// Get returns one persisted item by ID.
func (s *PostgresService) Get(ctx context.Context, itemID int64) (Item, error) {
	return s.repo.Get(ctx, itemID)
}

// Update changes matching inputs or withdraws an item. Any affected pending
// proposal is rejected before the new state becomes visible.
func (s *PostgresService) Update(ctx context.Context, userID, itemID int64, input UpdateInput) (Item, error) {
	input = normalizeUpdate(input)
	if err := validateUpdate(input); err != nil {
		return Item{}, err
	}

	result, err := s.repo.Update(ctx, userID, itemID, input)
	if err != nil {
		return Item{}, err
	}
	for _, rejected := range result.Rejections {
		reason := "item_changed"
		if input.Withdraw {
			reason = "item_withdrawn"
		}
		for _, recipientID := range rejected.UserIDs {
			if s.publish != nil {
				s.publish(recipientID, "chain.rejected", strconv.FormatInt(rejected.ChainID, 10), map[string]any{
					"reason": reason,
					"itemId": result.Item.ID,
				})
			}
		}
	}
	s.notify(result.Item, "item.status.updated")
	if result.Item.Status == "ANALYZING" {
		select {
		case s.jobs <- result.Item.ID:
		case <-s.ctx.Done():
			return Item{}, errors.New("item analysis service is stopped")
		}
	}
	return result.Item, nil
}

// ResolveCategories применяет ручной выбор владельца и сразу возвращает
// полностью категоризированную карточку в matching.
func (s *PostgresService) ResolveCategories(
	ctx context.Context,
	userID, itemID int64,
	input CategoryDecisionInput,
) (Item, error) {
	if err := validateCategoryDecision(input); err != nil {
		return Item{}, err
	}
	item, err := s.repo.ResolveCategories(ctx, userID, itemID, input)
	if err != nil {
		return Item{}, err
	}
	s.notify(item, "item.status.updated")
	return item, nil
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
		case itemID := <-s.jobs:
			s.analyze(itemID)
		}
	}
}

func (s *PostgresService) analyze(itemID int64) {
	ctx, cancel := context.WithTimeout(s.ctx, s.timeout)
	defer cancel()

	if err := s.analyzer.AnalyzeItem(ctx, itemID); err != nil {
		s.logger.Error("analyze item", zap.Int64("item_id", itemID), zap.Error(err))
		return
	}
	updated, err := s.repo.Get(ctx, itemID)
	if err != nil {
		s.logger.Error("load analyzed item", zap.Int64("item_id", itemID), zap.Error(err))
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
		INSERT INTO items (user_id, offer_title, offer_description, image_urls, offer_category_id, is_category_manual, quality_score)
		VALUES ($1, $2, $3, $4, $5, $6, $7)
		RETURNING id, user_id, offer_title, offer_description,
		          image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		          created_at, updated_at`,
		userID, input.OfferTitle, input.OfferDescription, pq.Array(input.ImageURLs), input.OfferCategoryID,
		input.OfferCategoryID != nil, conditionScore(input.Condition),
	)
	item, err := scanItem(row)
	if err != nil {
		return Item{}, fmt.Errorf("insert item: %w", err)
	}
	for _, wish := range input.Wishes {
		_, err := tx.ExecContext(ctx, `INSERT INTO item_wishes (item_id, want_description) VALUES ($1, $2)`, item.ID, wish)
		if err != nil {
			return Item{}, fmt.Errorf("insert wish: %w", err)
		}
	}
	item.Wishes, err = loadWishesTx(ctx, tx, item.ID)
	if err != nil {
		return Item{}, fmt.Errorf("load wishes: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("commit create item: %w", err)
	}
	return item, nil
}

func (r *postgresRepository) Update(ctx context.Context, userID, itemID int64, input UpdateInput) (updateResult, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return updateResult{}, fmt.Errorf("begin update item: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, itemID); err != nil {
		return updateResult{}, fmt.Errorf("lock item %d: %w", itemID, err)
	}
	current, err := scanItem(tx.QueryRowContext(ctx, `
		SELECT id, user_id, offer_title, offer_description,
		       image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		       created_at, updated_at
		FROM items WHERE id = $1 FOR UPDATE`, itemID))
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return updateResult{}, ErrNotFound
		}
		return updateResult{}, fmt.Errorf("lock item: %w", err)
	}
	if current.UserID != userID {
		return updateResult{}, ErrForbidden
	}
	if current.Status == "LOCKED" || current.Status == "EXCHANGED" {
		return updateResult{}, ErrConflict
	}

	offerTitle := current.OfferTitle
	titleChanged := false
	offerDescription := current.OfferDescription
	if input.OfferTitle != nil {
		offerTitle = *input.OfferTitle
		if offerTitle != current.OfferTitle {
			titleChanged = true
		}
	}
	if input.OfferDescription != nil {
		offerDescription = *input.OfferDescription
	}

	status := current.Status
	matchingChanged := false
	offerCategoryChanged := input.OfferCategoryID != nil
	switch {
	case input.Withdraw:
		status = "WITHDRAWN"
		matchingChanged = current.Status != "WITHDRAWN"
	case input.Wishes != nil:
		status = "ANALYZING"
		matchingChanged = true
	case input.OfferDescription != nil && current.Status != "WITHDRAWN":
		status = "ANALYZING"
		matchingChanged = true
	case input.OfferTitle != nil && titleChanged && current.Status != "WITHDRAWN":
		status = "ANALYZING"
		matchingChanged = true
	case input.Condition != nil && *input.Condition != current.Condition && current.Status != "WITHDRAWN":
		status = "ANALYZING"
		matchingChanged = true
	}
	if offerCategoryChanged && current.Status != "WITHDRAWN" {
		status = "ANALYZING"
		matchingChanged = true
	}

	rejections := make([]chainRejection, 0)
	if matchingChanged {
		reason := "item_changed"
		if input.Withdraw {
			reason = "item_withdrawn"
		}
		rejections, err = rejectPendingChainsForItem(ctx, tx, userID, itemID, reason)
		if err != nil {
			return updateResult{}, err
		}
	}

	row := tx.QueryRowContext(ctx, `
		UPDATE items
		SET offer_title = $5,
		    offer_description = $2,
		    status = $3::item_status,
		    offer_category_id = CASE
		        WHEN $6::int IS NOT NULL THEN $6::int
		        WHEN $4 AND NOT is_category_manual THEN NULL
		        ELSE offer_category_id
		    END,
		    is_category_manual = CASE WHEN $6::int IS NOT NULL THEN TRUE ELSE is_category_manual END,
		    quality_score = CASE WHEN $7::numeric IS NULL THEN quality_score ELSE $7::numeric END,
		    param_richness = CASE WHEN $4 THEN NULL ELSE param_richness END,
		    offer_embedding_local = CASE WHEN $4 THEN NULL ELSE offer_embedding_local END,
		    offer_embedding_external = CASE WHEN $4 THEN NULL ELSE offer_embedding_external END,
		    analysis_version = CASE WHEN $4 THEN analysis_version + 1 ELSE analysis_version END,
		    last_status_updated_at = CASE WHEN $4 THEN now() ELSE last_status_updated_at END,
		    updated_at = now()
		WHERE id = $1
		RETURNING id, user_id, offer_title, offer_description,
		          image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		          created_at, updated_at`,
		itemID, offerDescription, status, matchingChanged, offerTitle, input.OfferCategoryID, nullableConditionScore(input.Condition),
	)
	item, err := scanItem(row)
	if err != nil {
		return updateResult{}, fmt.Errorf("update item: %w", err)
	}
	if input.Withdraw {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_wishes WHERE item_id = $1`, item.ID); err != nil {
			return updateResult{}, fmt.Errorf("delete wishes: %w", err)
		}
	} else if input.Wishes != nil {
		if _, err := tx.ExecContext(ctx, `DELETE FROM item_wishes WHERE item_id = $1`, item.ID); err != nil {
			return updateResult{}, fmt.Errorf("delete wishes: %w", err)
		}
		for _, wish := range input.Wishes {
			_, err := tx.ExecContext(ctx, `INSERT INTO item_wishes (item_id, want_description) VALUES ($1, $2)`, item.ID, wish)
			if err != nil {
				return updateResult{}, fmt.Errorf("insert wish: %w", err)
			}
		}
	}
	item.Wishes, err = loadWishesTx(ctx, tx, item.ID)
	if err != nil {
		return updateResult{}, fmt.Errorf("load wishes: %w", err)
	}

	if matchingChanged {
		if _, err := tx.ExecContext(ctx, `
			UPDATE matching_jobs
			SET status = 'DONE', locked_at = NULL, last_error = NULL, updated_at = now()
			WHERE item_id = $1`, itemID); err != nil {
			return updateResult{}, fmt.Errorf("cancel matching job: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return updateResult{}, fmt.Errorf("commit update item: %w", err)
	}
	return updateResult{Item: item, Rejections: rejections}, nil
}

func (r *postgresRepository) ResolveCategories(
	ctx context.Context,
	userID, itemID int64,
	input CategoryDecisionInput,
) (Item, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return Item{}, fmt.Errorf("begin category decision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, itemID); err != nil {
		return Item{}, fmt.Errorf("lock item %d: %w", itemID, err)
	}
	item, err := scanItem(tx.QueryRowContext(ctx, `
		SELECT id, user_id, offer_title, offer_description,
		       image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		       created_at, updated_at
		FROM items
		WHERE id = $1
		FOR UPDATE`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("lock item for category decision: %w", err)
	}
	if item.UserID != userID {
		return Item{}, ErrForbidden
	}
	if item.Status != "ACTION_REQUIRED" {
		return Item{}, ErrConflict
	}

	wishRows, err := tx.QueryContext(ctx, `
		SELECT id, want_category_id
		FROM item_wishes
		WHERE item_id = $1
		ORDER BY id
		FOR UPDATE`, itemID)
	if err != nil {
		return Item{}, fmt.Errorf("lock wishes for category decision: %w", err)
	}
	defer func() { _ = wishRows.Close() }()

	type wishState struct {
		categoryID *int32
	}
	wishStates := make(map[int64]wishState)
	for wishRows.Next() {
		var wishID int64
		var categoryID sql.NullInt32
		if err := wishRows.Scan(&wishID, &categoryID); err != nil {
			return Item{}, fmt.Errorf("scan wish category decision: %w", err)
		}
		var category *int32
		if categoryID.Valid {
			value := categoryID.Int32
			category = &value
		}
		wishStates[wishID] = wishState{categoryID: category}
	}
	if err := wishRows.Err(); err != nil {
		return Item{}, fmt.Errorf("iterate wish category decisions: %w", err)
	}
	if err := wishRows.Close(); err != nil {
		return Item{}, fmt.Errorf("close wish category decisions: %w", err)
	}

	fields := make(map[string]string)
	if item.OfferCategoryID == nil {
		switch {
		case input.OfferCategoryID == nil:
			fields["offerCategoryId"] = "manual category is required"
		}
	} else if input.OfferCategoryID != nil {
		fields["offerCategoryId"] = "offer category does not require a decision"
	}

	decisions := make(map[int64]int32, len(input.Wishes))
	for _, decision := range input.Wishes {
		state, ok := wishStates[decision.WishID]
		switch {
		case !ok:
			fields["wishes"] = "contains a wish that does not belong to this item"
		case state.categoryID != nil:
			fields["wishes"] = "contains a wish that does not require a decision"
		default:
			decisions[decision.WishID] = decision.CategoryID
		}
	}
	for wishID, state := range wishStates {
		if state.categoryID == nil {
			if _, ok := decisions[wishID]; !ok {
				fields["wishes"] = "a category is required for every unresolved wish"
			}
		}
	}
	if len(fields) > 0 {
		return Item{}, &ValidationError{Fields: fields}
	}

	for wishID, categoryID := range decisions {
		if _, err := tx.ExecContext(ctx, `
			UPDATE item_wishes
			SET want_category_id = $1,
			    is_category_manual = TRUE
			WHERE id = $2 AND item_id = $3`, categoryID, wishID, itemID); err != nil {
			return Item{}, fmt.Errorf("apply wish %d category decision: %w", wishID, err)
		}
	}

	selectedOfferCategory := item.OfferCategoryID
	offerManual := item.OfferCategoryIsManual
	if input.OfferCategoryID != nil {
		selectedOfferCategory = input.OfferCategoryID
		offerManual = true
	}
	item, err = scanItem(tx.QueryRowContext(ctx, `
		UPDATE items
		SET offer_category_id = $2,
		    is_category_manual = $3,
		    status = 'MATCHING',
		    last_status_updated_at = NOW(),
		    updated_at = NOW()
		WHERE id = $1 AND status = 'ACTION_REQUIRED'
		RETURNING id, user_id, offer_title, offer_description,
		          image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		          created_at, updated_at`,
		itemID, selectedOfferCategory, offerManual))
	if err != nil {
		return Item{}, fmt.Errorf("complete category decision: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO matching_jobs (item_id, status, attempts, available_at, locked_at, last_error, updated_at)
		VALUES ($1, 'PENDING', 0, NOW(), NULL, NULL, NOW())
		ON CONFLICT (item_id) DO UPDATE
		SET status = 'PENDING', attempts = 0, available_at = NOW(), locked_at = NULL,
		    last_error = NULL, updated_at = NOW()`, itemID); err != nil {
		return Item{}, fmt.Errorf("schedule matching after category decision: %w", err)
	}
	item.Wishes, err = loadWishesTx(ctx, tx, itemID)
	if err != nil {
		return Item{}, fmt.Errorf("load resolved wishes: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return Item{}, fmt.Errorf("commit category decision: %w", err)
	}
	return item, nil
}

func rejectPendingChainsForItem(ctx context.Context, tx *sql.Tx, actorID, itemID int64, reason string) ([]chainRejection, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT c.id, participant.user_id
		FROM chains c
		JOIN chain_items changed_item ON changed_item.chain_id = c.id AND changed_item.item_id = $1
		JOIN chain_items participant ON participant.chain_id = c.id
		WHERE c.status = 'PENDING'
		ORDER BY c.id, participant.user_id
		FOR UPDATE OF c, changed_item, participant`, itemID)
	if err != nil {
		return nil, fmt.Errorf("lock pending item chains: %w", err)
	}
	defer func() { _ = rows.Close() }()

	byChain := make(map[int64][]int64)
	order := make([]int64, 0)
	for rows.Next() {
		var chainID, recipientID int64
		if err := rows.Scan(&chainID, &recipientID); err != nil {
			return nil, fmt.Errorf("scan pending item chain: %w", err)
		}
		if _, seen := byChain[chainID]; !seen {
			order = append(order, chainID)
		}
		byChain[chainID] = append(byChain[chainID], recipientID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate pending item chains: %w", err)
	}
	if len(order) == 0 {
		return nil, nil
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE chain_items SET status = 'DECLINED', updated_at = now()
		WHERE item_id = $1 AND chain_id = ANY($2)`, itemID, pq.Array(order)); err != nil {
		return nil, fmt.Errorf("decline changed item in pending chains: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE chains SET status = 'REJECTED', updated_at = now()
		WHERE id = ANY($1)`, pq.Array(order)); err != nil {
		return nil, fmt.Errorf("reject pending item chains: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chain_rejections (chain_id, reason, actor_user_id, item_id)
		SELECT unnest($1::bigint[]), $2, $3, $4
		ON CONFLICT (chain_id) DO NOTHING`, pq.Array(order), reason, actorID, itemID); err != nil {
		return nil, fmt.Errorf("record pending item chain rejections: %w", err)
	}

	result := make([]chainRejection, 0, len(order))
	for _, chainID := range order {
		result = append(result, chainRejection{ChainID: chainID, UserIDs: byChain[chainID]})
	}
	return result, nil
}

func (r *postgresRepository) Get(ctx context.Context, itemID int64) (Item, error) {
	item, err := scanItem(r.database.QueryRowContext(ctx, `
		SELECT id, user_id, offer_title, offer_description,
		       image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		       created_at, updated_at
		FROM items WHERE id = $1`, itemID))
	if errors.Is(err, sql.ErrNoRows) {
		return Item{}, ErrNotFound
	}
	if err != nil {
		return Item{}, fmt.Errorf("get item: %w", err)
	}
	item.Wishes, err = loadWishesDB(ctx, r.database, item.ID)
	if err != nil {
		return Item{}, fmt.Errorf("load wishes: %w", err)
	}
	return item, nil
}

func (r *postgresRepository) ListByUser(ctx context.Context, userID, afterID int64, limit int) ([]Item, *int64, error) {
	rows, err := r.database.QueryContext(ctx, `
		SELECT id, user_id, offer_title, offer_description,
		       image_urls, status::text, offer_category_id, is_category_manual, quality_score,
		       created_at, updated_at
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
		item.Wishes, err = loadWishesDB(ctx, r.database, item.ID)
		if err != nil {
			return nil, nil, fmt.Errorf("load wishes: %w", err)
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

type rowScanner interface {
	Scan(dest ...any) error
}

func scanItem(row rowScanner) (Item, error) {
	var item Item
	var offerCategoryID sql.NullInt32
	var qualityScore string
	var imageURLs pq.StringArray
	err := row.Scan(
		&item.ID,
		&item.UserID,
		&item.OfferTitle,
		&item.OfferDescription,
		&imageURLs,
		&item.Status,
		&offerCategoryID,
		&item.OfferCategoryIsManual,
		&qualityScore,
		&item.CreatedAt,
		&item.UpdatedAt,
	)
	if offerCategoryID.Valid {
		item.OfferCategoryID = &offerCategoryID.Int32
	}
	item.Condition = conditionFromScore(qualityScore)
	if imageURLs == nil {
		item.ImageURLs = []string{}
	} else {
		item.ImageURLs = append([]string(nil), imageURLs...)
	}
	return item, err
}

func conditionScore(condition string) float64 {
	switch condition {
	case ConditionNew:
		return 1
	case ConditionUsed:
		return 0.6
	default:
		return 0.8
	}
}

func nullableConditionScore(condition *string) any {
	if condition == nil {
		return nil
	}
	return conditionScore(*condition)
}

func conditionFromScore(score string) string {
	switch score {
	case "1.000":
		return ConditionNew
	case "0.600":
		return ConditionUsed
	default:
		return ConditionGood
	}
}

type queryer interface {
	QueryContext(ctx context.Context, query string, args ...any) (*sql.Rows, error)
}

func loadWishesTx(ctx context.Context, q queryer, itemID int64) ([]ItemWish, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT id, want_category_id, want_description
		FROM item_wishes
		WHERE item_id = $1
		ORDER BY id`, itemID)
	if err != nil {
		return nil, err
	}
	defer func() { _ = rows.Close() }()

	var wishes []ItemWish
	for rows.Next() {
		var wish ItemWish
		var catID sql.NullInt32
		if err := rows.Scan(&wish.ID, &catID, &wish.Description); err != nil {
			return nil, err
		}
		if catID.Valid {
			wish.CategoryID = &catID.Int32
		}
		wishes = append(wishes, wish)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	if wishes == nil {
		wishes = []ItemWish{}
	}
	return wishes, nil
}

func loadWishesDB(ctx context.Context, db *sql.DB, itemID int64) ([]ItemWish, error) {
	return loadWishesTx(ctx, db, itemID)
}
