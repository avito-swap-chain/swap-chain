package model

type CategoryMatch struct {
	CategoryID    int32
	RequiresInput bool
}

type CategoryCandidate struct {
	ID         int32
	Name       string
	Similarity float64
}
