package adapters

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"
)

func TestGigaChatGenerationSlotHonorsContext(t *testing.T) {
	client := &GigaChat{generationSlot: make(chan struct{}, 1)}
	release, err := client.acquireGenerationSlot(context.Background())
	if err != nil {
		t.Fatalf("acquireGenerationSlot() error = %v", err)
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := client.acquireGenerationSlot(ctx); !errors.Is(err, context.Canceled) {
		t.Fatalf("acquireGenerationSlot() error = %v, want context canceled", err)
	}

	release()
	secondRelease, err := client.acquireGenerationSlot(context.Background())
	if err != nil {
		t.Fatalf("acquireGenerationSlot() after release error = %v", err)
	}
	secondRelease()
}

func TestGigaChatAnalyzePhotoUploadsPNGAndDeletesFile(t *testing.T) {
	deleted := false
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/oauth":
			_, _ = fmt.Fprintf(writer, `{"access_token":"token","expires_at":%d}`, time.Now().Add(time.Hour).UnixMilli())
		case request.Method == http.MethodPost && request.URL.Path == "/files":
			if err := request.ParseMultipartForm(1 << 20); err != nil {
				t.Errorf("parse multipart: %v", err)
				http.Error(writer, "invalid multipart", http.StatusBadRequest)
				return
			}
			file, header, err := request.FormFile("file")
			if err != nil {
				t.Errorf("read uploaded file: %v", err)
				http.Error(writer, "missing file", http.StatusBadRequest)
				return
			}
			_ = file.Close()
			if header.Filename != "image.png" || header.Header.Get("Content-Type") != "image/png" {
				t.Errorf("uploaded file = %q %q", header.Filename, header.Header.Get("Content-Type"))
			}
			_, _ = writer.Write([]byte(`{"id":"file-1"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/chat":
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"result"}}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/files/file-1/delete":
			deleted = true
			writer.WriteHeader(http.StatusNoContent)
		default:
			http.Error(writer, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewGigaChat(GigaChatConfig{
		AuthKey:         "auth",
		Scope:           "scope",
		OAuthURL:        server.URL + "/oauth",
		ChatURL:         server.URL + "/chat",
		EmbeddingsURL:   server.URL + "/embeddings",
		FilesURL:        server.URL + "/files",
		ChatModel:       "chat",
		EmbeddingsModel: "embeddings",
		Timeout:         time.Second,
	})
	if err != nil {
		t.Fatalf("NewGigaChat() error = %v", err)
	}

	png := []byte{0x89, 0x50, 0x4e, 0x47, 0x0d, 0x0a, 0x1a, 0x0a, 0, 0, 0, 0}
	result, err := client.AnalyzePhoto(context.Background(), png, "prompt")
	if err != nil {
		t.Fatalf("AnalyzePhoto() error = %v", err)
	}
	if strings.TrimSpace(result) != "result" {
		t.Fatalf("AnalyzePhoto() = %q", result)
	}
	if !deleted {
		t.Fatal("uploaded file was not deleted")
	}
}

func TestGigaChatAnalyzePhotoDoesNotLoseResultWhenCleanupFails(t *testing.T) {
	var cleanupErr error
	server := httptest.NewServer(http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		switch {
		case request.Method == http.MethodPost && request.URL.Path == "/oauth":
			_, _ = fmt.Fprintf(writer, `{"access_token":"token","expires_at":%d}`, time.Now().Add(time.Hour).UnixMilli())
		case request.Method == http.MethodPost && request.URL.Path == "/files":
			_, _ = writer.Write([]byte(`{"id":"file-1"}`))
		case request.Method == http.MethodPost && request.URL.Path == "/chat":
			_, _ = writer.Write([]byte(`{"choices":[{"message":{"content":"result"}}]}`))
		case request.Method == http.MethodPost && request.URL.Path == "/files/file-1/delete":
			http.Error(writer, "forbidden", http.StatusForbidden)
		default:
			http.Error(writer, "unexpected request", http.StatusNotFound)
		}
	}))
	defer server.Close()

	client, err := NewGigaChat(GigaChatConfig{
		AuthKey:         "auth",
		Scope:           "scope",
		OAuthURL:        server.URL + "/oauth",
		ChatURL:         server.URL + "/chat",
		EmbeddingsURL:   server.URL + "/embeddings",
		FilesURL:        server.URL + "/files",
		ChatModel:       "chat",
		EmbeddingsModel: "embeddings",
		Timeout:         time.Second,
		OnCleanupError:  func(err error) { cleanupErr = err },
	})
	if err != nil {
		t.Fatalf("NewGigaChat() error = %v", err)
	}

	result, err := client.AnalyzePhoto(context.Background(), []byte("jpeg image"), "prompt")
	if err != nil {
		t.Fatalf("AnalyzePhoto() error = %v", err)
	}
	if result != "result" || cleanupErr == nil {
		t.Fatalf("result = %q, cleanup error = %v", result, cleanupErr)
	}
}
