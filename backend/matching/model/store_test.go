package model

import (
	"errors"
	"testing"
)

func TestInMemoryStoreVertexLifecycle(t *testing.T) {
	store := NewMemoryStore()
	vertex := Vertex{ItemID: 10}

	if err := store.AddVertex(vertex); err != nil {
		t.Fatalf("add vertex: %v", err)
	}
	if got := store.VerticesAmount(); got != 1 {
		t.Fatalf("vertices amount: got %d, want 1", got)
	}

	got, err := store.Vertex(vertex.ItemID)
	if err != nil {
		t.Fatalf("get vertex: %v", err)
	}
	if got != vertex {
		t.Fatalf("vertex: got %+v, want %+v", got, vertex)
	}

	if err := store.AddVertex(vertex); !errors.Is(err, ErrVertexAlreadyExists) {
		t.Fatalf("duplicate vertex error: got %v, want %v", err, ErrVertexAlreadyExists)
	}
	if _, err := store.Vertex(999); !errors.Is(err, ErrVertexNotFound) {
		t.Fatalf("missing vertex error: got %v, want %v", err, ErrVertexNotFound)
	}
}

func TestInMemoryStoreEdgeLifecycle(t *testing.T) {
	store := NewMemoryStore()
	for _, itemID := range []int{1, 2} {
		if err := store.AddVertex(Vertex{ItemID: itemID}); err != nil {
			t.Fatalf("add vertex %d: %v", itemID, err)
		}
	}

	edge := Edge{ID: 7, SourceID: 1, TargetID: 2, Score: 0.85}
	if err := store.AddEdge(edge); err != nil {
		t.Fatalf("add edge: %v", err)
	}
	if got := store.EdgesAmount(); got != 1 {
		t.Fatalf("edges amount: got %d, want 1", got)
	}

	got, err := store.Edge(edge.ID)
	if err != nil {
		t.Fatalf("get edge: %v", err)
	}
	if got != edge {
		t.Fatalf("edge: got %+v, want %+v", got, edge)
	}

	duplicate := Edge{ID: 8, SourceID: 1, TargetID: 2, Score: 0.9}
	if err := store.AddEdge(duplicate); !errors.Is(err, ErrEdgeAlreadyExists) {
		t.Fatalf("duplicate edge error: got %v, want %v", err, ErrEdgeAlreadyExists)
	}
	if _, err := store.Edge(999); !errors.Is(err, ErrEdgeNotFound) {
		t.Fatalf("missing edge error: got %v, want %v", err, ErrEdgeNotFound)
	}
}

func TestInMemoryStoreAddEdgeRequiresExistingVertices(t *testing.T) {
	store := NewMemoryStore()
	if err := store.AddVertex(Vertex{ItemID: 1}); err != nil {
		t.Fatalf("add source vertex: %v", err)
	}

	err := store.AddEdge(Edge{ID: 1, SourceID: 1, TargetID: 2})
	if !errors.Is(err, ErrVertexNotFound) {
		t.Fatalf("add edge error: got %v, want %v", err, ErrVertexNotFound)
	}
}

func TestInMemoryStoreAdjacencyMapIsDetached(t *testing.T) {
	store := graphStore(t, []Edge{{ID: 1, SourceID: 1, TargetID: 2, Score: 0.8}})

	adjacency, err := store.AdjacencyMap()
	if err != nil {
		t.Fatalf("adjacency map: %v", err)
	}
	adjacency[1][0].Score = 0
	delete(adjacency, 1)

	edge, err := store.Edge(1)
	if err != nil {
		t.Fatalf("get stored edge: %v", err)
	}
	if edge.Score != 0.8 {
		t.Fatalf("stored score changed through adjacency copy: got %v", edge.Score)
	}
}

func TestInMemoryStoreFindCyclesFindsSimpleCycleOnce(t *testing.T) {
	store := graphStore(t, []Edge{
		{ID: 1, SourceID: 1, TargetID: 2, Score: 0.9},
		{ID: 2, SourceID: 2, TargetID: 3, Score: 0.8},
		{ID: 3, SourceID: 3, TargetID: 1, Score: 0.7},
	})

	cycles, err := store.FindCycles(3)
	if err != nil {
		t.Fatalf("find cycles: %v", err)
	}
	if len(cycles) != 1 {
		t.Fatalf("cycles amount: got %d, want 1", len(cycles))
	}
	if len(cycles[0]) != 3 {
		t.Fatalf("cycle length: got %d, want 3", len(cycles[0]))
	}

	wantIDs := []int{1, 2, 3}
	for i, edge := range cycles[0] {
		if edge.ID != wantIDs[i] {
			t.Fatalf("edge at index %d: got ID %d, want %d", i, edge.ID, wantIDs[i])
		}
	}
}

func TestInMemoryStoreFindCyclesRespectsMaxDepth(t *testing.T) {
	store := graphStore(t, []Edge{
		{ID: 1, SourceID: 1, TargetID: 2},
		{ID: 2, SourceID: 2, TargetID: 3},
		{ID: 3, SourceID: 3, TargetID: 1},
	})

	cycles, err := store.FindCycles(2)
	if err != nil {
		t.Fatalf("find cycles: %v", err)
	}
	if len(cycles) != 0 {
		t.Fatalf("cycles amount: got %d, want 0", len(cycles))
	}
}

func graphStore(t *testing.T, edges []Edge) *InMemoryStore {
	t.Helper()

	store := NewMemoryStore()
	vertices := make(map[int]struct{})
	for _, edge := range edges {
		vertices[edge.SourceID] = struct{}{}
		vertices[edge.TargetID] = struct{}{}
	}
	for itemID := range vertices {
		if err := store.AddVertex(Vertex{ItemID: itemID}); err != nil {
			t.Fatalf("add vertex %d: %v", itemID, err)
		}
	}
	for _, edge := range edges {
		if err := store.AddEdge(edge); err != nil {
			t.Fatalf("add edge %+v: %v", edge, err)
		}
	}
	return store
}
