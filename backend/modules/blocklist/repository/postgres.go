// Package repository persists the personal blacklist in PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"fmt"
	"time"

	"swap-chain/internal/chains"
	"swap-chain/modules/blocklist/model"
	"swap-chain/shared/db"
)

// chainCanceler reuses the chains service rejection semantics so block-driven
// cancellation never diverges from the standard chain termination side effects.
type chainCanceler interface {
	CancelPendingBetween(ctx context.Context, tx *sql.Tx, userA, userB int64) ([]chains.Chain, error)
	PublishRejections(cancelled []chains.Chain, reason string)
}

// PostgreSQL stores blacklist rows and atomically cancels shared pending chains.
type PostgreSQL struct {
	database *sql.DB
	chains   chainCanceler
}

// NewPostgreSQL constructs a PostgreSQL-backed block-list repository.
func NewPostgreSQL(database *sql.DB, canceler chainCanceler) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("blocklist repository init: database is required")
	}
	if canceler == nil {
		return nil, fmt.Errorf("blocklist repository init: chain canceler is required")
	}
	return &PostgreSQL{database: database, chains: canceler}, nil
}

// Block inserts an idempotent block and atomically cancels every shared
// non-terminal chain in the same transaction.
func (r *PostgreSQL) Block(ctx context.Context, blockerID, blockedID int64) (model.Block, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Block{}, fmt.Errorf("begin block: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockUserPair(ctx, tx, blockerID, blockedID); err != nil {
		return model.Block{}, err
	}

	queries := db.New(tx)
	if err := queries.BlockUser(ctx, db.BlockUserParams{BlockerID: blockerID, BlockedID: blockedID}); err != nil {
		return model.Block{}, fmt.Errorf("insert block: %w", err)
	}

	cancelled, err := r.chains.CancelPendingBetween(ctx, tx, blockerID, blockedID)
	if err != nil {
		return model.Block{}, err
	}

	row, err := queries.GetUserBlock(ctx, db.GetUserBlockParams{BlockerID: blockerID, BlockedID: blockedID})
	if err != nil {
		return model.Block{}, fmt.Errorf("load block: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.Block{}, fmt.Errorf("commit block: %w", err)
	}

	r.chains.PublishRejections(cancelled, "blocked")
	return blockFrom(row.BlockID, row.BlockedUserID, row.BlockedUsername, row.CreatedAt), nil
}

// Unblock removes a directed block. A never-blocked pair is a no-op; a missing
// target user is reported as not found.
func (r *PostgreSQL) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin unblock: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	if err := lockUserPair(ctx, tx, blockerID, blockedID); err != nil {
		return err
	}
	queries := db.New(tx)
	deleted, err := queries.UnblockUser(ctx, db.UnblockUserParams{BlockerID: blockerID, BlockedID: blockedID})
	if err != nil {
		return fmt.Errorf("unblock user: %w", err)
	}
	if deleted > 0 {
		if _, err := tx.ExecContext(ctx, `
			INSERT INTO matching_jobs (item_id, status, attempts, available_at, locked_at, last_error, updated_at)
			SELECT id, 'PENDING', 0, now(), NULL, NULL, now()
			FROM items
			WHERE user_id IN ($1, $2) AND status = 'MATCHING'
			ON CONFLICT (item_id) DO UPDATE
			SET status='PENDING', attempts=0, available_at=now(), locked_at=NULL,
			    last_error=NULL, updated_at=now()`, blockerID, blockedID); err != nil {
			return fmt.Errorf("schedule matching after unblock: %w", err)
		}
	}
	if err := tx.Commit(); err != nil {
		return fmt.Errorf("commit unblock: %w", err)
	}
	return nil
}

// List returns the actor's blocks ordered by ascending block ID with a cursor.
func (r *PostgreSQL) List(ctx context.Context, blockerID, afterID int64, limit int) ([]model.Block, *int64, error) {
	rows, err := db.New(r.database).ListUserBlocks(ctx, db.ListUserBlocksParams{
		BlockerID: blockerID,
		AfterID:   afterID,
		PageSize:  int32(limit + 1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list blocks: %w", err)
	}

	blocks := make([]model.Block, 0, len(rows))
	for _, row := range rows {
		blocks = append(blocks, blockFrom(row.BlockID, row.BlockedUserID, row.BlockedUsername, row.CreatedAt))
	}

	var next *int64
	if len(blocks) > limit {
		cursor := blocks[limit-1].ID
		next = &cursor
		blocks = blocks[:limit]
	}
	return blocks, next, nil
}

// lockUserPair locks both user rows in a stable order and verifies they exist.
func lockUserPair(ctx context.Context, tx *sql.Tx, blockerID, blockedID int64) error {
	rows, err := tx.QueryContext(ctx, `
		SELECT id FROM users WHERE id IN ($1, $2) ORDER BY id FOR UPDATE`, blockerID, blockedID)
	if err != nil {
		return fmt.Errorf("lock user pair: %w", err)
	}
	defer func() { _ = rows.Close() }()

	found := make(map[int64]struct{}, 2)
	for rows.Next() {
		var id int64
		if err := rows.Scan(&id); err != nil {
			return fmt.Errorf("scan locked user pair: %w", err)
		}
		found[id] = struct{}{}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate locked user pair: %w", err)
	}
	if _, ok := found[blockerID]; !ok {
		return model.ErrForbidden
	}
	if _, ok := found[blockedID]; !ok {
		return model.ErrTargetNotFound
	}
	return nil
}

func blockFrom(blockID, blockedUserID int64, blockedUsername string, createdAt time.Time) model.Block {
	return model.Block{
		ID: blockID,
		BlockedUser: model.BlockedUser{
			ID:       blockedUserID,
			Username: blockedUsername,
		},
		BlockedAt: createdAt,
	}
}
