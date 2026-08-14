// Package repository persists message reports, the moderation queue and the
// administrator audit log in PostgreSQL.
package repository

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	chatmodel "swap-chain/modules/chat/model"
	"swap-chain/modules/moderation/model"
	"swap-chain/shared/db"
)

// PostgreSQL stores moderation state and audit records.
type PostgreSQL struct {
	database *sql.DB
}

// NewPostgreSQL constructs a PostgreSQL-backed moderation repository.
func NewPostgreSQL(database *sql.DB) (*PostgreSQL, error) {
	if database == nil {
		return nil, fmt.Errorf("moderation repository init: database is required")
	}
	return &PostgreSQL{database: database}, nil
}

// CreateReport validates message access and creates an idempotent report.
func (r *PostgreSQL) CreateReport(ctx context.Context, reporterID, messageID int64, reason string, comment *string) (model.Report, bool, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Report{}, false, fmt.Errorf("begin create report: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	message, err := queries.GetChatMessage(ctx, messageID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Report{}, false, model.ErrNotFound
	case err != nil:
		return model.Report{}, false, fmt.Errorf("load reported message: %w", err)
	}
	if message.SenderUserID == reporterID {
		return model.Report{}, false, model.ErrSelfReport
	}
	if message.RecipientUserID != reporterID {
		return model.Report{}, false, model.ErrReportUnavailable
	}

	var commentArg sql.NullString
	if comment != nil {
		commentArg = sql.NullString{String: *comment, Valid: true}
	}

	reportID, err := queries.InsertMessageReport(ctx, db.InsertMessageReportParams{
		ReporterID: reporterID,
		MessageID:  messageID,
		Reason:     db.ReportReason(reason),
		Comment:    commentArg,
	})
	created := err == nil
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return model.Report{}, false, fmt.Errorf("insert report: %w", err)
	}

	var report model.Report
	if created {
		report, err = loadReportByID(ctx, queries, reportID)
	} else {
		report, err = loadReportByReporterMessage(ctx, queries, reporterID, messageID)
	}
	if err != nil {
		return model.Report{}, false, err
	}
	if err := tx.Commit(); err != nil {
		return model.Report{}, false, fmt.Errorf("commit create report: %w", err)
	}
	return report, created, nil
}

// CreateUserReport validates the optional chain relationship and stores an idempotent complaint.
func (r *PostgreSQL) CreateUserReport(ctx context.Context, reporterID, targetID int64, chainID *int64, reason string, comment *string) (model.UserReport, bool, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.UserReport{}, false, fmt.Errorf("begin create user report: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	var targetExists bool
	if err := tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM users WHERE id = $1)`, targetID).Scan(&targetExists); err != nil {
		return model.UserReport{}, false, fmt.Errorf("check reported user: %w", err)
	}
	if !targetExists {
		return model.UserReport{}, false, model.ErrNotFound
	}
	if chainID != nil {
		var participantCount int
		if err := tx.QueryRowContext(ctx, `
			SELECT count(DISTINCT user_id)
			FROM chain_items
			WHERE chain_id = $1 AND user_id IN ($2, $3)
		`, *chainID, reporterID, targetID).Scan(&participantCount); err != nil {
			return model.UserReport{}, false, fmt.Errorf("check report chain: %w", err)
		}
		if participantCount != 2 {
			return model.UserReport{}, false, model.ErrUserReportUnavailable
		}
	}

	var chainArg, commentArg any
	if chainID != nil {
		chainArg = *chainID
	}
	if comment != nil {
		commentArg = *comment
	}
	row := tx.QueryRowContext(ctx, `
		INSERT INTO user_reports (reporter_user_id, target_user_id, chain_id, reason, comment)
		VALUES ($1, $2, $3, $4, $5)
		ON CONFLICT (reporter_user_id, target_user_id, chain_id) DO NOTHING
		RETURNING id, reporter_user_id, target_user_id, chain_id, reason, comment, created_at
	`, reporterID, targetID, chainArg, reason, commentArg)
	report, err := scanUserReport(row)
	created := err == nil
	if errors.Is(err, sql.ErrNoRows) {
		report, err = scanUserReport(tx.QueryRowContext(ctx, `
			SELECT id, reporter_user_id, target_user_id, chain_id, reason, comment, created_at
			FROM user_reports
			WHERE reporter_user_id = $1 AND target_user_id = $2 AND chain_id IS NOT DISTINCT FROM $3
		`, reporterID, targetID, chainArg))
	}
	if err != nil {
		return model.UserReport{}, false, fmt.Errorf("store user report: %w", err)
	}
	if err := tx.Commit(); err != nil {
		return model.UserReport{}, false, fmt.Errorf("commit create user report: %w", err)
	}
	return report, created, nil
}

type rowScanner interface {
	Scan(dest ...any) error
}

func scanUserReport(row rowScanner) (model.UserReport, error) {
	var report model.UserReport
	var chainID sql.NullInt64
	var comment sql.NullString
	if err := row.Scan(&report.ID, &report.ReporterID, &report.TargetID, &chainID, &report.Reason, &comment, &report.CreatedAt); err != nil {
		return model.UserReport{}, err
	}
	if chainID.Valid {
		report.ChainID = &chainID.Int64
	}
	if comment.Valid {
		report.Comment = &comment.String
	}
	return report, nil
}

// ListReports returns a cursor-paginated moderation queue page.
func (r *PostgreSQL) ListReports(ctx context.Context, adminID int64, filter model.ReportFilter, afterID int64, limit int) ([]model.Report, *int64, error) {
	queries := db.New(r.database)
	if err := requireAdmin(ctx, queries, adminID); err != nil {
		return nil, nil, err
	}

	rows, err := queries.ListMessageReports(ctx, db.ListMessageReportsParams{
		AfterID:    afterID,
		Status:     filter.Status,
		Reason:     filter.Reason,
		Unassigned: filter.Unassigned,
		AssigneeID: filter.AssigneeID,
		PageSize:   int32(limit + 1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list reports: %w", err)
	}

	reports := make([]model.Report, 0, len(rows))
	for _, row := range rows {
		reports = append(reports, mapReport(row.ID, row.ReporterUserID, row.ReporterUsername, row.MessageID, row.Reason, row.Comment, row.Status, row.AssigneeUserID, row.AssigneeUsername, row.DecisionComment, row.CreatedAt, row.UpdatedAt))
	}

	var next *int64
	if len(reports) > limit {
		cursor := reports[limit-1].ID
		next = &cursor
		reports = reports[:limit]
	}
	return reports, next, nil
}

// GetReport returns the admin detail of one report with thread context.
func (r *PostgreSQL) GetReport(ctx context.Context, adminID, reportID int64) (model.ReportDetail, error) {
	queries := db.New(r.database)
	if err := requireAdmin(ctx, queries, adminID); err != nil {
		return model.ReportDetail{}, err
	}

	row, err := queries.GetModerationReport(ctx, reportID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.ReportDetail{}, model.ErrNotFound
	case err != nil:
		return model.ReportDetail{}, fmt.Errorf("load report: %w", err)
	}
	report := mapReport(row.ID, row.ReporterUserID, row.ReporterUsername, row.MessageID, row.Reason, row.Comment, row.Status, row.AssigneeUserID, row.AssigneeUsername, row.DecisionComment, row.CreatedAt, row.UpdatedAt)

	message, err := queries.GetChatMessage(ctx, report.MessageID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.ReportDetail{}, model.ErrNotFound
	case err != nil:
		return model.ReportDetail{}, fmt.Errorf("load reported message: %w", err)
	}

	contextRows, err := queries.ListThreadMessages(ctx, db.ListThreadMessagesParams{
		ItemID:       message.ItemID,
		FirstUserID:  message.SenderUserID,
		SecondUserID: message.RecipientUserID,
	})
	if err != nil {
		return model.ReportDetail{}, fmt.Errorf("load thread context: %w", err)
	}
	contextMessages := make([]chatmodel.Message, 0, len(contextRows))
	for _, contextRow := range contextRows {
		contextMessages = append(contextMessages, mapChatMessage(contextRow.ID, contextRow.ItemID, contextRow.OriginChainID, contextRow.SenderUserID, contextRow.SenderUsername, contextRow.RecipientUserID, contextRow.RecipientUsername, contextRow.ClientMessageID, contextRow.MessageText, contextRow.CreatedAt))
	}

	return model.ReportDetail{
		Report:          report,
		ReportedMessage: mapChatMessage(message.ID, message.ItemID, message.OriginChainID, message.SenderUserID, message.SenderUsername, message.RecipientUserID, message.RecipientUsername, message.ClientMessageID, message.MessageText, message.CreatedAt),
		Context:         contextMessages,
	}, nil
}

// Assign assigns an open report to the current administrator.
func (r *PostgreSQL) Assign(ctx context.Context, adminID, reportID int64) (model.Report, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Report{}, fmt.Errorf("begin assign report: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := requireAdmin(ctx, queries, adminID); err != nil {
		return model.Report{}, err
	}

	row, err := queries.LockMessageReport(ctx, reportID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Report{}, model.ErrNotFound
	case err != nil:
		return model.Report{}, fmt.Errorf("lock report: %w", err)
	}

	switch {
	case row.Status != model.ReportOpen:
		return model.Report{}, model.ErrStateConflict
	case row.AssigneeUserID.Valid && row.AssigneeUserID.Int64 != adminID:
		return model.Report{}, model.ErrAlreadyAssigned
	case row.AssigneeUserID.Valid && row.AssigneeUserID.Int64 == adminID:
		return commitReadReport(ctx, tx, queries, reportID)
	}

	if err := queries.AssignMessageReport(ctx, db.AssignMessageReportParams{AssigneeID: adminID, ReportID: reportID}); err != nil {
		return model.Report{}, fmt.Errorf("assign report: %w", err)
	}
	if err := insertAudit(ctx, queries, adminID, model.ActionReportAssigned, "report", reportID, map[string]any{"reportId": reportID, "assigneeId": adminID}); err != nil {
		return model.Report{}, err
	}
	report, err := loadReportByID(ctx, queries, reportID)
	if err != nil {
		return model.Report{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Report{}, fmt.Errorf("commit assign report: %w", err)
	}
	return report, nil
}

// Decide applies a single terminal decision to an assigned open report.
func (r *PostgreSQL) Decide(ctx context.Context, adminID, reportID int64, decision, comment string) (model.Report, error) {
	tx, err := r.database.BeginTx(ctx, nil)
	if err != nil {
		return model.Report{}, fmt.Errorf("begin decide report: %w", err)
	}
	defer func() { _ = tx.Rollback() }()

	queries := db.New(tx)
	if err := requireAdmin(ctx, queries, adminID); err != nil {
		return model.Report{}, err
	}

	row, err := queries.LockMessageReport(ctx, reportID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Report{}, model.ErrNotFound
	case err != nil:
		return model.Report{}, fmt.Errorf("lock report: %w", err)
	}

	if row.Status != model.ReportOpen {
		return commitReadReport(ctx, tx, queries, reportID)
	}
	if !row.AssigneeUserID.Valid || row.AssigneeUserID.Int64 != adminID {
		return model.Report{}, model.ErrStateConflict
	}

	affected, err := queries.DecideMessageReport(ctx, db.DecideMessageReportParams{
		Status:          db.ReportStatus(decision),
		DecisionComment: comment,
		ReportID:        reportID,
		AssigneeID:      adminID,
	})
	if err != nil {
		return model.Report{}, fmt.Errorf("decide report: %w", err)
	}
	if affected != 1 {
		return model.Report{}, fmt.Errorf("decide report: unexpected affected rows %d", affected)
	}

	action := model.ActionReportResolved
	if decision == model.DecisionRejected {
		action = model.ActionReportRejected
	}
	if err := insertAudit(ctx, queries, adminID, action, "report", reportID, map[string]any{"reportId": reportID, "fromStatus": model.ReportOpen, "toStatus": decision}); err != nil {
		return model.Report{}, err
	}
	report, err := loadReportByID(ctx, queries, reportID)
	if err != nil {
		return model.Report{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Report{}, fmt.Errorf("commit decide report: %w", err)
	}
	return report, nil
}

// ListAudit returns the newest-first cursor page of the audit log.
func (r *PostgreSQL) ListAudit(ctx context.Context, adminID int64, filter model.AuditFilter, beforeID *int64, limit int) ([]model.AuditEntry, *int64, error) {
	queries := db.New(r.database)
	if err := requireAdmin(ctx, queries, adminID); err != nil {
		return nil, nil, err
	}

	var before sql.NullInt64
	if beforeID != nil {
		before = sql.NullInt64{Int64: *beforeID, Valid: true}
	}

	rows, err := queries.ListAuditLog(ctx, db.ListAuditLogParams{
		BeforeID:   before,
		Action:     filter.Action,
		AdminID:    filter.AdminID,
		TargetType: filter.TargetType,
		PageSize:   int32(limit + 1),
	})
	if err != nil {
		return nil, nil, fmt.Errorf("list audit log: %w", err)
	}

	entries := make([]model.AuditEntry, 0, len(rows))
	for _, row := range rows {
		entry, err := mapAuditEntry(row)
		if err != nil {
			return nil, nil, err
		}
		entries = append(entries, entry)
	}

	var next *int64
	if len(entries) > limit {
		cursor := entries[limit-1].ID
		next = &cursor
		entries = entries[:limit]
	}
	return entries, next, nil
}

func requireAdmin(ctx context.Context, queries *db.Queries, adminID int64) error {
	isAdmin, err := queries.IsAdminUser(ctx, adminID)
	if errors.Is(err, sql.ErrNoRows) || err == nil && !isAdmin {
		return model.ErrForbidden
	}
	if err != nil {
		return fmt.Errorf("authorize moderation: %w", err)
	}
	return nil
}

func insertAudit(ctx context.Context, queries *db.Queries, adminID int64, action, targetType string, targetID int64, metadata map[string]any) error {
	raw, err := json.Marshal(metadata)
	if err != nil {
		return fmt.Errorf("marshal audit metadata: %w", err)
	}
	if err := queries.InsertAuditLog(ctx, db.InsertAuditLogParams{
		AdminID:    adminID,
		Action:     db.AuditAction(action),
		TargetType: targetType,
		TargetID:   targetID,
		Metadata:   raw,
	}); err != nil {
		return fmt.Errorf("insert audit log: %w", err)
	}
	return nil
}

func commitReadReport(ctx context.Context, tx *sql.Tx, queries *db.Queries, reportID int64) (model.Report, error) {
	report, err := loadReportByID(ctx, queries, reportID)
	if err != nil {
		return model.Report{}, err
	}
	if err := tx.Commit(); err != nil {
		return model.Report{}, fmt.Errorf("commit report read: %w", err)
	}
	return report, nil
}

func loadReportByID(ctx context.Context, queries *db.Queries, reportID int64) (model.Report, error) {
	row, err := queries.GetModerationReport(ctx, reportID)
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Report{}, model.ErrNotFound
	case err != nil:
		return model.Report{}, fmt.Errorf("load report: %w", err)
	}
	return mapReport(row.ID, row.ReporterUserID, row.ReporterUsername, row.MessageID, row.Reason, row.Comment, row.Status, row.AssigneeUserID, row.AssigneeUsername, row.DecisionComment, row.CreatedAt, row.UpdatedAt), nil
}

func loadReportByReporterMessage(ctx context.Context, queries *db.Queries, reporterID, messageID int64) (model.Report, error) {
	row, err := queries.GetMessageReportByReporterMessage(ctx, db.GetMessageReportByReporterMessageParams{ReporterID: reporterID, MessageID: messageID})
	switch {
	case errors.Is(err, sql.ErrNoRows):
		return model.Report{}, model.ErrNotFound
	case err != nil:
		return model.Report{}, fmt.Errorf("load report by reporter and message: %w", err)
	}
	return mapReport(row.ID, row.ReporterUserID, row.ReporterUsername, row.MessageID, row.Reason, row.Comment, row.Status, row.AssigneeUserID, row.AssigneeUsername, row.DecisionComment, row.CreatedAt, row.UpdatedAt), nil
}

func mapReport(id, reporterUserID int64, reporterUsername string, messageID int64, reason string, comment sql.NullString, status string, assigneeUserID sql.NullInt64, assigneeUsername sql.NullString, decisionComment sql.NullString, createdAt, updatedAt time.Time) model.Report {
	report := model.Report{
		ID:        id,
		Reporter:  model.User{ID: reporterUserID, Username: reporterUsername},
		MessageID: messageID,
		Reason:    reason,
		Status:    status,
		CreatedAt: createdAt,
		UpdatedAt: updatedAt,
	}
	if comment.Valid {
		value := comment.String
		report.Comment = &value
	}
	if assigneeUserID.Valid {
		username := ""
		if assigneeUsername.Valid {
			username = assigneeUsername.String
		}
		report.Assignee = &model.User{ID: assigneeUserID.Int64, Username: username}
	}
	if decisionComment.Valid {
		value := decisionComment.String
		report.DecisionComment = &value
	}
	return report
}

func mapChatMessage(id, itemID, originChainID, senderUserID int64, senderUsername string, recipientUserID int64, recipientUsername, clientMessageID, text string, createdAt time.Time) chatmodel.Message {
	return chatmodel.Message{
		ID:            id,
		ItemID:        itemID,
		OriginChainID: originChainID,
		Sender:        chatmodel.Sender{ID: senderUserID, Username: senderUsername},
		Recipient: chatmodel.Sender{
			ID:       recipientUserID,
			Username: recipientUsername,
		},
		ClientMessageID: clientMessageID,
		Text:            text,
		CreatedAt:       createdAt,
	}
}

func mapAuditEntry(row db.ListAuditLogRow) (model.AuditEntry, error) {
	entry := model.AuditEntry{
		ID:         row.ID,
		Admin:      model.User{ID: row.AdminUserID, Username: row.AdminUsername},
		Action:     row.Action,
		TargetType: row.TargetType,
		TargetID:   row.TargetID,
		CreatedAt:  row.CreatedAt,
	}
	metadata := map[string]any{}
	if len(row.Metadata) > 0 {
		if err := json.Unmarshal(row.Metadata, &metadata); err != nil {
			return model.AuditEntry{}, fmt.Errorf("unmarshal audit metadata: %w", err)
		}
	}
	entry.Metadata = metadata
	return entry, nil
}
