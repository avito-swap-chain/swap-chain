package adapters

import (
	"context"
	"encoding/json"
	"errors"
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
	primary        Enricher
	fallback       Enricher
	reportFallback func(error)
}

func NewFallbackClient(primary Enricher, fallback Enricher, reportFallback func(error)) (*FallbackClient, error) {
	switch {
	case primary == nil:
		return nil, fmt.Errorf("fallback client init: 'primary enricher' is required")
	case fallback == nil:
		return nil, fmt.Errorf("fallback client init: 'fallback enricher' is required")
	case reportFallback == nil:
		return nil, fmt.Errorf("fallback client init: 'fallback reporter' is required")
	}

	return &FallbackClient{
		primary:        primary,
		fallback:       fallback,
		reportFallback: reportFallback,
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

	f.reportFallback(err)
	fallbackResult, fallbackErr := f.fallback.GenerateJSON(ctx, prompt)
	if fallbackErr != nil {
		return "", errors.Join(
			fmt.Errorf("primary enricher: %w", err),
			fmt.Errorf("fallback enricher: %w", fallbackErr),
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
