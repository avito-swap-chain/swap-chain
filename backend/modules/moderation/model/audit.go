// Package model defines the append-only administrator audit log.
package model

import "time"

const (
	ActionReportAssigned        = "REPORT_ASSIGNED"
	ActionReportResolved        = "REPORT_RESOLVED"
	ActionReportRejected        = "REPORT_REJECTED"
	ActionUserBlocked           = "USER_BLOCKED"
	ActionChainCancelled        = "CHAIN_CANCELLED"
	ActionDeliveryStatusChanged = "DELIVERY_STATUS_CHANGED"
)

// AuditEntry is a single immutable administrative audit record.
type AuditEntry struct {
	ID         int64
	Admin      User
	Action     string
	TargetType string
	TargetID   int64
	Metadata   map[string]any
	CreatedAt  time.Time
}

// AuditFilter narrows the audit log.
type AuditFilter struct {
	Action     string
	AdminID    int64
	TargetType string
}
