package service

import (
	"context"

	"swap-chain/modules/notifications/model"
)

type Repository interface {
	Create(ctx context.Context, notification model.Notification) (model.Notification, error)
	List(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error)
	MarkRead(ctx context.Context, userID int64, ids []int64) error
}

type Service interface {
	Create(ctx context.Context, userID int64, kind, title, text, targetURL string, entityID *int64) (model.Notification, error)
	List(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error)
	MarkRead(ctx context.Context, userID int64, ids []int64) error
}

type NotificationService struct {
	repo Repository
}

func New(repo Repository) (*NotificationService, error) {
	if repo == nil {
		return nil, &model.ValidationError{Field: "repo", Message: "repository is required"}
	}
	return &NotificationService{repo: repo}, nil
}

func (s *NotificationService) Create(ctx context.Context, userID int64, kind, title, text, targetURL string, entityID *int64) (model.Notification, error) {
	if userID <= 0 {
		return model.Notification{}, &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	if kind != model.KindChain && kind != model.KindMessage && kind != model.KindOffer {
		return model.Notification{}, &model.ValidationError{Field: "kind", Message: "must be chain, message, or offer"}
	}
	if title == "" {
		return model.Notification{}, &model.ValidationError{Field: "title", Message: "must not be empty"}
	}

	return s.repo.Create(ctx, model.Notification{
		UserID:    userID,
		Kind:      kind,
		Title:     title,
		Text:      text,
		TargetURL: targetURL,
		EntityID:  entityID,
	})
}

func (s *NotificationService) List(ctx context.Context, userID int64, cursor int64, limit int) (model.ListResult, error) {
	if userID <= 0 {
		return model.ListResult{}, &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	if err := model.ValidateListParams(cursor, limit); err != nil {
		return model.ListResult{}, err
	}
	return s.repo.List(ctx, userID, cursor, limit)
}

func (s *NotificationService) MarkRead(ctx context.Context, userID int64, ids []int64) error {
	if userID <= 0 {
		return &model.ValidationError{Field: "userId", Message: "must be positive"}
	}
	return s.repo.MarkRead(ctx, userID, ids)
}
