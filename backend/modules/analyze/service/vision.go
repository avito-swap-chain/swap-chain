package service

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	"swap-chain/modules/analyze/model"
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
	MarketplaceDescription string   `json:"marketplace_description"`
	SuggestedCategory      string   `json:"suggested_category"`
	VisualQuality          string   `json:"visual_quality"`
	QualityScore           *float64 `json:"quality_score"`
}

var visionCategories = map[string]struct{}{
	"Электроника":        {},
	"Аудио":              {},
	"Спорт и отдых":      {},
	"Транспорт":          {},
	"Одежда и обувь":     {},
	"Дом и дача":         {},
	"Хобби и творчество": {},
}

// DescribeImage анализирует фотографию товара и вытаскивает из нее данные для карточки
func (s *Vision) DescribeImage(ctx context.Context, imageBytes []byte) (*model.VisualAnalysis, error) {
	resp, err := s.adapter.AnalyzePhoto(ctx, imageBytes, VisionAnalysisPrompt)
	if err != nil {
		return nil, fmt.Errorf("describe image - adapter: %w", err)
	}

	var rawResponse RawResponse
	decoder := json.NewDecoder(bytes.NewBufferString(resp))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&rawResponse); err != nil {
		return nil, fmt.Errorf("describe image: raw response - unmarshalling: %w", err)
	}
	if err := ensureJSONEnd(decoder); err != nil {
		return nil, fmt.Errorf("describe image: raw response - trailing data: %w", err)
	}
	if strings.TrimSpace(rawResponse.MarketplaceDescription) == "" {
		return nil, fmt.Errorf("describe image: marketplace description is empty")
	}
	if _, ok := visionCategories[rawResponse.SuggestedCategory]; !ok {
		return nil, fmt.Errorf("describe image: unknown suggested category %q", rawResponse.SuggestedCategory)
	}
	if rawResponse.QualityScore == nil {
		return nil, fmt.Errorf("describe image: quality score is missing")
	}

	quality, err := model.ParseQuality(rawResponse.VisualQuality)
	if err != nil {
		return nil, fmt.Errorf("describe image: invalid quality from model: %w", err)
	}
	if *rawResponse.QualityScore < 0 || *rawResponse.QualityScore > 1 {
		return nil, fmt.Errorf("describe image: quality score %v is outside [0,1]", *rawResponse.QualityScore)
	}

	return &model.VisualAnalysis{
		MarketplaceDescription: rawResponse.MarketplaceDescription,
		SuggestedCategory:      rawResponse.SuggestedCategory,
		VisualQuality:          quality,
		QualityScore:           *rawResponse.QualityScore,
	}, nil
}

func ensureJSONEnd(decoder *json.Decoder) error {
	var extra any
	if err := decoder.Decode(&extra); errors.Is(err, io.EOF) {
		return nil
	} else if err != nil {
		return err
	}
	return fmt.Errorf("multiple JSON values")
}
