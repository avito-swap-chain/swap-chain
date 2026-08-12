package adapters

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

type VoyageConfig struct {
	BaseURL string
	APIKey  string
	Model   string
	Timeout time.Duration
}

func DefaultVoyageConfig(apiKey string) VoyageConfig {
	return VoyageConfig{
		BaseURL: "https://openrouter.ai/api/v1",
		APIKey:  apiKey,
		Model:   "voyageai/voyage-3-large",
		Timeout: 2 * time.Minute,
	}
}

type Voyage struct {
	client *http.Client
	cfg    VoyageConfig
}

func NewVoyage(cfg VoyageConfig) (*Voyage, error) {
	if strings.TrimSpace(cfg.APIKey) == "" {
		return nil, fmt.Errorf("voyage init: API key is required")
	}

	return &Voyage{
		client: &http.Client{Timeout: cfg.Timeout},
		cfg:    cfg,
	}, nil
}

type voyageEmbeddingRequest struct {
	Input []string `json:"input"`
	Model string   `json:"model"`
}

type voyageEmbeddingResponse struct {
	Data []struct {
		Embedding []float32 `json:"embedding"`
		Index     int       `json:"index"`
	} `json:"data"`
}

func (v *Voyage) Vectorize(ctx context.Context, text string) ([]float32, error) {
	reqBody := voyageEmbeddingRequest{
		Input: []string{text},
		Model: v.cfg.Model,
	}

	bodyBytes, err := json.Marshal(reqBody)
	if err != nil {
		return nil, fmt.Errorf("voyage marshal: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, v.cfg.BaseURL+"/embeddings", bytes.NewReader(bodyBytes))
	if err != nil {
		return nil, fmt.Errorf("voyage create request: %w", err)
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+v.cfg.APIKey)
	req.Header.Set("HTTP-Referer", "http://localhost")

	resp, err := v.client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("voyage do request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		respBody, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("voyage api error: status=%d body=%s", resp.StatusCode, string(respBody))
	}

	var res voyageEmbeddingResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		return nil, fmt.Errorf("voyage decode response: %w", err)
	}

	if len(res.Data) == 0 {
		return nil, fmt.Errorf("voyage api returned no embeddings")
	}

	return res.Data[0].Embedding, nil
}
