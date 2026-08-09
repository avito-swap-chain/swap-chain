package model

// CategoryMatch represents the best category candidate or a manual-selection request.
type CategoryMatch struct {
	CategoryID int
	Confidence float64
	IsManual   bool // назначается, если система не уверена в категории
}
