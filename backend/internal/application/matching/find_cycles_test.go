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

type registryStub struct {
	known map[string]struct{}
	err   error
	keys  []string
}

func (stub *registryStub) KnownCycleKeys(_ context.Context, keys []string) (map[string]struct{}, error) {
	stub.keys = append([]string(nil), keys...)
	return stub.known, stub.err
}

func (stub *matcherStub) FindCycles(_ context.Context, itemID int) ([][]model.Edge, error) {
	stub.itemID = itemID
	return stub.cycles, stub.err
}

func TestFindCyclesRejectsInvalidItemID(t *testing.T) {
	useCase := NewFindCycles(&matcherStub{}, &registryStub{})

	_, err := useCase.Execute(context.Background(), 0)
	if !errors.Is(err, ErrInvalidItemID) {
		t.Fatalf("Execute() error = %v, want ErrInvalidItemID", err)
	}
}

func TestFindCyclesDelegatesToMatcher(t *testing.T) {
	want := [][]model.Edge{{
		{SourceID: 1, TargetID: 2, Score: 0.8},
		{SourceID: 2, TargetID: 1, Score: 0.9},
	}}
	stub := &matcherStub{cycles: want}
	registry := &registryStub{known: map[string]struct{}{}}
	useCase := NewFindCycles(stub, registry)

	got, err := useCase.Execute(context.Background(), 42)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if stub.itemID != 42 {
		t.Fatalf("matcher item ID = %d, want 42", stub.itemID)
	}
	if len(got) != 1 || len(got[0]) != 2 || got[0][0] != want[0][0] {
		t.Fatalf("Execute() = %#v, want %#v", got, want)
	}
	if len(registry.keys) != 1 || registry.keys[0] != "1>2" {
		t.Fatalf("registry keys = %#v, want [1>2]", registry.keys)
	}
}

func TestFindCyclesExcludesPersistedAndDuplicateCycles(t *testing.T) {
	t.Parallel()

	forward := []model.Edge{{SourceID: 1, TargetID: 2}, {SourceID: 2, TargetID: 3}, {SourceID: 3, TargetID: 1}}
	rotated := []model.Edge{{SourceID: 2, TargetID: 3}, {SourceID: 3, TargetID: 1}, {SourceID: 1, TargetID: 2}}
	reverse := []model.Edge{{SourceID: 1, TargetID: 3}, {SourceID: 3, TargetID: 2}, {SourceID: 2, TargetID: 1}}
	useCase := NewFindCycles(
		&matcherStub{cycles: [][]model.Edge{forward, rotated, reverse}},
		&registryStub{known: map[string]struct{}{"1>2>3": {}}},
	)

	got, err := useCase.Execute(context.Background(), 1)
	if err != nil {
		t.Fatalf("Execute() error = %v", err)
	}
	if len(got) != 1 || len(got[0]) != 3 || got[0][0] != reverse[0] {
		t.Fatalf("Execute() = %#v, want only reverse cycle", got)
	}
}
