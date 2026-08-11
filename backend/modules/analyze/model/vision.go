package model

import "fmt"

type Quality string

const (
	New       Quality = "NEW"
	Excellent Quality = "EXCELLENT"
	Good      Quality = "GOOD"
	Fair      Quality = "FAIR"
	Poor      Quality = "POOR"
)

func ParseQuality(s string) (Quality, error) {
	switch Quality(s) {
	case New, Excellent, Good, Fair, Poor:
		return Quality(s), nil
	default:
		return "", fmt.Errorf("unknown visual quality: %q", s)
	}
}

type VisualAnalysis struct {
	MarketplaceDescription string
	SuggestedCategory      string
	VisualQuality          Quality
	QualityScore           float64
}
