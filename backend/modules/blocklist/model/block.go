// Package model defines the personal blacklist domain.
package model

import (
	"errors"
	"fmt"
	"time"
)

var (
	// ErrForbidden indicates that the current user cannot access the block list.
	ErrForbidden = errors.New("block list is not available to this user")
	// ErrSelfBlock indicates that a user tried to block themselves.
	ErrSelfBlock = errors.New("cannot block yourself")
	// ErrTargetNotFound indicates that the user to block/unblock does not exist.
	ErrTargetNotFound = errors.New("target user not found")
)

// ValidationError describes an invalid block-list command.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// BlockedUser is the transport-independent identity of a blocked user.
type BlockedUser struct {
	ID       int64
	Username string
}

// Block is a single directed blacklist entry of the current actor.
type Block struct {
	ID          int64
	BlockedUser BlockedUser
	BlockedAt   time.Time
}
