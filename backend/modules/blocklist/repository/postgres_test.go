package repository

import (
	"context"
	"database/sql"
	"testing"

	"swap-chain/internal/chains"
)

type fakeCanceler struct{}

func (fakeCanceler) CancelPendingBetween(_ context.Context, _ *sql.Tx, _, _ int64) ([]chains.Chain, error) {
	return nil, nil
}

func (fakeCanceler) PublishRejections(_ []chains.Chain, _ string) {}

func TestNewPostgreSQLRequiresDatabaseAndCanceler(t *testing.T) {
	if _, err := NewPostgreSQL(nil, fakeCanceler{}); err == nil {
		t.Fatal("NewPostgreSQL(nil, canceler) error = nil")
	}
	if _, err := NewPostgreSQL(&sql.DB{}, nil); err == nil {
		t.Fatal("NewPostgreSQL(db, nil) error = nil")
	}
	if _, err := NewPostgreSQL(&sql.DB{}, fakeCanceler{}); err != nil {
		t.Fatalf("NewPostgreSQL() error = %v", err)
	}
}
