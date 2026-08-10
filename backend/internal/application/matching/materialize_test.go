package matching

import (
	"context"
	"errors"
	"testing"

	"swap-chain/internal/chains"
	"swap-chain/internal/items"
	matchingmodel "swap-chain/modules/matching/model"
)

type materializeItemStub struct {
	item items.Item
	err  error
}

func (stub materializeItemStub) Get(context.Context, int64) (items.Item, error) {
	return stub.item, stub.err
}

type materializeFinderStub struct {
	cycles [][]matchingmodel.Edge
	err    error
	calls  int
}

func (stub *materializeFinderStub) Execute(context.Context, int64) ([][]matchingmodel.Edge, error) {
	stub.calls++
	return stub.cycles, stub.err
}

type materializeChainStub struct {
	userIDs []int64
	inputs  []chains.CreateInput
	errors  []error
}

func (stub *materializeChainStub) Create(
	_ context.Context,
	userID int64,
	input chains.CreateInput,
) (chains.Chain, error) {
	stub.userIDs = append(stub.userIDs, userID)
	stub.inputs = append(stub.inputs, input)
	index := len(stub.inputs) - 1
	if index < len(stub.errors) && stub.errors[index] != nil {
		return chains.Chain{}, stub.errors[index]
	}
	return chains.Chain{ID: int64(index + 1)}, nil
}

func TestMaterializerPersistsMatchingCyclesForItemOwner(t *testing.T) {
	finder := &materializeFinderStub{cycles: [][]matchingmodel.Edge{{
		{SourceID: 5, TargetID: 6},
		{SourceID: 6, TargetID: 5},
	}}}
	creator := &materializeChainStub{}
	materializer, err := NewMaterializer(
		materializeItemStub{item: items.Item{ID: 5, UserID: 2, Status: "MATCHING"}},
		finder,
		creator,
	)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}

	created, err := materializer.Materialize(context.Background(), 5)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if created != 1 {
		t.Fatalf("created = %d, want 1", created)
	}
	if len(creator.inputs) != 1 || len(creator.inputs[0].Edges) != 2 {
		t.Fatalf("created inputs = %#v", creator.inputs)
	}
	if creator.userIDs[0] != 2 {
		t.Fatalf("creator user ID = %d, want 2", creator.userIDs[0])
	}
	if creator.inputs[0].Edges[0] != (chains.Edge{SourceItemID: 5, TargetItemID: 6}) {
		t.Fatalf("first edge = %#v", creator.inputs[0].Edges[0])
	}
}

func TestMaterializerTreatsConcurrentDuplicateAsSuccess(t *testing.T) {
	finder := &materializeFinderStub{cycles: [][]matchingmodel.Edge{{
		{SourceID: 5, TargetID: 6},
		{SourceID: 6, TargetID: 5},
	}}}
	creator := &materializeChainStub{errors: []error{chains.ErrConflict}}
	materializer, err := NewMaterializer(
		materializeItemStub{item: items.Item{ID: 5, UserID: 2, Status: "MATCHING"}},
		finder,
		creator,
	)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}

	created, err := materializer.Materialize(context.Background(), 5)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if created != 0 {
		t.Fatalf("created = %d, want 0", created)
	}
}

func TestMaterializerSkipsItemOutsideMatchingState(t *testing.T) {
	finder := &materializeFinderStub{}
	materializer, err := NewMaterializer(
		materializeItemStub{item: items.Item{ID: 5, UserID: 2, Status: "LOCKED"}},
		finder,
		&materializeChainStub{},
	)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}

	created, err := materializer.Materialize(context.Background(), 5)
	if err != nil {
		t.Fatalf("Materialize() error = %v", err)
	}
	if created != 0 || finder.calls != 0 {
		t.Fatalf("created = %d, finder calls = %d; want both zero", created, finder.calls)
	}
}

func TestMaterializerReturnsChainPersistenceError(t *testing.T) {
	persistErr := errors.New("database unavailable")
	finder := &materializeFinderStub{cycles: [][]matchingmodel.Edge{{
		{SourceID: 5, TargetID: 6},
		{SourceID: 6, TargetID: 5},
	}}}
	materializer, err := NewMaterializer(
		materializeItemStub{item: items.Item{ID: 5, UserID: 2, Status: "MATCHING"}},
		finder,
		&materializeChainStub{errors: []error{persistErr}},
	)
	if err != nil {
		t.Fatalf("NewMaterializer() error = %v", err)
	}

	_, err = materializer.Materialize(context.Background(), 5)
	if !errors.Is(err, persistErr) {
		t.Fatalf("Materialize() error = %v, want %v", err, persistErr)
	}
}
