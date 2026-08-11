package service

import (
	"context"
	"strings"

	"swap-chain/modules/reputation/model"
)

type Repository interface {
	Stats(ctx context.Context, userID int64) (model.Stats, error)
	CreateReview(ctx context.Context, authorID, chainID int64, input model.CreateInput) (model.Review, error)
	ListReviews(ctx context.Context, targetUserID, cursor int64, limit int) (model.ListResult, error)
}

type Service interface {
	Stats(ctx context.Context, userID int64) (model.Stats, error)
	CreateReview(ctx context.Context, authorID, chainID int64, input model.CreateInput) (model.Review, error)
	ListReviews(ctx context.Context, targetUserID, cursor int64, limit int) (model.ListResult, error)
}

type Reputation struct {
	repository Repository
}

func New(repository Repository) (*Reputation, error) {
	if repository == nil {
		return nil, &model.ValidationError{Field: "repository", Message: "is required"}
	}
	return &Reputation{repository: repository}, nil
}

func (s *Reputation) Stats(ctx context.Context, userID int64) (model.Stats, error) {
	if userID <= 0 {
		return model.Stats{}, &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	return s.repository.Stats(ctx, userID)
}

func (s *Reputation) CreateReview(ctx context.Context, authorID, chainID int64, input model.CreateInput) (model.Review, error) {
	if authorID <= 0 {
		return model.Review{}, model.ErrForbidden
	}
	if chainID <= 0 {
		return model.Review{}, &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	if input.TargetUserID <= 0 {
		return model.Review{}, &model.ValidationError{Field: "targetUserId", Message: "must be positive"}
	}
	if input.TargetUserID == authorID {
		return model.Review{}, model.ErrForbidden
	}
	if input.Rating < 1 || input.Rating > 5 {
		return model.Review{}, &model.ValidationError{Field: "rating", Message: "must be between 1 and 5"}
	}
	if input.Text != nil {
		text := strings.TrimSpace(*input.Text)
		if text == "" {
			input.Text = nil
		} else {
			if len([]rune(text)) > 1000 {
				return model.Review{}, &model.ValidationError{Field: "text", Message: "must contain at most 1000 characters"}
			}
			input.Text = &text
		}
	}
	return s.repository.CreateReview(ctx, authorID, chainID, input)
}

func (s *Reputation) ListReviews(ctx context.Context, targetUserID, cursor int64, limit int) (model.ListResult, error) {
	if targetUserID <= 0 {
		return model.ListResult{}, &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	if cursor < 0 {
		return model.ListResult{}, &model.ValidationError{Field: "cursor", Message: "must be non-negative"}
	}
	if limit < 1 || limit > 100 {
		return model.ListResult{}, &model.ValidationError{Field: "limit", Message: "must be between 1 and 100"}
	}
	return s.repository.ListReviews(ctx, targetUserID, cursor, limit)
}
