package service

import (
	"context"
	"encoding/json"
	
	"swap-chain/analyze/adapters"
)

type Vectorizer struct {
	enricher adapters.Enricher
	embedder adapters.Embedder
}

func NewVectorizer(enricher adapters.Enricher, embedder adapters.Embedder) *Vectorizer {
	return &Vectorizer{
		enricher: enricher,
		embedder: embedder,
	}
}

func (s *Vectorizer) Vectorize(ctx context.Context, text string) ([]float32, error) {
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
