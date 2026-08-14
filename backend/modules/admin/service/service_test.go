package service

import (
	"context"
	"errors"
	"testing"

	"swap-chain/modules/admin/model"
)

type repositoryStub struct {
	chains           []model.Chain
	chainResult      model.Chain
	deliveries       []model.Delivery
	next             *int64
	listError        error
	transitionResult model.Delivery
	transitionError  error
	receiptResult    model.Receipt
	receiptError     error

	listCalls       int
	transitionCalls int
	receiptCalls    int
	actorID         int64
	deliveryID      int64
	chainID         int64
	targetStatus    string
}

func (r *repositoryStub) ListChains(_ context.Context, actorID int64, _ string, _ int64, _ int) ([]model.Chain, *int64, error) {
	r.actorID = actorID
	return r.chains, r.next, r.listError
}

func (r *repositoryStub) GetChain(_ context.Context, actorID, chainID int64) (model.Chain, error) {
	r.actorID, r.chainID = actorID, chainID
	return r.chainResult, r.listError
}

func (r *repositoryStub) ListDeliveries(
	_ context.Context,
	actorID int64,
	_ string,
	_ int64,
	_ int,
) ([]model.Delivery, *int64, error) {
	r.listCalls++
	r.actorID = actorID
	return r.deliveries, r.next, r.listError
}

func (r *repositoryStub) TransitionDelivery(
	_ context.Context,
	actorID int64,
	deliveryID int64,
	targetStatus string,
) (model.Delivery, error) {
	r.transitionCalls++
	r.actorID = actorID
	r.deliveryID = deliveryID
	r.targetStatus = targetStatus
	return r.transitionResult, r.transitionError
}

func (r *repositoryStub) ConfirmReceipt(_ context.Context, actorID, chainID int64) (model.Receipt, error) {
	r.receiptCalls++
	r.actorID = actorID
	r.chainID = chainID
	return r.receiptResult, r.receiptError
}

func (r *repositoryStub) ConfirmParticipantReceipt(_ context.Context, actorID, chainID, participantID int64) (model.Receipt, error) {
	r.receiptCalls++
	r.actorID, r.chainID, r.deliveryID = actorID, chainID, participantID
	return r.receiptResult, r.receiptError
}

func TestNewRequiresRepository(t *testing.T) {
	if _, err := New(nil, AdminConfig{MaxListLimit: 100}); err == nil {
		t.Fatal("New(nil) error = nil")
	}
}

func TestTransitionDeliveryCallsCallbackAfterSuccess(t *testing.T) {
	want := model.Delivery{ID: 10, Status: model.DeliveryAtPVZ}
	repository := &repositoryStub{transitionResult: want}
	callbackCalls := 0
	admin, err := NewWithCallback(repository, func(_ context.Context, got model.Delivery) {
		callbackCalls++
		if got != want {
			t.Errorf("callback delivery = %+v, want %+v", got, want)
		}
	}, AdminConfig{MaxListLimit: 100})
	if err != nil {
		t.Fatalf("NewWithCallback() error = %v", err)
	}

	if _, err := admin.TransitionDelivery(context.Background(), 7, 10, model.DeliveryAtPVZ); err != nil {
		t.Fatalf("TransitionDelivery() error = %v", err)
	}
	if callbackCalls != 1 {
		t.Fatalf("callback calls = %d, want 1", callbackCalls)
	}
	repository.transitionError = model.ErrTransitionConflict
	if _, err := admin.TransitionDelivery(context.Background(), 7, 10, model.DeliveryAtPVZ); !errors.Is(err, model.ErrTransitionConflict) {
		t.Fatalf("TransitionDelivery() error = %v", err)
	}
	if callbackCalls != 1 {
		t.Fatalf("callback called after repository error: %d", callbackCalls)
	}
}

func TestConfirmReceiptValidatesCommand(t *testing.T) {
	admin := newTestAdmin(t, &repositoryStub{})
	if _, err := admin.ConfirmReceipt(context.Background(), 0, 1); !errors.Is(err, model.ErrReceiptForbidden) {
		t.Fatalf("ConfirmReceipt() actor error = %v", err)
	}
	var validationError *model.ValidationError
	if _, err := admin.ConfirmReceipt(context.Background(), 1, 0); !errors.As(err, &validationError) {
		t.Fatalf("ConfirmReceipt() chain error = %v", err)
	}
}

func TestListDeliveriesValidatesCommand(t *testing.T) {
	repository := &repositoryStub{}
	admin := newTestAdmin(t, repository)

	tests := []struct {
		name    string
		actorID int64
		status  string
		afterID int64
		limit   int
	}{
		{name: "empty actor", actorID: 0, limit: 20},
		{name: "invalid status", actorID: 1, status: "UNKNOWN", limit: 20},
		{name: "negative cursor", actorID: 1, afterID: -1, limit: 20},
		{name: "zero limit", actorID: 1, limit: 0},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			_, _, err := admin.ListDeliveries(
				context.Background(),
				test.actorID,
				test.status,
				test.afterID,
				test.limit,
			)
			if err == nil {
				t.Fatal("ListDeliveries() error = nil")
			}
		})
	}

	if repository.listCalls != 0 {
		t.Fatalf("repository called %d times", repository.listCalls)
	}
}

func TestTransitionDeliveryForwardsCommandAndError(t *testing.T) {
	repository := &repositoryStub{
		transitionResult: model.Delivery{ID: 10, Status: model.DeliveryAtPVZ},
	}
	admin := newTestAdmin(t, repository)

	delivery, err := admin.TransitionDelivery(context.Background(), 7, 10, model.DeliveryAtPVZ)
	if err != nil {
		t.Fatalf("TransitionDelivery() error = %v", err)
	}
	if delivery != repository.transitionResult {
		t.Fatalf("TransitionDelivery() = %+v, want %+v", delivery, repository.transitionResult)
	}
	if repository.actorID != 7 || repository.deliveryID != 10 || repository.targetStatus != model.DeliveryAtPVZ {
		t.Fatalf("repository command = actor %d, delivery %d, status %q", repository.actorID, repository.deliveryID, repository.targetStatus)
	}

	repository.transitionError = model.ErrForbidden
	_, err = admin.TransitionDelivery(context.Background(), 7, 10, model.DeliveryAtPVZ)
	if !errors.Is(err, model.ErrForbidden) {
		t.Fatalf("TransitionDelivery() error = %v, want ErrForbidden", err)
	}
}

func TestConfirmReceiptReturnsRepositoryOutcome(t *testing.T) {
	want := model.Receipt{
		Delivery:    model.Delivery{ID: 15, Status: model.DeliveryReceived},
		ChainStatus: model.ChainCompleted,
	}
	repository := &repositoryStub{receiptResult: want}
	admin := newTestAdmin(t, repository)

	for range 2 {
		got, err := admin.ConfirmReceipt(context.Background(), 8, 12)
		if err != nil {
			t.Fatalf("ConfirmReceipt() error = %v", err)
		}
		if got != want {
			t.Fatalf("ConfirmReceipt() = %+v, want %+v", got, want)
		}
	}
	if repository.receiptCalls != 2 || repository.actorID != 8 || repository.chainID != 12 {
		t.Fatalf("repository calls = %d, actor %d, chain %d", repository.receiptCalls, repository.actorID, repository.chainID)
	}
}

func TestConfirmParticipantReceiptValidatesAndPublishes(t *testing.T) {
	want := model.Receipt{Delivery: model.Delivery{ID: 15, Status: model.DeliveryReceived}, ChainStatus: model.ChainAccepted}
	repository := &repositoryStub{receiptResult: want}
	callbackCalls := 0
	admin, err := NewWithCallback(repository, func(_ context.Context, delivery model.Delivery) {
		callbackCalls++
		if delivery != want.Delivery {
			t.Fatalf("callback delivery = %+v", delivery)
		}
	}, AdminConfig{MaxListLimit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := admin.ConfirmParticipantReceipt(context.Background(), 7, 12, 9); err != nil {
		t.Fatalf("ConfirmParticipantReceipt() error = %v", err)
	}
	if repository.actorID != 7 || repository.chainID != 12 || repository.deliveryID != 9 || callbackCalls != 1 {
		t.Fatalf("forwarded values = actor %d chain %d participant %d callbacks %d", repository.actorID, repository.chainID, repository.deliveryID, callbackCalls)
	}
	if _, err := admin.ConfirmParticipantReceipt(context.Background(), 7, 12, 0); err == nil {
		t.Fatal("zero participant accepted")
	}
}

func newTestAdmin(t *testing.T, repository Repository) *Admin {
	t.Helper()

	admin, err := New(repository, AdminConfig{MaxListLimit: 100})
	if err != nil {
		t.Fatalf("New() error = %v", err)
	}
	return admin
}
