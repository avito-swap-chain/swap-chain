package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/reputation/model"
)

type repositoryStub struct {
	input model.CreateInput
}

func (*repositoryStub) Stats(_ context.Context, _ int64) (model.Stats, error) {
	return model.Stats{}, nil
}

func (r *repositoryStub) CreateReview(_ context.Context, _, _ int64, input model.CreateInput) (model.Review, error) {
	r.input = input
	return model.Review{}, nil
}

func (*repositoryStub) ListReviews(_ context.Context, _ int64, _ int64, _ int) (model.ListResult, error) {
	return model.ListResult{}, nil
}

func TestCreateReviewValidatesAndNormalizesInput(t *testing.T) {
	repository := &repositoryStub{}
	service, err := New(repository)
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}

	spaces := "   "
	if _, err := service.CreateReview(context.Background(), 1, 2, model.CreateInput{TargetUserID: 3, Rating: 5, Text: &spaces}); err != nil {
		t.Fatalf("CreateReview() error = %v", err)
	}
	if repository.input.Text != nil {
		t.Fatalf("normalized empty text = %q, want nil", *repository.input.Text)
	}

	for _, test := range []struct {
		name  string
		input model.CreateInput
		err   error
	}{
		{name: "self review", input: model.CreateInput{TargetUserID: 1, Rating: 5}, err: model.ErrForbidden},
		{name: "rating too low", input: model.CreateInput{TargetUserID: 2, Rating: 0}},
		{name: "rating too high", input: model.CreateInput{TargetUserID: 2, Rating: 6}},
	} {
		t.Run(test.name, func(t *testing.T) {
			_, got := service.CreateReview(context.Background(), 1, 2, test.input)
			if test.err != nil && !errors.Is(got, test.err) {
				t.Fatalf("error = %v, want %v", got, test.err)
			}
			if test.err == nil {
				var validationError *model.ValidationError
				if !errors.As(got, &validationError) {
					t.Fatalf("error = %v, want ValidationError", got)
				}
			}
		})
	}
}
