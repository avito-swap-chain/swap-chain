package model

type CategoryMatch struct {
	CategoryID int
	Confidence float64
	IsManual   bool // назначается, если система не уверена в категории
}
