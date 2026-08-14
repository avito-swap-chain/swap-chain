package model

import (
	"errors"
	"fmt"
	"time"
)

const (
	SenderUser      = "USER"
	SenderModerator = "MODERATOR"
	SenderSystem    = "SYSTEM"
	MaxMessageLen   = 2000
)

var (
	ErrForbidden           = errors.New("support chat is forbidden")
	ErrNotFound            = errors.New("support thread not found")
	ErrNotJoined           = errors.New("moderator is not connected to support thread")
	ErrIdempotencyConflict = errors.New("client message ID was reused with different text")
)

type ValidationError struct{ Field, Message string }

func (e *ValidationError) Error() string { return fmt.Sprintf("invalid %s: %s", e.Field, e.Message) }

type User struct {
	ID       int64
	Username string
}

type Message struct {
	ID              int64
	ThreadID        int64
	SenderType      string
	Sender          *User
	ClientMessageID *string
	Text            string
	CreatedAt       time.Time
}

type Thread struct {
	ID          int64
	User        User
	Moderators  []User
	LastMessage *Message
	UnreadCount int64
	CreatedAt   time.Time
	UpdatedAt   time.Time
}

type ReadState struct{ ThreadID, LastReadMessageID, UnreadCount int64 }
