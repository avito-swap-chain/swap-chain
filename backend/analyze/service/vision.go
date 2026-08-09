package service

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"
	"swap-chain/analyze/model"
)

type visionAdapter interface {
	AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error)
}

// Vision converts an image-model response into validated domain data.
type Vision struct {
	adapters visionAdapter
}

// NewVision creates an image analysis service.
func NewVision(adapters visionAdapter) *Vision {
	return &Vision{
		adapters: adapters,
	}
}

type rawResponse struct {
	MarketplaceDescription string  `json:"marketplace_description"`
	VisualQuality          string  `json:"visual_quality"`
	QualityScore           float64 `json:"quality_score"`
}

// DescribeImage анализирует фотографию товара и вытаскивает из нее данные для карточки
func (s *Vision) DescribeImage(ctx context.Context, imageBytes []byte) (*model.VisualAnalysis, error) {
	resp, err := s.adapters.AnalyzePhoto(ctx, imageBytes, VisionAnalysisPrompt)
	if err != nil {
		return nil, fmt.Errorf("describe image - adapter: %w", err)
	}

	var rawResponse rawResponse
	if err := json.Unmarshal([]byte(resp), &rawResponse); err != nil {
		return nil, fmt.Errorf("describe image: raw response - unmarshalling: %w", err)
	}

	quality, err := model.ParseQuality(rawResponse.VisualQuality)
	if err != nil {
		return nil, fmt.Errorf("describe image: invalid quality from model: %w", err)
	}
	if strings.TrimSpace(rawResponse.MarketplaceDescription) == "" {
		return nil, fmt.Errorf("describe image: marketplace description is empty")
	}
	if rawResponse.QualityScore < 0 || rawResponse.QualityScore > 1 {
		return nil, fmt.Errorf("describe image: quality score %.4f is outside [0,1]", rawResponse.QualityScore)
	}

	return &model.VisualAnalysis{
		MarketplaceDescription: rawResponse.MarketplaceDescription,
		VisualQuality:          quality,
		QualityScore:           rawResponse.QualityScore,
	}, nil
}
