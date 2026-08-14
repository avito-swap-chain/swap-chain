package service

import (
	"context"
	"fmt"

	"swap-chain/modules/admin/model"
)

type Repository interface {
	ListChains(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Chain, *int64, error)
	GetChain(ctx context.Context, actorID, chainID int64) (model.Chain, error)
	ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error)
	TransitionDelivery(ctx context.Context, actorID, deliveryID int64, targetStatus string) (model.Delivery, error)
	ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error)
	ConfirmParticipantReceipt(ctx context.Context, actorID, chainID, participantID int64) (model.Receipt, error)
}

type OnDeliveryTransition func(ctx context.Context, delivery model.Delivery)

type Service interface {
	ListChains(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Chain, *int64, error)
	GetChain(ctx context.Context, actorID, chainID int64) (model.Chain, error)
	ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error)
	TransitionDelivery(ctx context.Context, actorID, deliveryID int64, targetStatus string) (model.Delivery, error)
	ConfirmReceipt(ctx context.Context, actorID, chainID int64) (model.Receipt, error)
	ConfirmParticipantReceipt(ctx context.Context, actorID, chainID, participantID int64) (model.Receipt, error)
}

func (s *Admin) ListChains(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Chain, *int64, error) {
	if actorID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > s.cfg.MaxListLimit {
		return nil, nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.cfg.MaxListLimit)}
	}
	if afterID < 0 {
		return nil, nil, &model.ValidationError{Field: "cursor", Message: "must be a non-negative chain ID"}
	}
	if status != "" && status != model.ChainAccepted && status != model.ChainCompleted {
		return nil, nil, &model.ValidationError{Field: "status", Message: "must be ACCEPTED or COMPLETED"}
	}
	return s.repository.ListChains(ctx, actorID, status, afterID, limit)
}

func (s *Admin) GetChain(ctx context.Context, actorID, chainID int64) (model.Chain, error) {
	if actorID <= 0 {
		return model.Chain{}, model.ErrForbidden
	}
	if chainID <= 0 {
		return model.Chain{}, &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	return s.repository.GetChain(ctx, actorID, chainID)
}

type AdminConfig struct {
	MaxListLimit int
}

type Admin struct {
	repository Repository
	onDelivery OnDeliveryTransition
	cfg        AdminConfig
}

func New(repository Repository, cfg AdminConfig) (*Admin, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin init: repository is required")
	}
	return &Admin{repository: repository, cfg: cfg}, nil
}

func NewWithCallback(repository Repository, onDelivery OnDeliveryTransition, cfg AdminConfig) (*Admin, error) {
	if repository == nil {
		return nil, fmt.Errorf("admin init: repository is required")
	}
	return &Admin{repository: repository, onDelivery: onDelivery, cfg: cfg}, nil
}

func (s *Admin) ListDeliveries(ctx context.Context, actorID int64, status string, afterID int64, limit int) ([]model.Delivery, *int64, error) {
	if actorID <= 0 {
		return nil, nil, model.ErrForbidden
	}
	if limit < 1 || limit > s.cfg.MaxListLimit {
		return nil, nil, &model.ValidationError{Field: "limit", Message: fmt.Sprintf("must be between 1 and %d", s.cfg.MaxListLimit)}
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

func (s *Admin) ConfirmParticipantReceipt(ctx context.Context, actorID, chainID, participantID int64) (model.Receipt, error) {
	if actorID <= 0 {
		return model.Receipt{}, model.ErrForbidden
	}
	if chainID <= 0 {
		return model.Receipt{}, &model.ValidationError{Field: "chainId", Message: "must be positive"}
	}
	if participantID <= 0 {
		return model.Receipt{}, &model.ValidationError{Field: "participantId", Message: "must be positive"}
	}
	receipt, err := s.repository.ConfirmParticipantReceipt(ctx, actorID, chainID, participantID)
	if err == nil && s.onDelivery != nil {
		s.onDelivery(ctx, receipt.Delivery)
	}
	return receipt, err
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
