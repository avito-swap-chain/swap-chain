package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"sort"
	"strings"
	"time"
)

const defaultOllamaTimeout = 30 * time.Second

type OllamaConfig struct {
	BaseURL         string
	ChatModel       string
	EmbeddingsModel string
	Timeout         time.Duration
}

func DefaultOllamaConfig() OllamaConfig {
	return OllamaConfig{
		BaseURL:         "http://localhost:11434",
		ChatModel:       "llama3.1",
		EmbeddingsModel: "bge-m3",
		Timeout:         defaultOllamaTimeout,
	}
}

// Ready verifies that Ollama exposes both configured models.
func (o *Ollama) Ready(ctx context.Context) error {
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, o.cfg.BaseURL+"/api/tags", nil)
	if err != nil {
		return fmt.Errorf("ollama readiness - create request: %w", err)
	}
	response, err := o.client.Do(request)
	if err != nil {
		return fmt.Errorf("ollama readiness - execute request: %w", err)
	}
	defer func() { _ = response.Body.Close() }()
	if response.StatusCode != http.StatusOK {
		return fmt.Errorf("ollama readiness: bad status: %d", response.StatusCode)
	}

	var payload struct {
		Models []struct {
			Name  string `json:"name"`
			Model string `json:"model"`
		} `json:"models"`
	}
	if err := json.NewDecoder(response.Body).Decode(&payload); err != nil {
		return fmt.Errorf("ollama readiness - decode response: %w", err)
	}

	available := make(map[string]struct{}, len(payload.Models)*2)
	for _, model := range payload.Models {
		available[canonicalModelName(model.Name)] = struct{}{}
		available[canonicalModelName(model.Model)] = struct{}{}
	}
	required := []string{o.cfg.ChatModel, o.cfg.EmbeddingsModel}
	missing := make([]string, 0, len(required))
	for _, model := range required {
		if _, ok := available[canonicalModelName(model)]; !ok {
			missing = append(missing, model)
		}
	}
	if len(missing) > 0 {
		sort.Strings(missing)
		return fmt.Errorf("ollama readiness: required models are missing: %s", strings.Join(missing, ", "))
	}
	return nil
}

func canonicalModelName(name string) string {
	name = strings.TrimSpace(name)
	if name == "" || strings.Contains(name, ":") {
		return name
	}
	return name + ":latest"
}

type Ollama struct {
	client *http.Client
	cfg    OllamaConfig
}

func NewOllama(cfg OllamaConfig) (*Ollama, error) {
	switch {
	case strings.TrimSpace(cfg.ChatModel) == "":
		return nil, fmt.Errorf("ollama init: 'chat model' is required")
	case strings.TrimSpace(cfg.EmbeddingsModel) == "":
		return nil, fmt.Errorf("ollama init: 'embeddings model' is required")
	case cfg.Timeout <= 0:
		return nil, fmt.Errorf("ollama init: 'timeout' must be positive")
	}
	if err := validateHTTPURL("base URL", cfg.BaseURL); err != nil {
		return nil, fmt.Errorf("ollama init: %w", err)
	}

	return &Ollama{
		client: &http.Client{Timeout: cfg.Timeout},
		cfg:    cfg,
	}, nil
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
