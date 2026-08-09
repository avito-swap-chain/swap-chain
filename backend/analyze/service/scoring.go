package service

import (
	"context"
	"encoding/json"
	"fmt"
	"swap-chain/analyze/model"
)

type jsonGenerator interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
}

// Scoring evaluates normalized item-description metrics.
type Scoring struct {
	generator jsonGenerator
}

// NewScoring creates a description scoring service.
func NewScoring(generator jsonGenerator) *Scoring {
	return &Scoring{
		generator: generator,
	}
}

// EvaluateDescription оценивает качество пользовательского описания товара
func (s *Scoring) EvaluateDescription(ctx context.Context, description string) (*model.DescriptionScore, error) {
	prompt := BuildParamRichnessPrompt(description)

	resp, err := s.generator.GenerateJSON(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("evaluate description: %w", err)
	}

	var score model.DescriptionScore
	if err := json.Unmarshal([]byte(resp), &score); err != nil {
		return nil, fmt.Errorf("evaluate description JSON parse error: %w", err)
	}

	return &score, nil
}
