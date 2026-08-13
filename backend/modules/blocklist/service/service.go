// Package service implements the personal blacklist use cases.
package service

import (
	"context"
	"fmt"

	"swap-chain/modules/blocklist/model"
)

// Repository persists and queries the actor's personal blacklist.
type Repository interface {
	Block(ctx context.Context, blockerID, blockedID int64) (model.Block, error)
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	List(ctx context.Context, blockerID, afterID int64, limit int) ([]model.Block, *int64, error)
}

// Service defines the block-list operations used by the HTTP layer.
type Service interface {
	Block(ctx context.Context, blockerID, blockedID int64) (model.Block, error)
	Unblock(ctx context.Context, blockerID, blockedID int64) error
	List(ctx context.Context, blockerID, afterID int64, limit int) ([]model.Block, *int64, error)
}

// BlocklistConfig bounds list pagination.
type BlocklistConfig struct {
	MaxListLimit int
}

// Blocklist coordinates blacklist commands with the repository.
type Blocklist struct {
	repository Repository
	cfg        BlocklistConfig
}

// New constructs a block-list service.
func New(repository Repository, cfg BlocklistConfig) (*Blocklist, error) {
	if repository == nil {
		return nil, fmt.Errorf("blocklist init: repository is required")
	}
	return &Blocklist{repository: repository, cfg: cfg}, nil
}

// Block validates and applies a directed block. Repeating a block is idempotent
// but still re-checks and cancels shared non-terminal chains.
func (s *Blocklist) Block(ctx context.Context, blockerID, blockedID int64) (model.Block, error) {
	if blockerID <= 0 {
		return model.Block{}, model.ErrForbidden
	}
	if blockedID <= 0 {
		return model.Block{}, &model.ValidationError{Field: "blockedUserId", Message: "must be positive"}
	}
	if blockerID == blockedID {
		return model.Block{}, model.ErrSelfBlock
	}
	return s.repository.Block(ctx, blockerID, blockedID)
}

// Unblock validates and removes a directed block. Unblocking never revives
// previously cancelled chains.
func (s *Blocklist) Unblock(ctx context.Context, blockerID, blockedID int64) error {
	if blockerID <= 0 {
		return model.ErrForbidden
	}
	if blockedID <= 0 {
		return &model.ValidationError{Field: "blockedUserId", Message: "must be positive"}
	}
	return s.repository.Unblock(ctx, blockerID, blockedID)
}

// List returns a cursor-paginated view of the actor's own blocks.
func (s *Blocklist) List(ctx context.Context, blockerID, afterID int64, limit int) ([]model.Block, *int64, error) {
	if blockerID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > s.cfg.MaxListLimit {
		return nil, nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.cfg.MaxListLimit)}
	}
	if afterID < 0 {
		return nil, nil, &model.ValidationError{Field: "cursor", Message: "must be a non-negative block ID"}
	}
	return s.repository.List(ctx, blockerID, afterID, limit)
}
