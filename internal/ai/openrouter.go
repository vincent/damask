package ai

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"damask/server/internal/transform"
)

const openRouterDefaultBaseURL = "https://openrouter.ai/api/v1"

const (
	openRouterTimeout = 120 * time.Second
	openRouterReferer = "https://damask.studio"
	openRouterTitle   = "Damask"

	modImage = "image"
	modText  = "text"
)

type openRouterProvider struct {
	apiKey          string
	keySource       KeySource
	defaultBgModel  string
	defaultI2IModel string
	baseURL         string
	httpClient      *http.Client
}

// NewOpenRouterProvider returns a Provider backed by OpenRouter.
// Pass apiKey="" and keySource="none" when not configured.
func NewOpenRouterProvider(apiKey string, keySource KeySource, defaultBgModel, defaultI2IModel string) Provider {
	return &openRouterProvider{
		apiKey:          apiKey,
		keySource:       keySource,
		defaultBgModel:  defaultBgModel,
		defaultI2IModel: defaultI2IModel,
		baseURL:         openRouterDefaultBaseURL,
		httpClient:      &http.Client{Timeout: openRouterTimeout},
	}
}

// NewOpenRouterProviderForTest constructs an openRouterProvider with a custom base URL,
// allowing tests to point at a local [httptest.Server] without a package-level mutable var.
func NewOpenRouterProviderForTest(
	apiKey string,
	keySource KeySource,
	defaultBgModel, defaultI2IModel, baseURL string,
) Provider {
	p := NewOpenRouterProvider(apiKey, keySource, defaultBgModel, defaultI2IModel).(*openRouterProvider) //nolint:errcheck // tests
	p.baseURL = baseURL
	return p
}

func (p *openRouterProvider) ID() ProviderID           { return ProviderOpenRouter }
func (p *openRouterProvider) IsConfigured() bool       { return p.apiKey != "" }
func (p *openRouterProvider) KeySource() KeySource     { return p.keySource }
func (p *openRouterProvider) Capabilities() Capability { return CapBgRemove | CapImageToImage }

const bgRemovePrompt = "Remove the background from this image. Return only the foreground on a transparent background."

func (p *openRouterProvider) BgRemove(ctx context.Context, imageData []byte, model string) ([]byte, error) {
	if model == "" {
		model = p.defaultBgModel
	}
	return p.editImage(ctx, imageData, model, bgRemovePrompt)
}

func (p *openRouterProvider) Transform(ctx context.Context, imageData []byte, prompt, model string) ([]byte, error) {
	if strings.TrimSpace(prompt) == "" {
		return nil, errors.New("ai/openrouter: prompt is required")
	}
	if model == "" {
		model = p.defaultI2IModel
	}
	return p.editImage(ctx, imageData, model, prompt)
}

func (p *openRouterProvider) ListModels(ctx context.Context) ([]Model, error) {
	const cacheKey = string(ProviderOpenRouter)
	if cached, ok := modelCache.Get(cacheKey); ok {
		return cached, nil
	}
	listCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()
	models, err := fetchOpenRouterModels(listCtx, p.apiKey, p.baseURL, p.httpClient)
	if err != nil {
		return nil, err
	}
	modelCache.Set(cacheKey, models, 0)
	return models, nil
}

func (p *openRouterProvider) ValidateKey(ctx context.Context) error {
	valCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
	defer cancel()

	req, err := http.NewRequestWithContext(valCtx, http.MethodGet, p.baseURL+"/auth/key", nil)
	if err != nil {
		return fmt.Errorf("openrouter: validate key: %w", err)
	}
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("openrouter: validate key: %w", err)
	}
	defer resp.Body.Close()
	_, _ = io.Copy(io.Discard, resp.Body)

	if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
		return ErrInvalidKey
	}
	if resp.StatusCode >= http.StatusBadRequest {
		return fmt.Errorf("openrouter: validate key: %w: status %d", ErrAPIError, resp.StatusCode)
	}
	return nil
}

type orChatResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
			Images  []struct {
				Type     string `json:"type"`
				ImageURL struct {
					URL string `json:"url"`
				} `json:"image_url"`
			} `json:"images"`
		} `json:"message"`
	} `json:"choices"`
}

type orErrorResponse struct {
	Error struct {
		Message string `json:"message"`
		Code    int    `json:"code"`
	} `json:"error"`
}

func (p *openRouterProvider) editImage(ctx context.Context, imageData []byte, model, prompt string) ([]byte, error) {
	if strings.TrimSpace(p.apiKey) == "" {
		return nil, errors.New("ai/openrouter: API key not configured")
	}

	_, contentType, err := detectImageFormat(imageData)
	if err != nil {
		return nil, err
	}

	dataURL := "data:" + contentType + ";base64," + base64.StdEncoding.EncodeToString(imageData)

	payload, err := json.Marshal(map[string]any{
		"model":      model,
		"modalities": []string{"image"},
		"messages": []map[string]any{
			{
				"role": "user",
				"content": []map[string]any{
					{"type": "text", "text": prompt},
					{"type": "image_url", "image_url": map[string]string{"url": dataURL}},
				},
			},
		},
	})
	if err != nil {
		return nil, fmt.Errorf("ai/openrouter: encode request: %w", err)
	}

	req, err := http.NewRequestWithContext(
		ctx,
		http.MethodPost,
		p.baseURL+"/chat/completions",
		bytes.NewReader(payload),
	)
	if err != nil {
		return nil, fmt.Errorf("ai/openrouter: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	p.setHeaders(req)

	resp, err := p.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("ai/openrouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("ai/openrouter: read response: %w", err)
	}

	if resp.StatusCode >= http.StatusBadRequest {
		var errResp orErrorResponse
		if jsonErr := json.Unmarshal(body, &errResp); jsonErr == nil && errResp.Error.Message != "" {
			return nil, fmt.Errorf("ai/openrouter: API error %d: %s", resp.StatusCode, errResp.Error.Message)
		}
		return nil, fmt.Errorf("ai/openrouter: API error %d", resp.StatusCode)
	}

	var chatResp orChatResponse
	if err = json.Unmarshal(body, &chatResp); err != nil {
		return nil, fmt.Errorf("ai/openrouter: decode response: %w", err)
	}
	if len(chatResp.Choices) == 0 {
		return nil, errors.New("ai/openrouter: empty response")
	}

	msg := chatResp.Choices[0].Message
	if len(msg.Images) == 0 {
		return nil, errors.New("ai/openrouter: response contains no images")
	}
	dataURLResp := msg.Images[0].ImageURL.URL
	// strip data:<mime>;base64, prefix
	_, after, ok := strings.Cut(dataURLResp, ";base64,")
	if !ok {
		return nil, errors.New("ai/openrouter: unexpected image URL format in response")
	}
	decoded, err := base64.StdEncoding.DecodeString(after)
	if err != nil {
		return nil, fmt.Errorf("ai/openrouter: decode image: %w", err)
	}
	return decoded, nil
}

func (p *openRouterProvider) setHeaders(req *http.Request) {
	req.Header.Set("Authorization", "Bearer "+p.apiKey)
	req.Header.Set("Http-Referer", openRouterReferer)
	req.Header.Set("X-Title", openRouterTitle)
}

func detectImageFormat(imageData []byte) (filename, contentType string, err error) {
	if len(imageData) == 0 {
		return "", "", errors.New("ai/openrouter: source image is empty")
	}
	switch http.DetectContentType(imageData) {
	case transform.MimeImagePNG:
		return "source.png", transform.MimeImagePNG, nil
	case transform.MimeImageJPEG:
		return "source.jpg", transform.MimeImageJPEG, nil
	case transform.MimeImageWebP:
		return "source.webp", transform.MimeImageWebP, nil
	case transform.MimeImageGIF:
		return "source.gif", transform.MimeImageGIF, nil
	default:
		return "", "", fmt.Errorf(
			"ai/openrouter: unsupported image format %q",
			http.DetectContentType(imageData),
		)
	}
}
