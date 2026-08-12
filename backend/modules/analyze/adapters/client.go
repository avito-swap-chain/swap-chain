package adapters

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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
	if err == nil {
		if normalized, ok := normalizeJSON(res); ok {
			return normalized, nil
		}
		err = errors.New("primary enricher returned invalid JSON")
	}

	if err != nil {
		log.Printf("WARN: primary enricher failed, falling back to Ollama... Error: %v", err)
	}

	fallbackResult, fallbackErr := f.fallback.GenerateJSON(ctx, prompt)
	if fallbackErr != nil {
		return "", errors.Join(
			fmt.Errorf("primary enricher: %w", err),
			fmt.Errorf("fallback enricher: %w", fallbackErr),
		)
	}

	if normalized, ok := normalizeJSON(fallbackResult); ok {
		return normalized, nil
	}
	return "", fmt.Errorf("fallback enricher returned invalid JSON: %s", fallbackResult)
}

type VisionAnalyzer interface {
	AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error)
}

type VisionFallbackClient struct {
	primary  VisionAnalyzer
	fallback VisionAnalyzer
}

func NewVisionFallbackClient(primary VisionAnalyzer, fallback VisionAnalyzer) (*VisionFallbackClient, error) {
	if primary == nil {
		return nil, fmt.Errorf("vision fallback init: 'primary' is required")
	}
	if fallback == nil {
		return nil, fmt.Errorf("vision fallback init: 'fallback' is required")
	}
	return &VisionFallbackClient{
		primary:  primary,
		fallback: fallback,
	}, nil
}

func (f *VisionFallbackClient) AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error) {
	res, err := f.primary.AnalyzePhoto(ctx, photoBytes, prompt)
	if err == nil {
		if normalized, ok := normalizeJSON(res); ok {
			return normalized, nil
		}
		err = errors.New("primary vision returned invalid JSON")
	}

	if err != nil {
		log.Printf("WARN: primary vision failed, falling back... Error: %v", err)
	}

	fallbackResult, fallbackErr := f.fallback.AnalyzePhoto(ctx, photoBytes, prompt)
	if fallbackErr != nil {
		return "", errors.Join(
			fmt.Errorf("primary vision: %w", err),
			fmt.Errorf("fallback vision: %w", fallbackErr),
		)
	}

	if normalized, ok := normalizeJSON(fallbackResult); ok {
		return normalized, nil
	}
	return "", fmt.Errorf("fallback vision returned invalid JSON: %s", fallbackResult)
}

type FallbackEmbedder struct {
	primary  Embedder
	fallback Embedder
}

func NewFallbackEmbedder(primary Embedder, fallback Embedder) (*FallbackEmbedder, error) {
	switch {
	case primary == nil:
		return nil, fmt.Errorf("fallback embedder init: 'primary embedder' is required")
	case fallback == nil:
		return nil, fmt.Errorf("fallback embedder init: 'fallback embedder' is required")
	}

	return &FallbackEmbedder{
		primary:  primary,
		fallback: fallback,
	}, nil
}

func (f *FallbackEmbedder) Vectorize(ctx context.Context, text string) ([]float32, error) {
	res, err := f.primary.Vectorize(ctx, text)
	if err == nil {
		return res, nil
	}

	if err != nil {
		log.Printf("WARN: primary embedder failed, falling back... Error: %v", err)
	}

	fallbackResult, fallbackErr := f.fallback.Vectorize(ctx, text)
	if fallbackErr != nil {
		return nil, errors.Join(
			fmt.Errorf("primary embedder: %w", err),
			fmt.Errorf("fallback embedder: %w", fallbackErr),
		)
	}

	return fallbackResult, nil
}

func normalizeJSON(response string) (string, bool) {
	trimmed := strings.TrimSpace(response)
	if json.Valid([]byte(trimmed)) {
		return trimmed, true
	}
	if !strings.HasPrefix(trimmed, "```") || !strings.HasSuffix(trimmed, "```") {
		return "", false
	}

	firstLineEnd := strings.IndexByte(trimmed, '\n')
	if firstLineEnd < 0 {
		return "", false
	}
	unwrapped := strings.TrimSpace(strings.TrimSuffix(trimmed[firstLineEnd+1:], "```"))
	return unwrapped, json.Valid([]byte(unwrapped))
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
