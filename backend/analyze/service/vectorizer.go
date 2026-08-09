package service

import (
	"context"
	"encoding/json"
)

type LLMClient interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

type Vectorizer struct {
	client LLMClient
}

func NewVectorizer(client LLMClient) *Vectorizer {
	return &Vectorizer{
		client: client,
	}
}

func (s *Vectorizer) Vectorize(ctx context.Context, text string) ([]float32, error) {
	prompt := BuildEnrichmentPrompt(text)
	enrichedJSON, err := s.client.GenerateJSON(ctx, prompt)
	if err != nil {
		return s.client.Vectorize(ctx, text)
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(enrichedJSON), &result); err != nil {
		return s.client.Vectorize(ctx, text)
	}

	enrichedText := result.Value
	if enrichedText == "" {
		enrichedText = text
	}

	return s.client.Vectorize(ctx, enrichedText)
}
