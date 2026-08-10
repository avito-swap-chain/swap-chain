package model

type CategoryMatch struct {
	CategoryID int32
	Confidence float64
	IsManual   bool // назначается, если система не уверена в категории
}

type CategoryCandidate struct {
	ID         int32
	Name       string
	Similarity float64
}
