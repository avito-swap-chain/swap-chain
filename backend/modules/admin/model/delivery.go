package model

import (
	"errors"
	"fmt"
	"time"
)

const (
	RoleAdmin = "ADMIN"

	DeliveryAwaitingPVZ = "AWAITING_PVZ"
	DeliveryAtPVZ       = "AT_PVZ"
	DeliveryInTransit   = "IN_DELIVERY"
	DeliveryReceived    = "RECEIVED"

	ChainAccepted  = "ACCEPTED"
	ChainCompleted = "COMPLETED"
)

var (
	ErrForbidden          = errors.New("admin access is required")
	ErrReceiptForbidden   = errors.New("delivery receipt is not available to this user")
	ErrChainNotFound      = errors.New("chain not found")
	ErrDeliveryNotFound   = errors.New("delivery not found")
	ErrTransitionConflict = errors.New("delivery transition conflicts with current state")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

type Delivery struct {
	ID                int64
	ChainID           int64
	ItemID            int64
	ItemTitle         string
	SenderID          int64
	SenderUsername    string
	RecipientID       int64
	RecipientUsername string
	Status            string
	UpdatedAt         time.Time
}

type Receipt struct {
	Delivery    Delivery
	ChainStatus string
}

func ValidateTargetStatus(status string) error {
	switch status {
	case DeliveryAtPVZ, DeliveryInTransit, DeliveryReceived:
		return nil
	default:
		return &ValidationError{Field: "status", Message: "must be AT_PVZ, IN_DELIVERY or RECEIVED"}
	}
}

// CanTransition разрешает только последовательные переходы и идемпотентный повтор команды.
func CanTransition(current, target string) bool {
	if current == target {
		return true
	}
	return current == DeliveryAwaitingPVZ && target == DeliveryAtPVZ ||
		current == DeliveryAtPVZ && target == DeliveryInTransit ||
		current == DeliveryInTransit && target == DeliveryReceived
}
