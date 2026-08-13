package repository

import (
	"database/sql"
	"testing"
	"time"
)

func TestNewPostgreSQLRequiresDatabase(t *testing.T) {
	if _, err := NewPostgreSQL(nil); err == nil {
		t.Fatal("NewPostgreSQL(nil) error = nil")
	}
}

func TestReportMapping(t *testing.T) {
	createdAt := time.Unix(100, 0).UTC()
	updatedAt := time.Unix(200, 0).UTC()
	report := mapReport(
		7, 1, "Аня", 42, "spam",
		sql.NullString{String: "назойливый", Valid: true},
		"open",
		sql.NullInt64{Int64: 9, Valid: true},
		sql.NullString{String: "ПВЗ", Valid: true},
		sql.NullString{},
		createdAt, updatedAt,
	)
	if report.ID != 7 || report.Reporter.ID != 1 || report.Reporter.Username != "Аня" ||
		report.MessageID != 42 || report.Reason != "spam" || report.Status != "open" ||
		report.Comment == nil || *report.Comment != "назойливый" ||
		report.Assignee == nil || report.Assignee.ID != 9 ||
		report.DecisionComment != nil ||
		!report.CreatedAt.Equal(createdAt) || !report.UpdatedAt.Equal(updatedAt) {
		t.Fatalf("mapReport() = %+v", report)
	}
}
