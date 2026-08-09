package service

import (
	"context"
	"encoding/json"
	"fmt"

	"swap-chain/analyze/adapters"
	"swap-chain/analyze/model"
)

type Scoring struct {
	enricher adapters.Enricher
}

func NewScoring(enricher adapters.Enricher) (*Scoring, error) {
	if enricher == nil {
		return nil, fmt.Errorf("scoring init: 'enricher' is required")
	}

	return &Scoring{
		enricher: enricher,
	}, nil
}

// EvaluateDescription оценивает качество пользовательского описания товара
func (s *Scoring) EvaluateDescription(ctx context.Context, description string) (*model.DescriptionScore, error) {
	prompt := BuildParamRichnessPrompt(description)

	resp, err := s.enricher.GenerateJSON(ctx, prompt)
	if err != nil {
		return nil, fmt.Errorf("evaluate description: %w", err)
	}

	var score model.DescriptionScore
	if err := json.Unmarshal([]byte(resp), &score); err != nil {
		return nil, fmt.Errorf("evaluate description JSON parse error: %w", err)
	}

	return &score, nil
}
