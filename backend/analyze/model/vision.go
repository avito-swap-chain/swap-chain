package model

import "fmt"

// Quality describes the visible condition of an item.
type Quality string

const (
	// New means the item has no visible signs of use.
	New Quality = "NEW"
	// Excellent means the item has minimal visible signs of use.
	Excellent Quality = "EXCELLENT"
	// Good means the item has normal visible signs of use.
	Good Quality = "GOOD"
	// Fair means the item has significant wear but remains usable.
	Fair Quality = "FAIR"
	// Poor means the item is heavily worn or visibly damaged.
	Poor Quality = "POOR"
)

// ParseQuality validates and converts an LLM quality value.
func ParseQuality(s string) (Quality, error) {
	switch Quality(s) {
	case New, Excellent, Good, Fair, Poor:
		return Quality(s), nil
	default:
		return "", fmt.Errorf("unknown visual quality: %q", s)
	}
}

// VisualAnalysis contains structured attributes extracted from an image.
type VisualAnalysis struct {
	MarketplaceDescription string
	VisualQuality          Quality
	QualityScore           float64
}
