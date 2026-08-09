package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

const defaultOllamaTimeout = 30 * time.Second

type OllamaConfig struct {
	BaseURL         string
	ChatModel       string
	EmbeddingsModel string
}

func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL:         "http://localhost:11434",
		ChatModel:       "llama3.1",
		EmbeddingsModel: "bge-m3",
	}
}

type Ollama struct {
	client *http.Client
	cfg    OllamaConfig
}

func NewOllama(cfg OllamaConfig) *Ollama {
	return &Ollama{
		client: &http.Client{Timeout: defaultOllamaTimeout},
		cfg:    cfg,
	}
}

func (o *Ollama) Vectorize(ctx context.Context, text string) ([]float32, error) {
	url := fmt.Sprintf("%s/api/embeddings", o.cfg.BaseURL)
	reqBody, err := json.Marshal(map[string]string{
		"model":  o.cfg.EmbeddingsModel,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("vectorize - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("vectorize - create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vectorize - execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vectorize: bad status: %d", resp.StatusCode)
	}

	var response struct {
		Embedding []float32 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("vectorize - decode response: %w", err)
	}

	return response.Embedding, nil
}

type OllamaGenerateRequest struct {
	Model   string                 `json:"model"`
	Prompt  string                 `json:"prompt"`
	Format  string                 `json:"format,omitempty"`
	Stream  bool                   `json:"stream"`
	Options map[string]interface{} `json:"options,omitempty"`
}

type OllamaGenerateResponse struct {
	Response string `json:"response"`
}

func (o *Ollama) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	url := fmt.Sprintf("%s/api/generate", o.cfg.BaseURL)
	reqBody, err := json.Marshal(OllamaGenerateRequest{
		Model:  o.cfg.ChatModel,
		Prompt: prompt,
		Format: "json",
		Stream: false,
		Options: map[string]any{
			"num_predict": 100,
			"temperature": 0.1,
		},
	})
	if err != nil {
		return "", fmt.Errorf("generate - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewBuffer(reqBody))
	if err != nil {
		return "", fmt.Errorf("generate - create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := o.client.Do(req)
	if err != nil {
		return "", fmt.Errorf("generate - execute request: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("generate: bad status: %d", resp.StatusCode)
	}

	var response OllamaGenerateResponse
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return "", fmt.Errorf("generate - decode response: %w", err)
	}

	return response.Response, nil
}
