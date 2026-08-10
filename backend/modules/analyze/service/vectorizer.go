package service

import (
	"context"
	"encoding/json"
	"fmt"

	"swap-chain/modules/analyze/adapters"
)

type Vectorizer struct {
	enricher adapters.Enricher
	embedder adapters.Embedder
}

func NewVectorizer(enricher adapters.Enricher, embedder adapters.Embedder) (*Vectorizer, error) {
	switch {
	case enricher == nil:
		return nil, fmt.Errorf("vectorizer init: 'enricher' is required")
	case embedder == nil:
		return nil, fmt.Errorf("vectorizer init: 'embedder' is required")
	}

	return &Vectorizer{
		enricher: enricher,
		embedder: embedder,
	}, nil
}

func (s *Vectorizer) EnrichAndVectorize(ctx context.Context, text string) ([]float32, error) {
	prompt := BuildEnrichmentPrompt(text)
	enrichedJSON, err := s.enricher.GenerateJSON(ctx, prompt)
	if err != nil {
		return s.embedder.Vectorize(ctx, text)
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(enrichedJSON), &result); err != nil {
		return s.embedder.Vectorize(ctx, text)
	}

	enrichedText := result.Value
	if enrichedText == "" {
		enrichedText = text
	}

	return s.embedder.Vectorize(ctx, enrichedText)
}
