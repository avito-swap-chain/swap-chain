package service

import (
	"context"
	"encoding/json"
	"fmt"

	"swap-chain/analyze/model"
)

type VisionAdapter interface {
	AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error)
}

type Vision struct {
	adapter VisionAdapter
}

func NewVision(adapter VisionAdapter) (*Vision, error) {
	if adapter == nil {
		return nil, fmt.Errorf("vision init: 'adapter' is required")
	}

	return &Vision{adapter: adapter}, nil
}

type RawResponse struct {
	MarketplaceDescription string  `json:"marketplace_description"`
	VisualQuality          string  `json:"visual_quality"`
	QualityScore           float64 `json:"quality_score"`
}

// DescribeImage анализирует фотографию товара и вытаскивает из нее данные для карточки
func (s *Vision) DescribeImage(ctx context.Context, imageBytes []byte) (*model.VisualAnalysis, error) {
	resp, err := s.adapter.AnalyzePhoto(ctx, imageBytes, VisionAnalysisPrompt)
	if err != nil {
		return nil, fmt.Errorf("describe image - adapter: %w", err)
	}

	fmt.Println("СЫРОЙ ОТВЕТ: ", resp)

	var rawResponse RawResponse
	if err := json.Unmarshal([]byte(resp), &rawResponse); err != nil {
		return nil, fmt.Errorf("describe image: raw response - unmarshalling: %w", err)
	}

	quality, err := model.ParseQuality(rawResponse.VisualQuality)
	if err != nil {
		return nil, fmt.Errorf("describe image: invalid quality from model: %w", err)
	}
	if rawResponse.QualityScore < 0 || rawResponse.QualityScore > 1 {
		return nil, fmt.Errorf("describe image: quality score %v is outside [0,1]", rawResponse.QualityScore)
	}

	return &model.VisualAnalysis{
		MarketplaceDescription: rawResponse.MarketplaceDescription,
		VisualQuality:          quality,
		QualityScore:           rawResponse.QualityScore,
	}, nil
}
