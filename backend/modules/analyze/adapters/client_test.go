package adapters

import (
	"context"
	"errors"
	"testing"
)

type enricherStub struct {
	response string
	err      error
	calls    int
}

func ignoreFallback(error) {}

func (s *enricherStub) GenerateJSON(context.Context, string) (string, error) {
	s.calls++
	return s.response, s.err
}

func TestFallbackClientUsesPrimary(t *testing.T) {
	primary := &enricherStub{response: `{"value":"primary"}`}
	fallback := &enricherStub{response: `{"value":"fallback"}`}
	client, err := NewFallbackClient(primary, fallback)
	if err != nil {
		t.Fatalf("NewFallbackClient() error = %v", err)
	}

	result, err := client.GenerateJSON(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if result != `{"value":"primary"}` || primary.calls != 1 || fallback.calls != 0 {
		t.Fatalf("GenerateJSON() = %q, primary calls = %d, fallback calls = %d", result, primary.calls, fallback.calls)
	}
}

func TestFallbackClientUsesFallbackOnPrimaryError(t *testing.T) {
	primary := &enricherStub{err: errors.New("primary unavailable")}
	fallback := &enricherStub{response: `{"value":"fallback"}`}
	client, err := NewFallbackClient(primary, fallback)
	if err != nil {
		t.Fatalf("NewFallbackClient() error = %v", err)
	}

	result, err := client.GenerateJSON(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if result != `{"value":"fallback"}` || primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("GenerateJSON() = %q, primary calls = %d, fallback calls = %d", result, primary.calls, fallback.calls)
	}
}

func TestFallbackClientUsesFallbackOnInvalidPrimaryJSON(t *testing.T) {
	primary := &enricherStub{response: "not-json"}
	fallback := &enricherStub{response: `{"value":"fallback"}`}
	client, err := NewFallbackClient(primary, fallback)
	if err != nil {
		t.Fatalf("NewFallbackClient() error = %v", err)
	}

	result, err := client.GenerateJSON(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if result != `{"value":"fallback"}` || primary.calls != 1 || fallback.calls != 1 {
		t.Fatalf("GenerateJSON() = %q, primary calls = %d, fallback calls = %d", result, primary.calls, fallback.calls)
	}
}

func TestFallbackClientAcceptsFencedPrimaryJSON(t *testing.T) {
	primary := &enricherStub{response: "```json\n{\"value\":\"primary\"}\n```"}
	fallback := &enricherStub{response: `{"value":"fallback"}`}
	client, err := NewFallbackClient(primary, fallback)
	if err != nil {
		t.Fatalf("NewFallbackClient() error = %v", err)
	}

	result, err := client.GenerateJSON(context.Background(), "prompt")
	if err != nil {
		t.Fatalf("GenerateJSON() error = %v", err)
	}
	if result != `{"value":"primary"}` || fallback.calls != 0 {
		t.Fatalf("GenerateJSON() = %q, fallback calls = %d", result, fallback.calls)
	}
}

