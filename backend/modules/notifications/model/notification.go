package model

import (
	"fmt"
	"time"
)

const (
	KindChain   = "chain"
	KindMessage = "message"
	KindOffer   = "offer"
)

const (
	maxCursorLimit = 100
)

type Notification struct {
	ID        int64
	UserID    int64
	Kind      string
	Title     string
	Text      string
	TargetURL string
	EntityID  *int64
	IsRead    bool
	CreatedAt time.Time
}

type ListResult struct {
	Items       []Notification
	NextCursor  *int64
	UnreadCount int64
}

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("validation: %s %s", e.Field, e.Message)
}

type ErrNotFound struct {
	UserID int64
}

func (e *ErrNotFound) Error() string {
	return fmt.Sprintf("notification not found for user %d", e.UserID)
}

func ValidateListParams(cursor int64, limit int) error {
	if cursor < 0 {
		return &ValidationError{Field: "cursor", Message: "must be non-negative"}
	}
	if limit < 1 || limit > maxCursorLimit {
		return &ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", maxCursorLimit)}
	}
	return nil
}

func ValidKinds() map[string]bool {
	return map[string]bool{
		KindChain:   true,
		KindMessage: true,
		KindOffer:   true,
	}
}
