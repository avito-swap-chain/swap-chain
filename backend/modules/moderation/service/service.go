// Package service implements message reports, the moderation queue and the audit log.
package service

import (
	"context"
	"fmt"
	"strings"

	"swap-chain/modules/moderation/model"
)

// Repository persists and queries reports, assignments and audit entries.
type Repository interface {
	CreateReport(ctx context.Context, reporterID, messageID int64, reason string, comment *string) (model.Report, bool, error)
	CreateUserReport(ctx context.Context, reporterID, targetID int64, chainID *int64, reason string, comment *string) (model.UserReport, bool, error)
	ListReports(ctx context.Context, adminID int64, filter model.ReportFilter, afterID int64, limit int) ([]model.Report, *int64, error)
	GetReport(ctx context.Context, adminID, reportID int64) (model.ReportDetail, error)
	Assign(ctx context.Context, adminID, reportID int64) (model.Report, error)
	Decide(ctx context.Context, adminID, reportID int64, decision, comment string) (model.Report, error)
	ListAudit(ctx context.Context, adminID int64, filter model.AuditFilter, beforeID *int64, limit int) ([]model.AuditEntry, *int64, error)
}

// Service defines the moderation operations used by the HTTP layer.
type Service interface {
	CreateReport(ctx context.Context, reporterID, messageID int64, reason, comment string) (model.Report, bool, error)
	CreateUserReport(ctx context.Context, reporterID, targetID int64, chainID *int64, reason, comment string) (model.UserReport, bool, error)
	ListReports(ctx context.Context, adminID int64, filter model.ReportFilter, afterID int64, limit int) ([]model.Report, *int64, error)
	GetReport(ctx context.Context, adminID, reportID int64) (model.ReportDetail, error)
	Assign(ctx context.Context, adminID, reportID int64) (model.Report, error)
	Decide(ctx context.Context, adminID, reportID int64, decision, comment string) (model.Report, error)
	ListAudit(ctx context.Context, adminID int64, filter model.AuditFilter, beforeID *int64, limit int) ([]model.AuditEntry, *int64, error)
}

// ModerationConfig bounds queue and audit pagination.
type ModerationConfig struct {
	MaxListLimit int
}

// Moderation coordinates report and queue commands with the repository.
type Moderation struct {
	repository Repository
	cfg        ModerationConfig
}

// New constructs a moderation service.
func New(repository Repository, cfg ModerationConfig) (*Moderation, error) {
	if repository == nil {
		return nil, fmt.Errorf("moderation init: repository is required")
	}
	return &Moderation{repository: repository, cfg: cfg}, nil
}

// CreateUserReport validates and stores a complaint about another user.
func (s *Moderation) CreateUserReport(ctx context.Context, reporterID, targetID int64, chainID *int64, reason, comment string) (model.UserReport, bool, error) {
	if reporterID <= 0 {
		return model.UserReport{}, false, model.ErrForbidden
	}
	if targetID <= 0 {
		return model.UserReport{}, false, &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	if reporterID == targetID {
		return model.UserReport{}, false, model.ErrSelfReport
	}
	if chainID != nil && *chainID <= 0 {
		return model.UserReport{}, false, &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	if !validUserReportReason(reason) {
		return model.UserReport{}, false, &model.ValidationError{Field: "reason", Message: "must be item, noshow, rude, fraud or other"}
	}
	normalizedComment, err := normalizeComment(reason, comment)
	if err != nil {
		return model.UserReport{}, false, err
	}
	return s.repository.CreateUserReport(ctx, reporterID, targetID, chainID, reason, normalizedComment)
}

// CreateReport validates a report command and creates or returns the idempotent report.
func (s *Moderation) CreateReport(ctx context.Context, reporterID, messageID int64, reason, comment string) (model.Report, bool, error) {
	if reporterID <= 0 {
		return model.Report{}, false, model.ErrForbidden
	}
	if messageID <= 0 {
		return model.Report{}, false, &model.ValidationError{Field: "messageId", Message: "must be positive"}
	}
	if !validReason(reason) {
		return model.Report{}, false, &model.ValidationError{Field: "reason", Message: "must be spam, abuse or other"}
	}
	normalizedComment, err := normalizeComment(reason, comment)
	if err != nil {
		return model.Report{}, false, err
	}
	return s.repository.CreateReport(ctx, reporterID, messageID, reason, normalizedComment)
}

// ListReports validates queue filters and returns a cursor-paginated page.
func (s *Moderation) ListReports(ctx context.Context, adminID int64, filter model.ReportFilter, afterID int64, limit int) ([]model.Report, *int64, error) {
	if adminID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > s.cfg.MaxListLimit {
		return nil, nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.cfg.MaxListLimit)}
	}
	if afterID < 0 {
		return nil, nil, &model.ValidationError{Field: "cursor", Message: "must be a non-negative report ID"}
	}
	if filter.Status != "" && !validStatus(filter.Status) {
		return nil, nil, &model.ValidationError{Field: "status", Message: "unsupported report status"}
	}
	if filter.Reason != "" && !validReason(filter.Reason) {
		return nil, nil, &model.ValidationError{Field: "reason", Message: "unsupported report reason"}
	}
	if filter.Unassigned && filter.AssigneeID != 0 {
		return nil, nil, &model.ValidationError{Field: "assigneeId", Message: "cannot be combined with unassigned"}
	}
	return s.repository.ListReports(ctx, adminID, filter, afterID, limit)
}

// GetReport returns the admin detail of one report.
func (s *Moderation) GetReport(ctx context.Context, adminID, reportID int64) (model.ReportDetail, error) {
	if adminID <= 0 {
		return model.ReportDetail{}, model.ErrForbidden
	}
	if reportID <= 0 {
		return model.ReportDetail{}, &model.ValidationError{Field: "reportId", Message: "must be positive"}
	}
	return s.repository.GetReport(ctx, adminID, reportID)
}

// Assign assigns an open report to the current administrator.
func (s *Moderation) Assign(ctx context.Context, adminID, reportID int64) (model.Report, error) {
	if adminID <= 0 {
		return model.Report{}, model.ErrForbidden
	}
	if reportID <= 0 {
		return model.Report{}, &model.ValidationError{Field: "reportId", Message: "must be positive"}
	}
	return s.repository.Assign(ctx, adminID, reportID)
}

// Decide applies a single terminal decision to an assigned open report.
func (s *Moderation) Decide(ctx context.Context, adminID, reportID int64, decision, comment string) (model.Report, error) {
	if adminID <= 0 {
		return model.Report{}, model.ErrForbidden
	}
	if reportID <= 0 {
		return model.Report{}, &model.ValidationError{Field: "reportId", Message: "must be positive"}
	}
	if !validDecision(decision) {
		return model.Report{}, &model.ValidationError{Field: "decision", Message: "must be resolved or rejected"}
	}
	comment = strings.TrimSpace(comment)
	if comment == "" {
		return model.Report{}, &model.ValidationError{Field: "comment", Message: "must not be empty"}
	}
	if len([]rune(comment)) > model.MaxCommentLength {
		return model.Report{}, &model.ValidationError{Field: "comment", Message: fmt.Sprintf("must contain at most %d characters", model.MaxCommentLength)}
	}
	return s.repository.Decide(ctx, adminID, reportID, decision, comment)
}

// ListAudit validates audit filters and returns the newest-first cursor page.
func (s *Moderation) ListAudit(ctx context.Context, adminID int64, filter model.AuditFilter, beforeID *int64, limit int) ([]model.AuditEntry, *int64, error) {
	if adminID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > s.cfg.MaxListLimit {
		return nil, nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.cfg.MaxListLimit)}
	}
	if filter.Action != "" && !validAction(filter.Action) {
		return nil, nil, &model.ValidationError{Field: "action", Message: "unsupported audit action"}
	}
	return s.repository.ListAudit(ctx, adminID, filter, beforeID, limit)
}

func normalizeComment(reason, comment string) (*string, error) {
	comment = strings.TrimSpace(comment)
	switch {
	case reason == model.ReasonOther && comment == "":
		return nil, &model.ValidationError{Field: "comment", Message: "must be provided for reason other"}
	case comment == "":
		return nil, nil
	case len([]rune(comment)) > model.MaxCommentLength:
		return nil, &model.ValidationError{Field: "comment", Message: fmt.Sprintf("must contain at most %d characters", model.MaxCommentLength)}
	default:
		return &comment, nil
	}
}

func validReason(reason string) bool {
	return reason == model.ReasonSpam || reason == model.ReasonAbuse || reason == model.ReasonOther
}

func validUserReportReason(reason string) bool {
	switch reason {
	case model.ReasonItem, model.ReasonNoShow, model.ReasonRude, model.ReasonFraud, model.ReasonOther:
		return true
	default:
		return false
	}
}

func validStatus(status string) bool {
	return status == model.ReportOpen || status == model.ReportResolved || status == model.ReportRejected
}

func validDecision(decision string) bool {
	return decision == model.DecisionResolved || decision == model.DecisionRejected
}

func validAction(action string) bool {
	switch action {
	case model.ActionReportAssigned,
		model.ActionReportResolved,
		model.ActionReportRejected,
		model.ActionUserBlocked,
		model.ActionChainCancelled,
		model.ActionDeliveryStatusChanged:
		return true
	default:
		return false
	}
}
