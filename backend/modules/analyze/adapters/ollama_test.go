package adapters

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestDefaultOllamaConfigUsesLongRunningTimeout(t *testing.T) {
	if timeout := DefaultOllamaConfig().Timeout; timeout != 4*time.Minute {
		t.Fatalf("Timeout = %s, want 4m", timeout)
	}
}

func TestOllamaVectorize(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			t.Errorf("method: got %s, want POST", r.Method)
		}
		if got := r.Header.Get("Content-Type"); got != "application/json" {
			t.Errorf("content type: got %q, want application/json", got)
		}

		var body map[string]any
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			t.Errorf("decode request: %v", err)
		}
		if body["model"] != "test-model" || (body["prompt"] != "велосипед" && body["input"] != "велосипед") {
			t.Errorf("request body: got %+v", body)
		}

		w.Header().Set("Content-Type", "application/json")
		_, _ = w.Write([]byte(`{"embeddings":[[0.1,0.2,0.3]]}`))
	}))
	defer server.Close()

	adapter := &Ollama{
		client: server.Client(),
		cfg: OllamaConfig{
			BaseURL:         server.URL,
			EmbeddingsModel: "test-model",
		},
	}
	got, err := adapter.Vectorize(context.Background(), "велосипед")
	if err != nil {
		t.Fatalf("vectorize: %v", err)
	}
	want := []float32{0.1, 0.2, 0.3}
	if len(got) != len(want) {
		t.Fatalf("embedding length: got %d, want %d", len(got), len(want))
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("embedding[%d]: got %v, want %v", i, got[i], want[i])
		}
	}
}

func TestOllamaVectorizeRejectsNonOKResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, "unavailable", http.StatusServiceUnavailable)
	}))
	defer server.Close()

	adapter := &Ollama{
		client: server.Client(),
		cfg: OllamaConfig{
			BaseURL:         server.URL,
			EmbeddingsModel: "test-model",
		},
	}
	_, err := adapter.Vectorize(context.Background(), "велосипед")
	if err == nil || !strings.Contains(err.Error(), "bad status: 503") {
		t.Fatalf("error: got %v, want status error", err)
	}
}

func TestOllamaVectorizeRejectsInvalidJSON(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte("not-json"))
	}))
	defer server.Close()

	adapter := &Ollama{
		client: server.Client(),
		cfg: OllamaConfig{
			BaseURL:         server.URL,
			EmbeddingsModel: "test-model",
		},
	}
	_, err := adapter.Vectorize(context.Background(), "велосипед")
	if err == nil || !strings.Contains(err.Error(), "decode response") {
		t.Fatalf("error: got %v, want decode error", err)
	}
}

func TestOllamaVectorizeHonorsCancelledContext(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(http.ResponseWriter, *http.Request) {
		t.Error("request must not reach server with an already cancelled context")
	}))
	defer server.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel()

	adapter := &Ollama{
		client: server.Client(),
		cfg: OllamaConfig{
			BaseURL:         server.URL,
			EmbeddingsModel: "test-model",
		},
	}
	_, err := adapter.Vectorize(ctx, "велосипед")
	if err == nil {
		t.Fatal("vectorize: expected context error, got nil")
	}
}

func TestOllamaReadyAcceptsConfiguredModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, request *http.Request) {
		if request.Method != http.MethodGet || request.URL.Path != "/api/tags" {
			t.Fatalf("request = %s %s", request.Method, request.URL.Path)
		}
		_, _ = w.Write([]byte(`{"models":[{"name":"bge-m3:latest"},{"model":"llama3.1:latest"}]}`))
	}))
	defer server.Close()
	adapter := &Ollama{
		client: server.Client(),
		cfg:    OllamaConfig{BaseURL: server.URL, ChatModel: "llama3.1", EmbeddingsModel: "bge-m3"},
	}
	if err := adapter.Ready(context.Background()); err != nil {
		t.Fatalf("ready: %v", err)
	}
}

func TestOllamaReadyReportsMissingModels(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		_, _ = w.Write([]byte(`{"models":[{"name":"bge-m3:latest"}]}`))
	}))
	defer server.Close()
	adapter := &Ollama{
		client: server.Client(),
		cfg:    OllamaConfig{BaseURL: server.URL, ChatModel: "llama3.1", EmbeddingsModel: "bge-m3"},
	}
	err := adapter.Ready(context.Background())
	if err == nil || !strings.Contains(err.Error(), "llama3.1") {
		t.Fatalf("ready error = %v", err)
	}
}
