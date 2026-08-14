package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/metrics/model"
)

type repositoryStub struct {
	actorID int64
	result  model.Funnel
	err     error
}

func (r *repositoryStub) Funnel(_ context.Context, actorID int64) (model.Funnel, error) {
	r.actorID = actorID
	return r.result, r.err
}

func TestMetricsValidatesAdminAndDelegates(t *testing.T) {
	if _, err := New(nil); err == nil {
		t.Fatal("New(nil) succeeded")
	}

	repository := &repositoryStub{result: model.Funnel{EligibleItems: 12}}
	service, err := New(repository)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := service.Funnel(context.Background(), 0); !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("Funnel(0) error = %v", err)
	}
	result, err := service.Funnel(context.Background(), 42)
	if err != nil || result.EligibleItems != 12 || repository.actorID != 42 {
		t.Fatalf("Funnel() = %+v, %v; actor=%d", result, err, repository.actorID)
	}
}
