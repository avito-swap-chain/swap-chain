package adapters

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
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
		Model:   "qwen/qwen3-embedding:4b",
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

func (o *OpenRouter) AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", strings.TrimRight(o.cfg.BaseURL, "/"))
	base64Img := base64.StdEncoding.EncodeToString(photoBytes)

	// Determine mime type basic
	mimeType := "image/jpeg"
	if len(photoBytes) > 4 && photoBytes[0] == 0x89 && photoBytes[1] == 0x50 && photoBytes[2] == 0x4E && photoBytes[3] == 0x47 {
		mimeType = "image/png"
	}

	bodyMap := map[string]any{
		"model": o.cfg.Model,
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{
						"type": "text",
						"text": prompt,
					},
					{
						"type": "image_url",
						"image_url": map[string]string{
							"url": fmt.Sprintf("data:%s;base64,%s", mimeType, base64Img),
						},
					},
				},
			},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	reqBody, err := json.Marshal(bodyMap)
	if err != nil {
		return "", fmt.Errorf("openrouter analyze photo - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("openrouter analyze photo - create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	req.Header.Set("HTTP-Referer", "http://localhost")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter analyze photo - execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		return "", fmt.Errorf("openrouter analyze photo: bad status: %d, body: %s", resp.StatusCode, string(body))
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("openrouter analyze photo - decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openrouter analyze photo: empty choices response")
	}

	return response.Choices[0].Message.Content, nil
}

func (o *OpenRouter) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/chat/completions", strings.TrimRight(o.cfg.BaseURL, "/"))

	bodyMap := map[string]any{
		"model": o.cfg.Model,
		"messages": []map[string]any{
			{
				"role":    "user",
				"content": prompt,
			},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	reqBody, err := json.Marshal(bodyMap)
	if err != nil {
		return "", fmt.Errorf("openrouter generate json - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("openrouter generate json - create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+o.cfg.APIKey)
	req.Header.Set("HTTP-Referer", "http://localhost")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("openrouter generate json - execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("openrouter generate json: bad status: %d", resp.StatusCode)
	}

	var response struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("openrouter generate json - decode response: %w", err)
	}

	if len(response.Choices) == 0 {
		return "", fmt.Errorf("openrouter generate json: empty choices response")
	}

	return response.Choices[0].Message.Content, nil
}
