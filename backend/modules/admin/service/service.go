package service

import (
	"context"
	"fmt"

	"swap-chain/modules/admin/model"
)

type Repository interface {
	ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error)
	TransitionDelivery(ctx context.Context, actorID, deliveryID int64, targetStatus string) (model.Delivery, error)
	ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error)
}

type OnDeliveryTransition func(ctx context.Context, delivery model.Delivery)

type Service interface {
	ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error)
	TransitionDelivery(ctx context.Context, actorID, deliveryID int64, targetStatus string) (model.Delivery, error)
	ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error)
}

type Admin struct {
	repository Repository
	onDelivery OnDeliveryTransition
}

func New(repository Repository) (*Admin, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin init: repository is required")
	}
	return &Admin{repository: repository}, nil
}

func NewWithCallback(repository Repository, onDelivery OnDeliveryTransition) (*Admin, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin init: repository is required")
	}
	return &Admin{repository: repository, onDelivery: onDelivery}, nil
}

func (s *Admin) ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error) {
	if actorID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > 100 {
		return nil, nil, &model.ValidationError{Field: "limit", Message: "must be between 1 and 100"}
	}
	if afterID < 0 {
		return nil, nil, &model.ValidationError{Field: "cursor", Message: "must be a positive delivery ID"}
	}
	if status != "" && status != model.DeliveryAwaitingPVZ && status != model.DeliveryAtPVZ && status != model.DeliveryInTransit && status != model.DeliveryReceived {
		return nil, nil, &model.ValidationError{Field: "status", Message: "unsupported delivery status"}
	}
	return s.repository.ListDeliveries(ctx, actorID, status, afterID, limit)
}

// ConfirmReceipt подтверждает получение только входящей вещи текущего участника цепочки.
func (s *Admin) ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error) {
	if actorID <= 0 {
		return model.Receipt{}, model.ErrReceiptForbidden
	}
	if chainID <= 0 {
		return model.Receipt{}, &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	return s.repository.ConfirmReceipt(ctx, actorID, chainID)
}

func (s *Admin) TransitionDelivery(ctx context.Context, actorID, deliveryID int64, targetStatus string) (model.Delivery, error) {
	if actorID <= 0 {
		return model.Delivery{}, model.ErrForbidden
	}
	if deliveryID <= 0 {
		return model.Delivery{}, &model.ValidationError{Field: "deliveryId", Message: "must be positive"}
	}
	if err := model.ValidateTargetStatus(targetStatus); err != nil {
		return model.Delivery{}, err
	}
	delivery, err := s.repository.TransitionDelivery(ctx, actorID, deliveryID, targetStatus)
	if err != nil {
		return delivery, err
	}
	if s.onDelivery != nil {
		s.onDelivery(ctx, delivery)
	}
	return delivery, nil
}
