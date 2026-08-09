package adapters

import (
	"bytes"
	"context"
	"crypto/tls"
	"crypto/x509"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"net/url"
	"os"
	"strings"
	"sync"
	"time"

	"github.com/google/uuid"

	"swap-chain/analyze/model"
)

type GigaChatConfig struct {
	AuthKey         string
	Scope           string
	OAuthURL        string
	ChatURL         string
	EmbeddingsURL   string
	FilesURL        string
	ChatModel       string
	EmbeddingsModel string
	CABundleFile    string
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
		EmbeddingsModel: "Embeddings",
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

func NewGigaChat(cfg GigaChatConfig) (*GigaChat, error) {
	switch {
	case strings.TrimSpace(cfg.AuthKey) == "":
		return nil, fmt.Errorf("gigachat init: 'auth key' is required")
	case strings.TrimSpace(cfg.Scope) == "":
		return nil, fmt.Errorf("gigachat init: 'scope' is required")
	case strings.TrimSpace(cfg.ChatModel) == "":
		return nil, fmt.Errorf("gigachat init: 'chat model' is required")
	case strings.TrimSpace(cfg.EmbeddingsModel) == "":
		return nil, fmt.Errorf("gigachat init: 'embeddings model' is required")
	case cfg.Timeout <= 0:
		return nil, fmt.Errorf("gigachat init: 'timeout' must be positive")
	}

	urls := []struct {
		name  string
		value string
	}{
		{name: "oauth URL", value: cfg.OAuthURL},
		{name: "chat URL", value: cfg.ChatURL},
		{name: "embeddings URL", value: cfg.EmbeddingsURL},
		{name: "files URL", value: cfg.FilesURL},
	}
	for _, endpoint := range urls {
		if err := validateHTTPURL(endpoint.name, endpoint.value); err != nil {
			return nil, fmt.Errorf("gigachat init: %w", err)
		}
	}

	client := &http.Client{Timeout: cfg.Timeout}
	if cfg.CABundleFile != "" {
		tlsConfig, err := buildGigaChatTLSConfig(cfg.CABundleFile)
		if err != nil {
			return nil, fmt.Errorf("gigachat init: %w", err)
		}
		client.Transport = &http.Transport{TLSClientConfig: tlsConfig}
	}

	return &GigaChat{
		cfg:    cfg,
		client: client,
	}, nil
}

func buildGigaChatTLSConfig(caBundleFile string) (*tls.Config, error) {
	caPEM, err := os.ReadFile(caBundleFile)
	if err != nil {
		return nil, fmt.Errorf("read CA bundle %q: %w", caBundleFile, err)
	}

	rootCAs, err := x509.SystemCertPool()
	if err != nil {
		rootCAs = x509.NewCertPool()
	}
	if !rootCAs.AppendCertsFromPEM(caPEM) {
		return nil, fmt.Errorf("add certificates from CA bundle %q", caBundleFile)
	}

	return &tls.Config{RootCAs: rootCAs}, nil
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", &model.APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
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
		return "", model.ErrEmptyChoices
	}

	return chatResp.Choices[0].Message.Content, nil
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
		return "", model.ErrEmptyChoices
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return "", &model.APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
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
	defer func() { _ = resp.Body.Close() }()

	if resp.StatusCode >= 400 {
		respBytes, _ := io.ReadAll(resp.Body)
		return &model.APIError{StatusCode: resp.StatusCode, Message: string(respBytes)}
	}

	if err := json.NewDecoder(resp.Body).Decode(respBody); err != nil {
		return err
	}

	return nil
}
