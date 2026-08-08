package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
)

type Ollama struct {
	client *http.Client
	url    string
	model  string
}

func NewOllama() *Ollama {
	return &Ollama{
		client: &http.Client{},
		url:    "http://localhost:11434/api/embeddings",
		model:  "bge-m3",
	}
}

func (a *Ollama) Vectorize(ctx context.Context, text string) ([]float64, error) {
	reqBody, err := json.Marshal(map[string]string{
		"model":  a.model,
		"prompt": text,
	})
	if err != nil {
		return nil, fmt.Errorf("vectorize - marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, "POST", a.url, bytes.NewBuffer(reqBody))
	if err != nil {
		return nil, fmt.Errorf("vectorize - create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	resp, err := a.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("vectorize - execute request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("vectorize: bad status: %d", resp.StatusCode)
	}

	var response struct {
		Embedding []float64 `json:"embedding"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&response); err != nil {
		return nil, fmt.Errorf("vectorize - decode response: %w", err)
	}

	return response.Embedding, nil
}
