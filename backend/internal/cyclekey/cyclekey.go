// Package cyclekey canonicalizes directed item cycles.
package cyclekey

import (
	"errors"
	"strconv"
	"strings"
)

// ErrInvalidCycle indicates that edges do not form one closed directed cycle.
var ErrInvalidCycle = errors.New("edges must form one closed directed cycle")

// Edge is one directed item-to-item relation in a cycle.
type Edge struct {
	SourceID int64
	TargetID int64
}

// Canonical returns a rotation-independent key while preserving direction.
func Canonical(edges []Edge) (string, error) {
	if len(edges) < 2 {
		return "", ErrInvalidCycle
	}

	successors := make(map[int64]int64, len(edges))
	targets := make(map[int64]struct{}, len(edges))
	var start int64
	for _, edge := range edges {
		if edge.SourceID <= 0 || edge.TargetID <= 0 || edge.SourceID == edge.TargetID {
			return "", ErrInvalidCycle
		}
		if _, exists := successors[edge.SourceID]; exists {
			return "", ErrInvalidCycle
		}
		if _, exists := targets[edge.TargetID]; exists {
			return "", ErrInvalidCycle
		}
		successors[edge.SourceID] = edge.TargetID
		targets[edge.TargetID] = struct{}{}
		if start == 0 || edge.SourceID < start {
			start = edge.SourceID
		}
	}
	for sourceID := range successors {
		if _, exists := targets[sourceID]; !exists {
			return "", ErrInvalidCycle
		}
	}

	parts := make([]string, 0, len(edges))
	visited := make(map[int64]struct{}, len(edges))
	current := start
	for range edges {
		if _, exists := visited[current]; exists {
			return "", ErrInvalidCycle
		}
		visited[current] = struct{}{}
		parts = append(parts, strconv.FormatInt(current, 10))
		current = successors[current]
	}
	if current != start || len(visited) != len(edges) {
		return "", ErrInvalidCycle
	}
	return strings.Join(parts, ">"), nil
}
