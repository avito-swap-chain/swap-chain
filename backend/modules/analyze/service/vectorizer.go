package service

import (
	"context"
	"encoding/json"
	"fmt"

	"swap-chain/modules/analyze/adapters"

	"go.uber.org/zap"
)

type Vectorizer struct {
	enricher         adapters.Enricher
	localEmbedder    adapters.Embedder
	externalEmbedder adapters.Embedder
	logger           *zap.Logger
}

func NewVectorizer(
	enricher adapters.Enricher,
	localEmbedder adapters.Embedder,
	externalEmbedder adapters.Embedder,
	logger *zap.Logger,
) (*Vectorizer, error) {
	switch {
	case enricher == nil:
		return nil, fmt.Errorf("vectorizer init: 'enricher' is required")
	case localEmbedder == nil:
		return nil, fmt.Errorf("vectorizer init: 'local embedder' is required")
	case externalEmbedder == nil:
		return nil, fmt.Errorf("vectorizer init: 'external embedder' is required")
	case logger == nil:
		return nil, fmt.Errorf("vectorizer init: 'logger' is required")
	}

	return &Vectorizer{
		enricher:         enricher,
		localEmbedder:    localEmbedder,
		externalEmbedder: externalEmbedder,
		logger:           logger,
	}, nil
}

func (s *Vectorizer) EnrichAndVectorize(ctx context.Context, text string) ([]float32, []float32, error) {
	prompt := BuildEnrichmentPrompt(text)
	enrichedJSON, err := s.enricher.GenerateJSON(ctx, prompt)
	if err != nil {
		s.logger.Warn("enrich and vectorize: enrich failed", zap.Error(err))
		return s.doVectorize(ctx, text)
	}

	var result struct {
		Value string `json:"value"`
	}
	if err := json.Unmarshal([]byte(enrichedJSON), &result); err != nil {
		s.logger.Warn("enrich and vectorize: unmarshaling failed", zap.Error(err))
		return s.doVectorize(ctx, text)
	}

	enrichedText := result.Value
	if enrichedText == "" {
		enrichedText = text
	}

	return s.doVectorize(ctx, enrichedText)
}

func (s *Vectorizer) doVectorize(ctx context.Context, text string) ([]float32, []float32, error) {
	var local, external []float32
	var err error

	local, err = s.localEmbedder.Vectorize(ctx, text)
	if err != nil {
		return nil, nil, fmt.Errorf("do vectorize: local vectorize: %w", err)
	}

	if s.externalEmbedder != nil {
		external, err = s.externalEmbedder.Vectorize(ctx, text)
		if err != nil {
			s.logger.Warn("do vectorize: external embedder error, fallback to local", zap.Error(err))
		}
	}

	return local, external, nil
}
