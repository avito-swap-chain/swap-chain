package service

import (
	"context"
	"errors"
	"strings"
	"testing"

	"swap-chain/modules/reputation/model"
)

type repositoryStub struct {
	input       model.CreateInput
	statsUserID int64
	listUserID  int64
	listCursor  int64
	listLimit   int
}

func (r *repositoryStub) Stats(_ context.Context, userID int64) (model.Stats, error) {
	r.statsUserID = userID
	return model.Stats{CompletedExchanges: 3}, nil
}

func (r *repositoryStub) CreateReview(_ context.Context, _, _ int64, input model.CreateInput) (model.Review, error) {
	r.input = input
	return model.Review{}, nil
}

func (r *repositoryStub) ListReviews(_ context.Context, userID int64, cursor int64, limit int) (model.ListResult, error) {
	r.listUserID, r.listCursor, r.listLimit = userID, cursor, limit
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

func TestReputationValidatesQueriesAndDelegates(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded")
	}
	repository := &repositoryStub{}
	service, _ := New(repository)

	if _, err := service.Stats(context.Background(), 0); err == nil {
		t.Fatal("Stats(0) succeeded")
	}
	stats, err := service.Stats(context.Background(), 7)
	if err != nil || stats.CompletedExchanges != 3 || repository.statsUserID != 7 {
		t.Fatalf("Stats() = %+v, %v", stats, err)
	}

	for _, input := range []struct {
		userID, cursor int64
		limit          int
	}{{0, 0, 20}, {1, -1, 20}, {1, 0, 0}, {1, 0, 101}} {
		if _, err := service.ListReviews(context.Background(), input.userID, input.cursor, input.limit); err == nil {
			t.Fatalf("ListReviews(%+v) succeeded", input)
		}
	}
	if _, err := service.ListReviews(context.Background(), 8, 12, 50); err != nil {
		t.Fatal(err)
	}
	if repository.listUserID != 8 || repository.listCursor != 12 || repository.listLimit != 50 {
		t.Fatalf("list arguments = %d/%d/%d", repository.listUserID, repository.listCursor, repository.listLimit)
	}
}

func TestCreateReviewTrimsTextAndRejectsInvalidIdentity(t *testing.T) {
	repository := &repositoryStub{}
	service, _ := New(repository)

	text := "  Всё отлично  "
	if _, err := service.CreateReview(context.Background(), 1, 2, model.CreateInput{TargetUserID: 3, Rating: 4, Text: &text}); err != nil {
		t.Fatal(err)
	}
	if repository.input.Text == nil || *repository.input.Text != "Всё отлично" {
		t.Fatalf("normalized text = %v", repository.input.Text)
	}

	longText := strings.Repeat("я", 1001)
	for _, call := range []struct {
		authorID, chainID int64
		input             model.CreateInput
	}{{0, 1, model.CreateInput{TargetUserID: 2, Rating: 5}}, {1, 0, model.CreateInput{TargetUserID: 2, Rating: 5}}, {1, 1, model.CreateInput{TargetUserID: 0, Rating: 5}}, {1, 1, model.CreateInput{TargetUserID: 2, Rating: 5, Text: &longText}}} {
		if _, err := service.CreateReview(context.Background(), call.authorID, call.chainID, call.input); err == nil {
			t.Fatalf("CreateReview(%+v) succeeded", call)
		}
	}
}
