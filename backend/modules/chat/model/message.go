package model

import (
	"errors"
	"fmt"
	"time"
)

const MaxMessageLength = 2000

var (
	ErrItemNotFound        = errors.New("chat item not found")
	ErrThreadNotFound      = errors.New("chat thread not found")
	ErrMessageNotFound     = errors.New("chat message not found")
	ErrForbidden           = errors.New("chat is not available to this user")
	ErrIdempotencyConflict = errors.New("client message ID was already used with different text")
)

type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

type Sender struct {
	ID       int64
	Username string
}

type Message struct {
	ID              int64
	ItemID          int64
	OriginChainID   int64
	Sender          Sender
	Recipient       Sender
	ClientMessageID string
	Text            string
	CreatedAt       time.Time
}

type ItemSummary struct {
	ID       int64
	Title    string
	ImageURL string
}

type Thread struct {
	Item        ItemSummary
	Counterpart Sender
	LastMessage *Message
	UnreadCount int64
}

type ReadState struct {
	ItemID            int64
	CounterpartID     int64
	LastReadMessageID int64
	UnreadCount       int64
}
