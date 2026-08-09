// Package chains implements the exchange-chain lifecycle.
package chains

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"

	"swap-chain/internal/cyclekey"
	"swap-chain/internal/items"
)

// Chain, participant and decision statuses used by the MVP lifecycle.
const (
	StatusPending  = "PENDING"
	StatusAccepted = "ACCEPTED"
	StatusRejected = "REJECTED"

	ParticipantWaiting  = "WAITING"
	ParticipantApproved = "APPROVED"
	ParticipantDeclined = "DECLINED"

	DecisionApproved = "APPROVED"
	DecisionDeclined = "DECLINED"
)

var (
	// ErrNotFound indicates that a chain does not exist.
	ErrNotFound = errors.New("chain not found")
	// ErrForbidden indicates that the current user is not a chain participant.
	ErrForbidden = errors.New("chain is not available to this user")
	// ErrConflict indicates that current item or chain state prevents the operation.
	ErrConflict = errors.New("chain cannot be changed in its current state")
)

// ValidationError describes an invalid chain command.
type ValidationError struct {
	Message string
}

func (e *ValidationError) Error() string {
	return e.Message
}

// Edge maps the item a participant gives to the item they receive.
type Edge struct {
	SourceItemID int64
	TargetItemID int64
}

// CreateInput contains a closed matching cycle selected by its initiator.
type CreateInput struct {
	Edges []Edge
}

// User is the participant identity exposed by the chain service.
type User struct {
	ID       int64
	Username string
}

// Participant contains one user's direction and decision in a chain.
type Participant struct {
	User        User
	GiveItem    items.Item
	ReceiveItem items.Item
	Status      string
}

// Chain is a complete exchange proposal aggregate.
type Chain struct {
	ID           int64
	Status       string
	Participants []Participant
	CreatedAt    time.Time
	ExpiresAt    time.Time
	UpdatedAt    time.Time
}

// Service defines chain operations used by the HTTP layer.
type Service interface {
	Create(ctx context.Context, userID int64, input CreateInput) (Chain, error)
	List(ctx context.Context, userID int64, status string, afterID int64, limit int) ([]Chain, *int64, error)
	Get(ctx context.Context, userID, chainID int64) (Chain, error)
	Decide(ctx context.Context, userID, chainID int64, decision string) (Chain, error)
	ExpirePending(ctx context.Context, limit int) (int, error)
}

func validateCreate(input CreateInput) ([]int64, string, error) {
	if len(input.Edges) < 2 || len(input.Edges) > 3 {
		return nil, "", &ValidationError{Message: "chain must contain two or three edges"}
	}

	targets := make(map[int64]struct{}, len(input.Edges))
	sources := make(map[int64]struct{}, len(input.Edges))
	for _, edge := range input.Edges {
		if edge.SourceItemID <= 0 || edge.TargetItemID <= 0 || edge.SourceItemID == edge.TargetItemID {
			return nil, "", &ValidationError{Message: "chain edges must reference different positive item IDs"}
		}
		if _, exists := sources[edge.SourceItemID]; exists {
			return nil, "", &ValidationError{Message: "each source item must occur once"}
		}
		if _, exists := targets[edge.TargetItemID]; exists {
			return nil, "", &ValidationError{Message: "each target item must occur once"}
		}
		sources[edge.SourceItemID] = struct{}{}
		targets[edge.TargetItemID] = struct{}{}
	}
	for itemID := range sources {
		if _, exists := targets[itemID]; !exists {
			return nil, "", &ValidationError{Message: "edges must form one closed cycle"}
		}
	}
	canonicalEdges := make([]cyclekey.Edge, 0, len(input.Edges))
	for _, edge := range input.Edges {
		canonicalEdges = append(canonicalEdges, cyclekey.Edge{SourceID: edge.SourceItemID, TargetID: edge.TargetItemID})
	}
	key, err := cyclekey.Canonical(canonicalEdges)
	if err != nil {
		return nil, "", &ValidationError{Message: "edges must form one closed cycle"}
	}

	itemIDs := make([]int64, 0, len(sources))
	for itemID := range sources {
		itemIDs = append(itemIDs, itemID)
	}
	sort.Slice(itemIDs, func(i, j int) bool { return itemIDs[i] < itemIDs[j] })
	return itemIDs, key, nil
}

func validateDecision(decision string) error {
	if decision != DecisionApproved && decision != DecisionDeclined {
		return &ValidationError{Message: fmt.Sprintf("unsupported chain decision %q", decision)}
	}
	return nil
}
