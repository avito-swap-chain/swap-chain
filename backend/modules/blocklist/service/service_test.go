package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/blocklist/model"
)

type fakeRepository struct {
	block model.Block
	err   error
}

func (f *fakeRepository) Block(_ context.Context, _, _ int64) (model.Block, error) {
	return f.block, f.err
}

func (f *fakeRepository) Unblock(_ context.Context, _, _ int64) error {
	return f.err
}

func (f *fakeRepository) List(_ context.Context, _, _ int64, _ int) ([]model.Block, *int64, error) {
	return nil, nil, f.err
}

func newTestService() *Blocklist {
	return &Blocklist{repository: &fakeRepository{}, cfg: BlocklistConfig{MaxListLimit: 100}}
}

func TestBlockRejectsSelfAndInvalidTarget(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, err := service.Block(ctx, 0, 2); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("Block(blocker=0) error = %v, want ErrForbidden", err)
	}
	if _, err := service.Block(ctx, 1, 0); !isValidationError(err) {
		t.Fatalf("Block(blocked=0) error = %v, want ValidationError", err)
	}
	if _, err := service.Block(ctx, 1, 1); !errors.Is(err, model.ErrSelfBlock) {
		t.Fatalf("Block(self) error = %v, want ErrSelfBlock", err)
	}
	if _, err := service.Block(ctx, 1, 2); err != nil {
		t.Fatalf("Block(valid) error = %v", err)
	}
}

func TestUnblockValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if err := service.Unblock(ctx, 0, 2); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("Unblock(blocker=0) error = %v, want ErrForbidden", err)
	}
	if err := service.Unblock(ctx, 1, 0); !isValidationError(err) {
		t.Fatalf("Unblock(blocked=0) error = %v, want ValidationError", err)
	}
	if err := service.Unblock(ctx, 1, 2); err != nil {
		t.Fatalf("Unblock(valid) error = %v", err)
	}
}

func TestListValidation(t *testing.T) {
	service := newTestService()
	ctx := context.Background()

	if _, _, err := service.List(ctx, 0, 0, 10); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("List(blocker=0) error = %v, want ErrForbidden", err)
	}
	if _, _, err := service.List(ctx, 1, 0, 0); !isValidationError(err) {
		t.Fatalf("List(limit=0) error = %v, want ValidationError", err)
	}
	if _, _, err := service.List(ctx, 1, -1, 10); !isValidationError(err) {
		t.Fatalf("List(after=-1) error = %v, want ValidationError", err)
	}
	if _, _, err := service.List(ctx, 1, 0, 10); err != nil {
		t.Fatalf("List(valid) error = %v", err)
	}
}

func isValidationError(err error) bool {
	var validationError *model.ValidationError
	return errors.As(err, &validationError)
}
