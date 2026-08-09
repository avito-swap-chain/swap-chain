package adapters

import (
	"context"
	"fmt"
	"net/url"
	"strings"
)

type Enricher interface {
	GenerateJSON(ctx context.Context, prompt string) (string, error)
}

type Embedder interface {
	Vectorize(ctx context.Context, text string) ([]float32, error)
}

type FallbackClient struct {
	primary  Enricher
	fallback Enricher
}

func NewFallbackClient(primary Enricher, fallback Enricher) (*FallbackClient, error) {
	switch {
	case primary == nil:
		return nil, fmt.Errorf("fallback client init: 'primary enricher' is required")
	case fallback == nil:
		return nil, fmt.Errorf("fallback client init: 'fallback enricher' is required")
	}

	return &FallbackClient{
		primary:  primary,
		fallback: fallback,
	}, nil
}

func (f *FallbackClient) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	res, err := f.primary.GenerateJSON(ctx, prompt)
	if err != nil {
		return f.fallback.GenerateJSON(ctx, prompt)
	}

	return res, nil
}

func validateHTTPURL(field string, rawURL string) error {
	if strings.TrimSpace(rawURL) == "" {
		return fmt.Errorf("%s is required", field)
	}

	parsedURL, err := url.Parse(rawURL)
	if err != nil {
		return fmt.Errorf("parse %s: %w", field, err)
	}
	if (parsedURL.Scheme != "http" && parsedURL.Scheme != "https") || parsedURL.Host == "" {
		return fmt.Errorf("%s must be an absolute HTTP URL", field)
	}

	return nil
}
