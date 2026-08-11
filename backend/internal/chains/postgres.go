package chains

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sort"
	"strconv"
	"time"

	"github.com/lib/pq"

	"swap-chain/internal/outbox"
)

const proposalLifetime = 24 * time.Hour

// PublishEvent sends an event to explicit users after a database commit.
type PublishEvent func(userIDs []int64, eventType, entityID string, data map[string]any)

type eventOutbox interface {
	EnqueueTx(ctx context.Context, tx *sql.Tx, event outbox.PendingEvent) error
}

// PostgresService persists and transitions exchange-chain aggregates.
type PostgresService struct {
	database *sql.DB
	publish  PublishEvent
	outbox   eventOutbox
	now      func() time.Time
}

// NewPostgresService creates a PostgreSQL-backed chain service.
func NewPostgresService(database *sql.DB, publish PublishEvent) *PostgresService {
	return NewPostgresServiceWithOutbox(database, publish, nil)
}

// NewPostgresServiceWithOutbox creates a chain service with durable realtime events.
func NewPostgresServiceWithOutbox(database *sql.DB, publish PublishEvent, eventOutbox eventOutbox) *PostgresService {
	return &PostgresService{database: database, publish: publish, outbox: eventOutbox, now: time.Now}
}

// Create validates and persists a pending proposal from a closed matching cycle.
func (s *PostgresService) Create(ctx context.Context, userID int64, input CreateInput) (Chain, error) {
	itemIDs, cycleKey, err := validateCreate(input)
	if err != nil {
		return Chain{}, err
	}

	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Chain{}, fmt.Errorf("begin create chain: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockItems(ctx, tx, itemIDs); err != nil {
		return Chain{}, err
	}
	owners, err := loadEligibleItemOwners(ctx, tx, itemIDs)
	if err != nil {
		return Chain{}, err
	}
	if !containsOwner(owners, userID) {
		return Chain{}, ErrForbidden
	}

	var chainID int64
	expiresAt := s.now().UTC().Add(proposalLifetime)
	if err := tx.QueryRowContext(ctx, `
		INSERT INTO chains (status, expires_at, updated_at, cycle_key)
		VALUES ('PENDING', $1, now(), $2)
		RETURNING id`, expiresAt, cycleKey,
	).Scan(&chainID); err != nil {
		var pqError *pq.Error
		if errors.As(err, &pqError) && pqError.Code == "23505" {
			return Chain{}, ErrConflict
		}
		return Chain{}, fmt.Errorf("insert chain: %w", err)
	}

	for _, edge := range input.Edges {
		ownerID := owners[edge.SourceItemID]
		status := ParticipantWaiting
		if ownerID == userID {
			status = ParticipantApproved
		}
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO chain_items (chain_id, item_id, user_id, next_item_id, status, updated_at)
			VALUES ($1, $2, $3, $4, $5::participant_status, now())`,
			chainID, edge.SourceItemID, ownerID, edge.TargetItemID, status,
		); err != nil {
			return Chain{}, fmt.Errorf("insert chain participant: %w", err)
		}
	}

	chain, err := loadChain(ctx, tx, chainID)
	if err != nil {
		return Chain{}, err
	}
	if err := s.enqueueChainEvent(ctx, tx, chain, "chain.created", "created", nil); err != nil {
		return Chain{}, err
	}
	if err := tx.Commit(); err != nil {
		return Chain{}, fmt.Errorf("commit create chain: %w", err)
	}
	s.notify(chain, "chain.created", nil)
	return chain, nil
}

// KnownCycleKeys returns persisted directed cycle signatures from every lifecycle state.
func (s *PostgresService) KnownCycleKeys(ctx context.Context, keys []string) (map[string]struct{}, error) {
	known := make(map[string]struct{})
	if len(keys) == 0 {
		return known, nil
	}
	rows, err := s.database.QueryContext(ctx, `SELECT cycle_key FROM chains WHERE cycle_key = ANY($1)`, pq.Array(keys))
	if err != nil {
		return nil, fmt.Errorf("load known cycle keys: %w", err)
	}
	defer closeRows(rows)
	for rows.Next() {
		var key string
		if err := rows.Scan(&key); err != nil {
			return nil, fmt.Errorf("scan known cycle key: %w", err)
		}
		known[key] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate known cycle keys: %w", err)
	}
	return known, nil
}

// List returns chains involving the requested user.
func (s *PostgresService) List(ctx context.Context, userID int64, status string, afterID int64, limit int) ([]Chain, *int64, error) {
	if limit < 1 || limit > 100 {
		return nil, nil, &ValidationError{Message: "limit must be between 1 and 100"}
	}
	if status != "" && status != StatusPending && status != StatusAccepted && status != StatusRejected && status != StatusCompleted {
		return nil, nil, &ValidationError{Message: "unsupported chain status"}
	}

	rows, err := s.database.QueryContext(ctx, `
		SELECT c.id
		FROM chains c
		WHERE c.id > $1
		  AND ($2 = '' OR c.status::text = $2)
		  AND EXISTS (SELECT 1 FROM chain_items ci WHERE ci.chain_id = c.id AND ci.user_id = $3)
		ORDER BY c.id
		LIMIT $4`, afterID, status, userID, limit+1)
	if err != nil {
		return nil, nil, fmt.Errorf("list chain IDs: %w", err)
	}
	defer closeRows(rows)

	ids := make([]int64, 0, limit+1)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, nil, fmt.Errorf("scan chain ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("iterate chain IDs: %w", err)
	}

	var next *int64
	if len(ids) > limit {
		value := ids[limit-1]
		next = &value
		ids = ids[:limit]
	}
	result := make([]Chain, 0, len(ids))
	for _, id := range ids {
		chain, err := loadChain(ctx, s.database, id)
		if err != nil {
			return nil, nil, err
		}
		result = append(result, chain)
	}
	return result, next, nil
}

// Get returns a chain only when the requested user participates in it.
func (s *PostgresService) Get(ctx context.Context, userID, chainID int64) (Chain, error) {
	chain, err := loadChain(ctx, s.database, chainID)
	if err != nil {
		return Chain{}, err
	}
	if !hasParticipant(chain, userID) {
		return Chain{}, ErrForbidden
	}
	return chain, nil
}

// Decide records a participant decision and atomically accepts or rejects the chain.
func (s *PostgresService) Decide(ctx context.Context, userID, chainID int64, decision string) (Chain, error) {
	if err := validateDecision(decision); err != nil {
		return Chain{}, err
	}

	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Chain{}, fmt.Errorf("begin chain decision: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	itemIDs, err := loadChainItemIDs(ctx, tx, chainID)
	if err != nil {
		return Chain{}, err
	}
	if err := lockItems(ctx, tx, itemIDs); err != nil {
		return Chain{}, err
	}

	var status string
	var expiresAt time.Time
	if err := tx.QueryRowContext(ctx, `SELECT status::text, expires_at FROM chains WHERE id = $1 FOR UPDATE`, chainID).Scan(&status, &expiresAt); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Chain{}, ErrNotFound
		}
		return Chain{}, fmt.Errorf("lock chain: %w", err)
	}

	var participantStatus string
	if err := tx.QueryRowContext(ctx, `
		SELECT status::text FROM chain_items WHERE chain_id = $1 AND user_id = $2 FOR UPDATE`,
		chainID, userID,
	).Scan(&participantStatus); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return Chain{}, ErrForbidden
		}
		return Chain{}, fmt.Errorf("lock chain participant: %w", err)
	}

	if status != StatusPending {
		chain, err := loadChain(ctx, tx, chainID)
		if err != nil {
			return Chain{}, err
		}
		if err := tx.Commit(); err != nil {
			return Chain{}, fmt.Errorf("commit terminal chain read: %w", err)
		}
		return chain, nil
	}

	if !expiresAt.After(s.now()) {
		return s.rejectAndCommit(ctx, tx, chainID, "expired", 0)
	}
	if decision == DecisionDeclined {
		if _, err := tx.ExecContext(ctx, `
			UPDATE chain_items SET status = 'DECLINED', updated_at = now()
			WHERE chain_id = $1 AND user_id = $2`, chainID, userID); err != nil {
			return Chain{}, fmt.Errorf("decline chain: %w", err)
		}
		return s.rejectAndCommit(ctx, tx, chainID, "declined", userID)
	}

	participantChanged := participantStatus == ParticipantWaiting
	if participantChanged {
		if _, err := tx.ExecContext(ctx, `
			UPDATE chain_items SET status = 'APPROVED', updated_at = now()
			WHERE chain_id = $1 AND user_id = $2`, chainID, userID); err != nil {
			return Chain{}, fmt.Errorf("approve chain: %w", err)
		}
	}

	var waiting int
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM chain_items WHERE chain_id = $1 AND status = 'WAITING'`, chainID,
	).Scan(&waiting); err != nil {
		return Chain{}, fmt.Errorf("count pending participants: %w", err)
	}
	if waiting > 0 {
		if !participantChanged {
			chain, err := loadChain(ctx, tx, chainID)
			if err != nil {
				return Chain{}, err
			}
			if err := tx.Commit(); err != nil {
				return Chain{}, fmt.Errorf("commit idempotent chain decision: %w", err)
			}
			return chain, nil
		}
		return s.commitUpdated(ctx, tx, chainID, "chain.updated", fmt.Sprintf("approved:%d", userID), nil)
	}

	allMatching, err := itemsAreMatching(ctx, tx, itemIDs)
	if err != nil {
		return Chain{}, err
	}
	if !allMatching {
		return s.rejectAndCommit(ctx, tx, chainID, "item_unavailable", 0)
	}

	competing, err := competingRecipients(ctx, tx, chainID, itemIDs)
	if err != nil {
		return Chain{}, err
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE items SET status = 'LOCKED', updated_at = now() WHERE id = ANY($1)`, pq.Array(itemIDs)); err != nil {
		return Chain{}, fmt.Errorf("lock accepted chain items: %w", err)
	}
	if _, err := tx.ExecContext(ctx, `
		UPDATE chains SET status = 'ACCEPTED', updated_at = now() WHERE id = $1`, chainID); err != nil {
		return Chain{}, fmt.Errorf("accept chain: %w", err)
	}
	if len(competing) > 0 {
		competitorIDs := sortedMapKeys(competing)
		if _, err := tx.ExecContext(ctx, `
			UPDATE chains SET status = 'REJECTED', updated_at = now()
			WHERE id = ANY($1) AND status = 'PENDING'`, pq.Array(competitorIDs)); err != nil {
			return Chain{}, fmt.Errorf("reject competing chains: %w", err)
		}
		for _, competingID := range competitorIDs {
			if err := recordChainRejection(ctx, tx, competingID, "item_unavailable", 0, 0); err != nil {
				return Chain{}, err
			}
		}
	}

	chain, err := loadChain(ctx, tx, chainID)
	if err != nil {
		return Chain{}, err
	}
	if err := s.enqueueChainEvent(ctx, tx, chain, "chain.accepted", "accepted", nil); err != nil {
		return Chain{}, err
	}
	for competingID, recipients := range competing {
		if err := s.enqueueToUsers(ctx, tx, competingID, recipients, "chain.rejected", "rejected", map[string]any{"reason": "item_unavailable"}); err != nil {
			return Chain{}, err
		}
	}
	if err := tx.Commit(); err != nil {
		return Chain{}, fmt.Errorf("commit accepted chain: %w", err)
	}
	s.notify(chain, "chain.accepted", nil)
	for competingID, recipients := range competing {
		s.publishUsers(recipients, "chain.rejected", competingID, map[string]any{"reason": "item_unavailable"})
	}
	return chain, nil
}

// ExpirePending rejects up to limit proposals whose deadline has passed.
func (s *PostgresService) ExpirePending(ctx context.Context, limit int) (int, error) {
	if limit < 1 || limit > 1000 {
		return 0, &ValidationError{Message: "expiry limit must be between 1 and 1000"}
	}
	rows, err := s.database.QueryContext(ctx, `
		SELECT id FROM chains
		WHERE status = 'PENDING' AND expires_at <= $1
		ORDER BY id LIMIT $2`, s.now().UTC(), limit)
	if err != nil {
		return 0, fmt.Errorf("find expired chains: %w", err)
	}
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			closeRows(rows)
			return 0, fmt.Errorf("scan expired chain: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		closeRows(rows)
		return 0, fmt.Errorf("iterate expired chains: %w", err)
	}
	if err := rows.Close(); err != nil {
		return 0, fmt.Errorf("close expired chains: %w", err)
	}

	expired := 0
	for _, id := range ids {
		chain, changed, err := s.expireOne(ctx, id)
		if err != nil {
			return expired, err
		}
		if changed {
			expired++
			s.notify(chain, "chain.rejected", map[string]any{"reason": "expired"})
		}
	}
	return expired, nil
}

func (s *PostgresService) expireOne(ctx context.Context, chainID int64) (Chain, bool, error) {
	tx, err := s.database.BeginTx(ctx, nil)
	if err != nil {
		return Chain{}, false, fmt.Errorf("begin expire chain: %w", err)
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx, `
		UPDATE chains SET status = 'REJECTED', updated_at = now()
		WHERE id = $1 AND status = 'PENDING' AND expires_at <= $2`, chainID, s.now().UTC())
	if err != nil {
		return Chain{}, false, fmt.Errorf("expire chain: %w", err)
	}
	changed, err := result.RowsAffected()
	if err != nil {
		return Chain{}, false, fmt.Errorf("read expired chain result: %w", err)
	}
	if changed == 0 {
		return Chain{}, false, nil
	}
	chain, err := loadChain(ctx, tx, chainID)
	if err != nil {
		return Chain{}, false, err
	}
	if err := recordChainRejection(ctx, tx, chainID, "expired", 0, 0); err != nil {
		return Chain{}, false, err
	}
	if err := s.enqueueChainEvent(ctx, tx, chain, "chain.rejected", "rejected", map[string]any{"reason": "expired"}); err != nil {
		return Chain{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return Chain{}, false, fmt.Errorf("commit expired chain: %w", err)
	}
	return chain, true, nil
}

func (s *PostgresService) rejectAndCommit(ctx context.Context, tx *sql.Tx, chainID int64, reason string, actorID int64) (Chain, error) {
	if _, err := tx.ExecContext(ctx, `UPDATE chains SET status = 'REJECTED', updated_at = now() WHERE id = $1`, chainID); err != nil {
		return Chain{}, fmt.Errorf("reject chain: %w", err)
	}
	if err := recordChainRejection(ctx, tx, chainID, reason, actorID, 0); err != nil {
		return Chain{}, err
	}
	return s.commitUpdated(ctx, tx, chainID, "chain.rejected", "rejected", map[string]any{"reason": reason})
}

func recordChainRejection(ctx context.Context, tx *sql.Tx, chainID int64, reason string, actorID, itemID int64) error {
	if _, err := tx.ExecContext(ctx, `
		INSERT INTO chain_rejections (chain_id, reason, actor_user_id, item_id)
		VALUES ($1, $2, NULLIF($3::bigint, 0), NULLIF($4::bigint, 0))
		ON CONFLICT (chain_id) DO NOTHING`, chainID, reason, actorID, itemID); err != nil {
		return fmt.Errorf("record chain %d rejection: %w", chainID, err)
	}
	return nil
}

func (s *PostgresService) commitUpdated(ctx context.Context, tx *sql.Tx, chainID int64, eventType, eventKey string, data map[string]any) (Chain, error) {
	chain, err := loadChain(ctx, tx, chainID)
	if err != nil {
		return Chain{}, err
	}
	if err := s.enqueueChainEvent(ctx, tx, chain, eventType, eventKey, data); err != nil {
		return Chain{}, err
	}
	if err := tx.Commit(); err != nil {
		return Chain{}, fmt.Errorf("commit chain update: %w", err)
	}
	s.notify(chain, eventType, data)
	return chain, nil
}

func (s *PostgresService) enqueueChainEvent(
	ctx context.Context,
	tx *sql.Tx,
	chain Chain,
	eventType, eventKey string,
	data map[string]any,
) error {
	userIDs := make([]int64, 0, len(chain.Participants))
	for _, participant := range chain.Participants {
		userIDs = append(userIDs, participant.User.ID)
	}
	return s.enqueueToUsers(ctx, tx, chain.ID, userIDs, eventType, eventKey, data)
}

func (s *PostgresService) enqueueToUsers(
	ctx context.Context,
	tx *sql.Tx,
	chainID int64,
	userIDs []int64,
	eventType, eventKey string,
	data map[string]any,
) error {
	if s.outbox == nil {
		return nil
	}
	if err := s.outbox.EnqueueTx(ctx, tx, outbox.PendingEvent{
		DeduplicationKey: fmt.Sprintf("chain:%d:%s", chainID, eventKey),
		Type:             eventType,
		EntityID:         strconv.FormatInt(chainID, 10),
		RecipientIDs:     userIDs,
		Data:             data,
	}); err != nil {
		return fmt.Errorf("enqueue %s event for chain %d: %w", eventType, chainID, err)
	}
	return nil
}

func lockItems(ctx context.Context, tx *sql.Tx, itemIDs []int64) error {
	for _, itemID := range itemIDs {
		if _, err := tx.ExecContext(ctx, `SELECT pg_advisory_xact_lock($1)`, itemID); err != nil {
			return fmt.Errorf("lock item %d: %w", itemID, err)
		}
	}
	return nil
}

func loadEligibleItemOwners(ctx context.Context, tx *sql.Tx, itemIDs []int64) (map[int64]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT id, user_id, status::text FROM items
		WHERE id = ANY($1)
		ORDER BY id FOR UPDATE`, pq.Array(itemIDs))
	if err != nil {
		return nil, fmt.Errorf("load proposal items: %w", err)
	}
	defer closeRows(rows)
	owners := make(map[int64]int64, len(itemIDs))
	for rows.Next() {
		var itemID, ownerID int64
		var status string
		if err := rows.Scan(&itemID, &ownerID, &status); err != nil {
			return nil, fmt.Errorf("scan proposal item: %w", err)
		}
		if status != "MATCHING" {
			return nil, ErrConflict
		}
		owners[itemID] = ownerID
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate proposal items: %w", err)
	}
	if len(owners) != len(itemIDs) {
		return nil, ErrNotFound
	}
	seenOwners := make(map[int64]struct{}, len(owners))
	for _, ownerID := range owners {
		if _, exists := seenOwners[ownerID]; exists {
			return nil, &ValidationError{Message: "chain items must belong to different users"}
		}
		seenOwners[ownerID] = struct{}{}
	}
	return owners, nil
}

func loadChainItemIDs(ctx context.Context, tx *sql.Tx, chainID int64) ([]int64, error) {
	rows, err := tx.QueryContext(ctx, `SELECT item_id FROM chain_items WHERE chain_id = $1 ORDER BY item_id`, chainID)
	if err != nil {
		return nil, fmt.Errorf("load chain item IDs: %w", err)
	}
	defer closeRows(rows)
	var ids []int64
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("scan chain item ID: %w", err)
		}
		ids = append(ids, id)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate chain item IDs: %w", err)
	}
	if len(ids) == 0 {
		return nil, ErrNotFound
	}
	return ids, nil
}

func itemsAreMatching(ctx context.Context, tx *sql.Tx, itemIDs []int64) (bool, error) {
	var matching int
	if err := tx.QueryRowContext(ctx, `
		SELECT count(*) FROM items WHERE id = ANY($1) AND status = 'MATCHING'`, pq.Array(itemIDs),
	).Scan(&matching); err != nil {
		return false, fmt.Errorf("check accepted chain items: %w", err)
	}
	return matching == len(itemIDs), nil
}

func competingRecipients(ctx context.Context, tx *sql.Tx, chainID int64, itemIDs []int64) (map[int64][]int64, error) {
	rows, err := tx.QueryContext(ctx, `
		SELECT c.id, participant.user_id
		FROM chains c
		JOIN chain_items participant ON participant.chain_id = c.id
		WHERE c.id <> $1
		  AND c.status = 'PENDING'
		  AND EXISTS (
			SELECT 1 FROM chain_items shared
			WHERE shared.chain_id = c.id AND shared.item_id = ANY($2)
		  )
		ORDER BY c.id, participant.user_id`, chainID, pq.Array(itemIDs))
	if err != nil {
		return nil, fmt.Errorf("load competing chains: %w", err)
	}
	defer closeRows(rows)
	result := make(map[int64][]int64)
	for rows.Next() {
		var competingID, userID int64
		if err := rows.Scan(&competingID, &userID); err != nil {
			return nil, fmt.Errorf("scan competing chain recipient: %w", err)
		}
		result[competingID] = append(result[competingID], userID)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate competing chain recipients: %w", err)
	}
	return result, nil
}

type queryer interface {
	QueryContext(context.Context, string, ...any) (*sql.Rows, error)
}

func loadChain(ctx context.Context, q queryer, chainID int64) (Chain, error) {
	rows, err := q.QueryContext(ctx, `
		SELECT c.id, c.status::text, c.created_at, c.expires_at, c.updated_at,
		       u.id, u.username, ci.status::text,
		       give_item.id, give_item.user_id, give_item.offer_title,
		       COALESCE(give_item.offer_description, ''), COALESCE(give_item.want_description, ''),
		       give_item.image_urls, give_item.status::text, give_item.created_at, give_item.updated_at,
		       receive_item.id, receive_item.user_id, receive_item.offer_title,
		       COALESCE(receive_item.offer_description, ''), COALESCE(receive_item.want_description, ''),
		       receive_item.image_urls, receive_item.status::text, receive_item.created_at, receive_item.updated_at,
		       incoming_delivery.delivery_status::text, incoming_delivery.delivery_updated_at
		FROM chains c
		JOIN chain_items ci ON ci.chain_id = c.id
		JOIN users u ON u.id = ci.user_id
		JOIN items give_item ON give_item.id = ci.item_id
		JOIN items receive_item ON receive_item.id = ci.next_item_id
		LEFT JOIN chain_items AS incoming_delivery
		  ON incoming_delivery.chain_id = c.id
		 AND incoming_delivery.item_id = receive_item.id
		WHERE c.id = $1
		ORDER BY ci.id`, chainID)
	if err != nil {
		return Chain{}, fmt.Errorf("load chain: %w", err)
	}
	defer closeRows(rows)

	var chain Chain
	for rows.Next() {
		var participant Participant
		var giveImages, receiveImages pq.StringArray
		var incomingDeliveryStatus sql.NullString
		var incomingDeliveryUpdatedAt sql.NullTime
		if err := rows.Scan(
			&chain.ID, &chain.Status, &chain.CreatedAt, &chain.ExpiresAt, &chain.UpdatedAt,
			&participant.User.ID, &participant.User.Username, &participant.Status,
			&participant.GiveItem.ID, &participant.GiveItem.UserID, &participant.GiveItem.OfferTitle,
			&participant.GiveItem.OfferDescription, &participant.GiveItem.WantDescription,
			&giveImages, &participant.GiveItem.Status, &participant.GiveItem.CreatedAt, &participant.GiveItem.UpdatedAt,
			&participant.ReceiveItem.ID, &participant.ReceiveItem.UserID, &participant.ReceiveItem.OfferTitle,
			&participant.ReceiveItem.OfferDescription, &participant.ReceiveItem.WantDescription,
			&receiveImages, &participant.ReceiveItem.Status, &participant.ReceiveItem.CreatedAt, &participant.ReceiveItem.UpdatedAt,
			&incomingDeliveryStatus, &incomingDeliveryUpdatedAt,
		); err != nil {
			return Chain{}, fmt.Errorf("scan chain: %w", err)
		}
		if !incomingDeliveryStatus.Valid || !incomingDeliveryUpdatedAt.Valid {
			return Chain{}, fmt.Errorf("load chain %d: participant %d has no incoming delivery", chainID, participant.User.ID)
		}
		participant.IncomingDeliveryStatus = incomingDeliveryStatus.String
		participant.IncomingDeliveryUpdatedAt = incomingDeliveryUpdatedAt.Time
		if participant.IncomingDeliveryStatus == "RECEIVED" {
			participant.ReceiptConfirmed = true
			confirmedAt := participant.IncomingDeliveryUpdatedAt
			participant.ReceiptConfirmedAt = &confirmedAt
		}
		if giveImages == nil {
			participant.GiveItem.ImageURLs = []string{}
		} else {
			participant.GiveItem.ImageURLs = append([]string(nil), giveImages...)
		}
		if receiveImages == nil {
			participant.ReceiveItem.ImageURLs = []string{}
		} else {
			participant.ReceiveItem.ImageURLs = append([]string(nil), receiveImages...)
		}
		chain.Participants = append(chain.Participants, participant)
	}
	if err := rows.Err(); err != nil {
		return Chain{}, fmt.Errorf("iterate chain: %w", err)
	}
	if chain.ID == 0 {
		return Chain{}, ErrNotFound
	}
	return chain, nil
}

func (s *PostgresService) notify(chain Chain, eventType string, data map[string]any) {
	userIDs := make([]int64, 0, len(chain.Participants))
	for _, participant := range chain.Participants {
		userIDs = append(userIDs, participant.User.ID)
	}
	s.publishUsers(userIDs, eventType, chain.ID, data)
}

func (s *PostgresService) publishUsers(userIDs []int64, eventType string, chainID int64, data map[string]any) {
	if s.publish != nil {
		s.publish(userIDs, eventType, strconv.FormatInt(chainID, 10), data)
	}
}

func containsOwner(owners map[int64]int64, userID int64) bool {
	for _, ownerID := range owners {
		if ownerID == userID {
			return true
		}
	}
	return false
}

func hasParticipant(chain Chain, userID int64) bool {
	for _, participant := range chain.Participants {
		if participant.User.ID == userID {
			return true
		}
	}
	return false
}

func sortedMapKeys(values map[int64][]int64) []int64 {
	keys := make([]int64, 0, len(values))
	for key := range values {
		keys = append(keys, key)
	}
	sort.Slice(keys, func(i, j int) bool { return keys[i] < keys[j] })
	return keys
}

func closeRows(rows *sql.Rows) {
	_ = rows.Close()
}
