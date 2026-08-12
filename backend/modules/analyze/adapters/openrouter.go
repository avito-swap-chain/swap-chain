package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"time"
)

type OpenRouterConfig struct {
	BaseURL    string
	APIKey     string
	Model      string
	Timeout    time.Duration
	Dimensions int
}

func DefaultOpenRouterConfig(apiKey string) OpenRouterConfig {
	return OpenRouterConfig{
		BaseURL: "https://openrouter.ai/api/v1",
		APIKey:  apiKey,
		Model:   "qwen/qwen3-embedding:4b", // Or whatever the exact ID is
		Timeout: 2 * time.Minute,
	}
}

type OpenRouter struct {
	client *http.Client
	cfg    OpenRouterConfig
}

func NewOpenRouter(cfg OpenRouterConfig) (*OpenRouter, error) {
	switch {
	case strings.TrimSpace(cfg.APIKey) == "":
		return nil, fmt.Errorf("openrouter init: 'API key' is required")
	case strings.TrimSpace(cfg.Model) == "":
		return nil, fmt.Errorf("openrouter init: 'model' is required")
	case cfg.Timeout <= 0:
		return nil, fmt.Errorf("openrouter init: 'timeout' must be positive")
	}

	if err := validateHTTPURL("base URL", cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("openrouter init: %w", err)
	}

	return &OpenRouter{
		client: &http.Client{Timeout: cfg.Timeout},
		cfg:    cfg,
	}, nil
}

func (o *OpenRouter) Vectorize(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/embeddings", strings.TrimRight(o.cfg.BaseURL, "/"))

	bodyMap := map[string]any{
		"model": o.cfg.Model,
		"input": text,
	}
	if o.cfg.Dimensions > 0 {
		bodyMap["dimensions"] = o.cfg.Dimensions
	}

	reqBody, err := json.Marshal(bodyMap)
	if err != nil {
		return nil, fmt.Errorf("openrouter vectorize - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("openrouter vectorize - create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	// OpenRouter typically requests a Referer header to identify the app
	req.Header.Set("HTTP-Referer", "http://localhost")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("openrouter vectorize - execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("openrouter vectorize: bad status: %d", resp.StatusCode)
	}

	var response struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("openrouter vectorize - decode response: %w", err)
	}

	if len(response.Data) == 0 || len(response.Data[0].Embedding) == 0 {
		return nil, fmt.Errorf("openrouter vectorize: empty embeddings response")
	}

	return response.Data[0].Embedding, nil
}
