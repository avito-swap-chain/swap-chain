package matching

import (
	"context"
	"errors"
	"testing"

	"swap-chain/matching/model"
)

type matcherStub struct {
	cycles [][]model.Edge
	err    error
	itemID int
}

func (stub *matcherStub) FindCycles(_ context.Context, itemID int) ([][]model.Edge, error) {
	stub.itemID = itemID
	return stub.cycles, stub.err
}

func TestFindCyclesRejectsInvalidItemID(t *testing.T) {
	useCase := NewFindCycles(&matcherStub{})

	_, err := useCase.Execute(context.Background(), 0)
	if !errors.Is(err, ErrInvalidItemID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidItemID", err)
	}
}

func TestFindCyclesDelegatesToMatcher(t *testing.T) {
	want := [][]model.Edge{{{SourceID: 1, TargetID: 2, Score: 0.8}}}
	stub := &matcherStub{cycles: want}
	useCase := NewFindCycles(stub)

	got, err := useCase.Execute(context.Background(), 42)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stub.itemID != 42 {
		t.Fatalf("matcher item ID = %d, want 42", stub.itemID)
	}
	if len(got) != 1 || len(got[0]) != 1 || got[0][0] != want[0][0] {
		t.Fatalf("Execute() = %#v, want %#v", got, want)
	}
}
