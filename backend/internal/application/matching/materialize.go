package matching

import (
	"context"
	"errors"
	"fmt"

	"swap-chain/internal/chains"
	"swap-chain/internal/items"
	matchingmodel "swap-chain/modules/matching/model"
)

type materializeItemLoader interface {
	Get(ctx context.Context, itemID int64) (items.Item, error)
}

type materializeCycleFinder interface {
	Execute(ctx context.Context, itemID int64) ([][]matchingmodel.Edge, error)
}

type materializeChainCreator interface {
	Create(ctx context.Context, userID int64, input chains.CreateInput) (chains.Chain, error)
}

// Materializer turns ephemeral matching cycles into durable pending proposals.
// It deliberately orchestrates existing application/domain services instead of
// making an internal HTTP request or coupling the analyze module to matching.
type Materializer struct {
	items  materializeItemLoader
	finder materializeCycleFinder
	chains materializeChainCreator
}

func NewMaterializer(
	itemLoader materializeItemLoader,
	finder materializeCycleFinder,
	chainCreator materializeChainCreator,
) (*Materializer, error) {
	switch {
	case itemLoader == nil:
		return nil, fmt.Errorf("matching materializer: item loader is required")
	case finder == nil:
		return nil, fmt.Errorf("matching materializer: cycle finder is required")
	case chainCreator == nil:
		return nil, fmt.Errorf("matching materializer: chain creator is required")
	default:
		return &Materializer{items: itemLoader, finder: finder, chains: chainCreator}, nil
	}
}

// Materialize finds all currently unknown cycles rooted at itemID and persists
// each one. A duplicate/conflicting proposal is safe to skip: cycle_key and the
// chain transaction remain the final concurrency guards.
func (m *Materializer) Materialize(ctx context.Context, itemID int64) (int, error) {
	item, err := m.items.Get(ctx, itemID)
	switch {
	case errors.Is(err, items.ErrNotFound):
		return 0, nil
	case err != nil:
		return 0, fmt.Errorf("load matching job item %d: %w", itemID, err)
	case item.Status != "MATCHING":
		return 0, nil
	}

	cycles, err := m.finder.Execute(ctx, itemID)
	if err != nil {
		return 0, fmt.Errorf("find cycles for matching job item %d: %w", itemID, err)
	}

	created := 0
	for _, cycle := range cycles {
		edges := make([]chains.Edge, 0, len(cycle))
		for _, edge := range cycle {
			edges = append(edges, chains.Edge{
				SourceItemID: edge.SourceID,
				TargetItemID: edge.TargetID,
			})
		}

		if _, err := m.chains.Create(ctx, item.UserID, chains.CreateInput{Edges: edges}); err != nil {
			if errors.Is(err, chains.ErrConflict) {
				continue
			}
			return created, fmt.Errorf("persist matching cycle for item %d: %w", itemID, err)
		}
		created++
	}

	return created, nil
}
