// Package matching contains application use cases for the matching engine.
package matching

import (
	"context"
	"errors"
	"fmt"

	"swap-chain/internal/cyclekey"
	"swap-chain/modules/matching/model"
)

// ErrInvalidItemID reports an invalid item identifier at the application boundary.
var ErrInvalidItemID = errors.New("item id must be positive")

type cycleMatcher interface {
	FindCycles(ctx context.Context, itemID int64) ([][]model.Edge, error)
}

type cycleRegistry interface {
	KnownCycleKeys(ctx context.Context, keys []string) (map[string]struct{}, error)
}

// FindCycles executes matching for one existing item.
type FindCycles struct {
	matcher  cycleMatcher
	registry cycleRegistry
}

// NewFindCycles constructs the matching use case.
func NewFindCycles(matcher cycleMatcher, registry cycleRegistry) *FindCycles {
	return &FindCycles{matcher: matcher, registry: registry}
}

// Execute validates the item identifier and delegates cycle discovery.
func (useCase *FindCycles) Execute(ctx context.Context, itemID int64) ([][]model.Edge, error) {
	if itemID <= 0 {
		return nil, ErrInvalidItemID
	}
	cycles, err := useCase.matcher.FindCycles(ctx, itemID)
	if err != nil {
		return nil, err
	}

	keys := make([]string, 0, len(cycles))
	cyclesByKey := make(map[string][]model.Edge, len(cycles))
	for _, cycle := range cycles {
		edges := make([]cyclekey.Edge, 0, len(cycle))
		for _, edge := range cycle {
			edges = append(edges, cyclekey.Edge{SourceID: int64(edge.SourceID), TargetID: int64(edge.TargetID)})
		}
		key, keyErr := cyclekey.Canonical(edges)
		if keyErr != nil {
			return nil, fmt.Errorf("canonicalize matching cycle: %w", keyErr)
		}
		if _, duplicate := cyclesByKey[key]; duplicate {
			continue
		}
		keys = append(keys, key)
		cyclesByKey[key] = cycle
	}

	known, err := useCase.registry.KnownCycleKeys(ctx, keys)
	if err != nil {
		return nil, fmt.Errorf("filter persisted matching cycles: %w", err)
	}
	result := make([][]model.Edge, 0, len(keys))
	for _, key := range keys {
		if _, exists := known[key]; !exists {
			result = append(result, cyclesByKey[key])
		}
	}
	return result, nil
}
