package model

import (
	"errors"
	"fmt"
	"time"
)

var (
	ErrUserNotFound      = errors.New("reputation user not found")
	ErrChainNotFound     = errors.New("review chain not found")
	ErrChainNotCompleted = errors.New("reviews require a completed chain")
	ErrForbidden         = errors.New("review target is not an exchange neighbour")
	ErrAlreadyExists     = errors.New("review already exists")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

type Stats struct {
	CompletedExchanges int64
	Rating             *float64
	ReviewsCount       int64
}

type Author struct {
	ID       int64
	Username string
}

type Review struct {
	ID           int64
	ChainID      int64
	Author       Author
	TargetUserID int64
	Rating       int
	Text         *string
	CreatedAt    time.Time
}

type CreateInput struct {
	TargetUserID int64
	Rating       int
	Text         *string
}

type ListResult struct {
	Reviews    []Review
	NextCursor *int64
}
