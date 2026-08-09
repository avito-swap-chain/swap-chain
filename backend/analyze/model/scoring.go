// Package model contains values produced by item analysis.
package model

// DescriptionScore contains normalized quality metrics for an item description.
type DescriptionScore struct {
	ParamRichness float64 `json:"param_richness"`
}
