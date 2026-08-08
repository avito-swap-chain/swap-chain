package model

import (
	"sync"
)

type InMemoryStore struct {
	lock     sync.RWMutex
	vertices map[int]Vertex
	edges    map[int]Edge // edgeID -> Edge

	outEdges map[int]map[int]Edge // source -> target
}

func NewMemoryStore() *InMemoryStore {
	return &InMemoryStore{
		vertices: make(map[int]Vertex),
		edges:    make(map[int]Edge),
		outEdges: make(map[int]map[int]Edge),
	}
}

func (s *InMemoryStore) AddVertex(vertex Vertex) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.vertices[vertex.ItemID]; ok {
		return ErrVertexAlreadyExists
	}

	s.vertices[vertex.ItemID] = vertex
	s.outEdges[vertex.ItemID] = make(map[int]Edge)

	return nil
}

func (s *InMemoryStore) Vertex(itemID int) (Vertex, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	vertex, ok := s.vertices[itemID]
	if !ok {
		return Vertex{}, ErrVertexNotFound
	}
	return vertex, nil
}

func (s *InMemoryStore) AddEdge(edge Edge) error {
	s.lock.Lock()
	defer s.lock.Unlock()

	if _, ok := s.vertices[edge.SourceID]; !ok {
		return ErrVertexNotFound
	}
	if _, ok := s.vertices[edge.TargetID]; !ok {
		return ErrVertexNotFound
	}

	if _, ok := s.outEdges[edge.SourceID][edge.TargetID]; ok {
		return ErrEdgeAlreadyExists
	}

	s.edges[edge.ID] = edge
	s.outEdges[edge.SourceID][edge.TargetID] = edge
	return nil
}

func (s *InMemoryStore) Edge(edgeID int) (Edge, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	edge, ok := s.edges[edgeID]
	if !ok {
		return Edge{}, ErrEdgeNotFound
	}
	return edge, nil
}

func (s *InMemoryStore) VerticesAmount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	
	return len(s.vertices)
}

func (s *InMemoryStore) EdgesAmount() int {
	s.lock.RLock()
	defer s.lock.RUnlock()
	
	return len(s.edges)
}

func (s *InMemoryStore) AdjacencyMap() (map[int][]Edge, error) {
	s.lock.RLock()
	defer s.lock.RUnlock()

	adjMap := make(map[int][]Edge)
	for sourceID, targets := range s.outEdges {
		edgesSlice := make([]Edge, 0, len(targets))
		for _, edge := range targets {
			edgesSlice = append(edgesSlice, edge)
		}
		adjMap[sourceID] = edgesSlice
	}

	return adjMap, nil
}

func (s *InMemoryStore) FindCycles(maxDepth int) ([][]Edge, error) {
	adjMap, err := s.AdjacencyMap()
	if err != nil {
		return nil, err
	}

	var allCycles [][]Edge

	var dfs func(currentNode, startNode, depth int, path []Edge, visited map[int]bool)
	dfs = func(currentNode, startNode, depth int, path []Edge, visited map[int]bool) {
		if depth > maxDepth {
			return
		}

		for _, edge := range adjMap[currentNode] {
			neighbor := edge.TargetID

			if neighbor < startNode {
				continue
			}
			
			if neighbor == startNode {
				cycle := make([]Edge, len(path)+1)
				copy(cycle, path)
				cycle[len(path)] = edge
				allCycles = append(allCycles, cycle)
				continue
			}

			if !visited[neighbor] {
				visited[neighbor] = true
				path = append(path, edge)

				dfs(neighbor, startNode, depth+1, path, visited)

				path = path[:len(path)-1]
				visited[neighbor] = false
			}
		}
	}

	for node := range adjMap {
		visited := make(map[int]bool)
		visited[node] = true
		dfs(node, node, 1, []Edge{}, visited)
	}

	return allCycles, nil
}
