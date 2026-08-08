package model

type Graph interface {
	AddVertex(vertex Vertex) error
	Vertex(itemID int) (Vertex, error)
	AddEdge(edge Edge) error
	Edge(edgeID int) (Edge, error)

	VerticesAmount() int
	EdgesAmount() int
	AdjacencyMap() (map[int][]Edge, error)

	// FindCycles ищёт все возможные циклы обменов.
	FindCycles(maxDepth int) ([][]Edge, error)
}

type Vertex struct {
	ItemID   int
}

type Edge struct {
	ID       int
	SourceID int
	TargetID int
	Score    float64
}
