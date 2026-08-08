// Package matching contains application use cases for the matching engine.
package matching

import (
	"context"
	"errors"

	"swap-chain/matching/model"
)

// ErrInvalidItemID reports an invalid item identifier at the application boundary.
var ErrInvalidItemID = errors.New("item id must be positive")

type cycleMatcher interface {
	FindCycles(ctx context.Context, itemID int) ([][]model.Edge, error)
}

// FindCycles executes matching for one existing item.
type FindCycles struct {
	matcher cycleMatcher
}

// NewFindCycles constructs the matching use case.
func NewFindCycles(matcher cycleMatcher) *FindCycles {
	return &FindCycles{matcher: matcher}
}

// Execute validates the item identifier and delegates cycle discovery.
func (useCase *FindCycles) Execute(ctx context.Context, itemID int) ([][]model.Edge, error) {
	if itemID <= 0 {
		return nil, ErrInvalidItemID
	}
	return useCase.matcher.FindCycles(ctx, itemID)
}
