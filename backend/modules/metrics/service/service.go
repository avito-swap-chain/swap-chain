package service

import (
	"context"
	"fmt"

	"swap-chain/modules/metrics/model"
)

type Repository interface {
	Funnel(ctx context.Context, actorID int64) (model.Funnel, error)
}

type Service interface {
	Funnel(ctx context.Context, actorID int64) (model.Funnel, error)
}

type Metrics struct {
	repository Repository
}

func New(repository Repository) (*Metrics, error) {
	if repository == nil {
		return nil, fmt.Errorf("metrics service: repository is required")
	}
	return &Metrics{repository: repository}, nil
}

func (s *Metrics) Funnel(ctx context.Context, actorID int64) (model.Funnel, error) {
	if actorID <= 0 {
		return model.Funnel{}, model.ErrForbidden
	}
	return s.repository.Funnel(ctx, actorID)
}
