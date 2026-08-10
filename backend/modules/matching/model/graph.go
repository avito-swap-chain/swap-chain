package model

type Graph interface {
	AddVertex(vertex Vertex) error
	Vertex(itemID int64) (Vertex, error)
	AddEdge(edge Edge) error
	Edge(edgeID int) (Edge, error)

	VerticesAmount() int
	EdgesAmount() int
	AdjacencyMap() (map[int64][]Edge, error)

	// FindCycles ищёт все возможные циклы обменов.
	FindCycles(maxDepth int) ([][]Edge, error)
}

type Vertex struct {
	ItemID int64
}

type Edge struct {
	ID       int
	SourceID int64
	TargetID int64
	Score    float64
}
