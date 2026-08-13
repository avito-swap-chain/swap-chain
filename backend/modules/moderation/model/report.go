// Package model defines the message-report and moderation-queue domain.
package model

import (
	"errors"
	"fmt"
	"time"

	chatmodel "swap-chain/modules/chat/model"
)

const (
	// Report statuses.
	ReportOpen     = "open"
	ReportResolved = "resolved"
	ReportRejected = "rejected"

	// Report reasons.
	ReasonSpam  = "spam"
	ReasonAbuse = "abuse"
	ReasonOther = "other"

	// Terminal decisions.
	DecisionResolved = "resolved"
	DecisionRejected = "rejected"

	// MaxCommentLength bounds the report and decision comment.
	MaxCommentLength = 1000
)

var (
	// ErrForbidden indicates that the current user lacks moderation access.
	ErrForbidden = errors.New("moderation access is required")
	// ErrNotFound indicates that the report or message does not exist.
	ErrNotFound = errors.New("report not found")
	// ErrSelfReport indicates that a user tried to report their own message.
	ErrSelfReport = errors.New("cannot report your own message")
	// ErrReportUnavailable indicates the reported message is not accessible to the user.
	ErrReportUnavailable = errors.New("message is not available to this user")
	// ErrStateConflict indicates the report cannot change in its current state.
	ErrStateConflict = errors.New("report cannot change in its current state")
	// ErrAlreadyAssigned indicates the report is assigned to another administrator.
	ErrAlreadyAssigned = errors.New("report is already assigned to another administrator")
)

// ValidationError describes an invalid moderation command.
type ValidationError struct {
	Field   string
	Message string
}

func (e *ValidationError) Error() string {
	return fmt.Sprintf("invalid %s: %s", e.Field, e.Message)
}

// User is the transport-independent identity used by moderation results.
type User struct {
	ID       int64
	Username string
}

// Report is a single message-report aggregate.
type Report struct {
	ID              int64
	Reporter        User
	MessageID       int64
	Reason          string
	Comment         *string
	Status          string
	Assignee        *User
	DecisionComment *string
	CreatedAt       time.Time
	UpdatedAt       time.Time
}

// ReportDetail is the admin view of one report with the reported message and
// the full thread context in deterministic (ascending message ID) order.
type ReportDetail struct {
	Report          Report
	ReportedMessage chatmodel.Message
	Context         []chatmodel.Message
}

// ReportFilter narrows the moderation queue.
type ReportFilter struct {
	Status     string
	Reason     string
	AssigneeID int64
	Unassigned bool
}
