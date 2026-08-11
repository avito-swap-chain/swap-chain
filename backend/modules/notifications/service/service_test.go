package service_test

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/notifications/model"
	"swap-chain/modules/notifications/service"
)

type mockRepository struct {
	createFunc   func(ctx context.Context, n model.Notification) (model.Notification, error)
	listFunc     func(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error)
	markReadFunc func(ctx context.Context, userID int64, ids []int64) error
}

func (m *mockRepository) Create(ctx context.Context, n model.Notification) (model.Notification, error) {
	return m.createFunc(ctx, n)
}

func (m *mockRepository) List(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error) {
	return m.listFunc(ctx, userID, cursor, limit)
}

func (m *mockRepository) MarkRead(ctx context.Context, userID int64, ids []int64) error {
	return m.markReadFunc(ctx, userID, ids)
}

func TestCreate_ValidatesRequiredFields(t *testing.T) {
	repo := &mockRepository{}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	tests := []struct {
		name    string
		userID  int64
		kind    string
		title   string
		wantErr string
	}{
		{name: "negative userID", userID: 0, kind: "chain", title: "Test", wantErr: "must be positive"},
		{name: "invalid kind", userID: 1, kind: "invalid", title: "Test", wantErr: "must be chain, message, or offer"},
		{name: "empty title", userID: 1, kind: "chain", title: "", wantErr: "must not be empty"},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			_, err := svc.Create(ctx, tt.userID, tt.kind, tt.title, "", "", nil)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			var valErr *model.ValidationError
			if !errors.As(err, &valErr) {
				t.Fatalf("expected ValidationError, got %T: %v", err, err)
			}
		})
	}
}

func TestCreate_Success(t *testing.T) {
	repo := &mockRepository{
		createFunc: func(ctx context.Context, n model.Notification) (model.Notification, error) {
			n.ID = 1
			return n, nil
		},
	}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	entityID := int64(42)
	result, err := svc.Create(context.Background(), 1, "chain", "Title", "Text", "/chains/1", &entityID)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.ID != 1 {
		t.Errorf("expected ID 1, got %d", result.ID)
	}
	if result.Kind != "chain" {
		t.Errorf("expected kind chain, got %s", result.Kind)
	}
}

func TestList_Validation(t *testing.T) {
	repo := &mockRepository{}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	ctx := context.Background()

	_, err = svc.List(ctx, 0, 0, 20)
	if err == nil {
		t.Fatal("expected error for zero userID")
	}

	_, err = svc.List(ctx, 1, -1, 20)
	if err == nil {
		t.Fatal("expected error for negative cursor")
	}

	_, err = svc.List(ctx, 1, 0, 0)
	if err == nil {
		t.Fatal("expected error for zero limit")
	}

	_, err = svc.List(ctx, 1, 0, 200)
	if err == nil {
		t.Fatal("expected error for limit > 100")
	}
}

func TestMarkRead_Validation(t *testing.T) {
	repo := &mockRepository{}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	err = svc.MarkRead(context.Background(), 0, nil)
	if err == nil {
		t.Fatal("expected error for zero userID")
	}
}

func TestMarkRead_All(t *testing.T) {
	called := false
	repo := &mockRepository{
		markReadFunc: func(ctx context.Context, userID int64, ids []int64) error {
			called = true
			if len(ids) != 0 {
				t.Errorf("expected empty ids for mark all, got %v", ids)
			}
			return nil
		},
	}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.MarkRead(context.Background(), 1, nil); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !called {
		t.Fatal("expected markRead to be called")
	}
}

func TestMarkRead_Selected(t *testing.T) {
	ids := []int64{1, 2, 3}
	repo := &mockRepository{
		markReadFunc: func(ctx context.Context, userID int64, receivedIDs []int64) error {
			if len(receivedIDs) != len(ids) {
				t.Errorf("expected %d ids, got %d", len(ids), len(receivedIDs))
			}
			return nil
		},
	}
	svc, err := service.New(repo)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if err := svc.MarkRead(context.Background(), 1, ids); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
