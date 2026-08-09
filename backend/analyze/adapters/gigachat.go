package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"
)

type APIError struct {
	StatusCode int
	Message    string
}

func (e *APIError) Error() string {
	return fmt.Sprintf("gigachat api error: status=%d, message=%s", e.StatusCode, e.Message)
}

type GigaChatConfig struct {
	AuthKey         string
	Scope           string
	OAuthURL        string
	ChatURL         string
	EmbeddingsURL   string
	FilesURL        string
	ChatModel       string
	EmbeddingsModel string
	Timeout         time.Duration
}

func DefaultGigaChatConfig(authKey string) GigaChatConfig {
	return GigaChatConfig{
		AuthKey:         authKey,
		Scope:           "GIGACHAT_API_PERS",
		OAuthURL:        "https://ngw.devices.sberbank.ru:9443/api/v2/oauth",
		ChatURL:         "https://api.giga.chat/v1/chat/completions",
		EmbeddingsURL:   "https://api.giga.chat/v1/embeddings",
		FilesURL:        "https://api.giga.chat/v1/files",
		ChatModel:       "GigaChat-3-Ultra",
		EmbeddingsModel: "GigaEmbeddings-3B-2025-09",
		Timeout:         30 * time.Second,
	}
}

type TokenResponse struct {
	AccessToken string `json:"access_token"`
	ExpiresAt   int64  `json:"expires_at"`
}

type FileUploadResponse struct {
	ID string `json:"id"`
}

type GigaChat struct {
	cfg         GigaChatConfig
	mu          sync.RWMutex
	accessToken string
	expiresAt   time.Time
	client      *http.Client
}

func NewGigaChat(cfg GigaChatConfig) *GigaChat {
	return &GigaChat{
		cfg: cfg,
		client: &http.Client{
			Transport: &http.Transport{
				TLSClientConfig: &tls.Config{InsecureSkipVerify: true},
			},
			Timeout: cfg.Timeout,
		},
	}
}

func (g *GigaChat) GetToken(ctx context.Context) (string, error) {
	g.mu.RLock()
	if g.accessToken != "" && time.Now().Add(2*time.Minute).Before(g.expiresAt) {
		token := g.accessToken
		g.mu.RUnlock()
		return token, nil
	}
	g.mu.RUnlock()

	g.mu.Lock()
	defer g.mu.Unlock()

	if g.accessToken != "" && time.Now().Add(2*time.Minute).Before(g.expiresAt) {
		return g.accessToken, nil
	}

	return g.refreshToken(ctx)
}

func (g *GigaChat) refreshToken(ctx context.Context) (string, error) {
	data := url.Values{}
	data.Set("scope", g.cfg.Scope)
	body := strings.NewReader(data.Encode())

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.OAuthURL, body)
	if err != nil {
		return "", err
	}

	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("RqUID", uuid.New().String())
	req.Header.Set("Authorization", "Basic "+g.cfg.AuthKey)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", &APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
	}

	var tokenResp TokenResponse
	if err := json.NewDecoder(resp.Body).Decode(&tokenResp); err != nil {
		return "", err
	}

	g.accessToken = tokenResp.AccessToken
	g.expiresAt = time.Unix(0, tokenResp.ExpiresAt*int64(time.Millisecond))

	return g.accessToken, nil
}

func (g *GigaChat) GenerateJSON(ctx context.Context, prompt string) (string, error) {
	token, err := g.GetToken(ctx)
	if err != nil {
		return "", err
	}

	requestBody := map[string]interface{}{
		"model": g.cfg.ChatModel,
		"messages": []map[string]interface{}{
			{"role": "user", "content": prompt},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := g.doJSONRequest(ctx, g.cfg.ChatURL, token, requestBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (g *GigaChat) Vectorize(ctx context.Context, text string) ([]float32, error) {
	token, err := g.GetToken(ctx)
	if err != nil {
		return nil, err
	}

	requestBody := map[string]interface{}{
		"model": g.cfg.EmbeddingsModel,
		"input": []string{text},
	}

	var embResp struct {
		Data []struct {
			Embedding []float32 `json:"embedding"`
		} `json:"data"`
	}

	if err := g.doJSONRequest(ctx, g.cfg.EmbeddingsURL, token, requestBody, &embResp); err != nil {
		return nil, err
	}

	if len(embResp.Data) == 0 {
		return nil, fmt.Errorf("empty embeddings data")
	}

	return embResp.Data[0].Embedding, nil
}

func (g *GigaChat) AnalyzePhoto(ctx context.Context, photoBytes []byte, prompt string) (string, error) {
	token, err := g.GetToken(ctx)
	if err != nil {
		return "", err
	}

	fileID, err := g.uploadPhoto(ctx, token, photoBytes)
	if err != nil {
		return "", err
	}

	requestBody := map[string]interface{}{
		"model": g.cfg.ChatModel,
		"messages": []map[string]interface{}{
			{
				"role":        "user",
				"content":     prompt,
				"attachments": []string{fileID},
			},
		},
		"response_format": map[string]string{"type": "json_object"},
	}

	var chatResp struct {
		Choices []struct {
			Message struct {
				Content string `json:"content"`
			} `json:"message"`
		} `json:"choices"`
	}

	if err := g.doJSONRequest(ctx, g.cfg.ChatURL, token, requestBody, &chatResp); err != nil {
		return "", err
	}

	if len(chatResp.Choices) == 0 {
		return "", fmt.Errorf("empty choices")
	}

	return chatResp.Choices[0].Message.Content, nil
}

func (g *GigaChat) uploadPhoto(ctx context.Context, token string, photoBytes []byte) (string, error) {
	bodyBuf := &bytes.Buffer{}
	writer := multipart.NewWriter(bodyBuf)

	_ = writer.WriteField("purpose", "general")

	h := make(textproto.MIMEHeader)
	h.Set("Content-Disposition", `form-data; name="file"; filename="image.jpg"`)
	h.Set("Content-Type", http.DetectContentType(photoBytes))

	part, err := writer.CreatePart(h)
	if err != nil {
		return "", err
	}
	if _, err = part.Write(photoBytes); err != nil {
		return "", err
	}
	if err = writer.Close(); err != nil {
		return "", err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.cfg.FilesURL, bodyBuf)
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", writer.FormDataContentType())
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.client.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", &APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
	}

	var fileResp FileUploadResponse
	if err := json.NewDecoder(resp.Body).Decode(&fileResp); err != nil {
		return "", err
	}

	return fileResp.ID, nil
}

func (g *GigaChat) doJSONRequest(ctx context.Context, url string, token string, reqBody interface{}, respBody interface{}) error {
	reqBytes, err := json.Marshal(reqBody)
	if err != nil {
		return err
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader(reqBytes))
	if err != nil {
		return err
	}

	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.client.Do(req)
	if err != nil {
		return err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return &APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
	}

	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return err
	}

	return nil
}
