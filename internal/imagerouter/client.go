package imagerouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"mime/multipart"
	"net/http"
	"net/textproto"
	"strings"
	"time"
)

var (
	ErrNotConfigured = errors.New("imagerouter: API key not configured")
	ErrInvalidModel  = errors.New("imagerouter: model not found or not image2image")
	ErrAPIError      = errors.New("imagerouter: API returned non-2xx")
)

var apiBaseURL = "https://api.imagerouter.io/v1"

type Client struct {
	apiKey                  string
	httpClient              *http.Client
	retryPaidOnFreeLimit429 bool
}

type BgRemoveParams struct {
	Model string
}

type PromptParams struct {
	Prompt string
	Model  string
}

type imageResponseEnvelope struct {
	Created int64 `json:"created"`
	Data    []struct {
		URL           string  `json:"url"`
		B64JSON       string  `json:"b64_json"`
		RevisedPrompt *string `json:"revised_prompt"`
	} `json:"data"`
	Cost    float64 `json:"cost"`
	Latency int     `json:"latency"`
}

type errorResponseEnvelope struct {
	Status     int    `json:"status"`
	StatusText string `json:"statusText"`
	Error      struct {
		Message string `json:"message"`
		Type    string `json:"type"`
	} `json:"error"`
}

type apiError struct {
	cause   error
	message string
}

func (e *apiError) Error() string {
	return e.message
}

func (e *apiError) Unwrap() error {
	return e.cause
}

func NewClient(apiKey string, retryPaidOnFreeLimit429 bool) *Client {
	return &Client{
		apiKey:                  apiKey,
		retryPaidOnFreeLimit429: retryPaidOnFreeLimit429,
		httpClient: &http.Client{
			Timeout: 120 * time.Second,
		},
	}
}

// SetBaseURLForTest overrides the API base URL and returns a restore function.
func SetBaseURLForTest(raw string) func() {
	prev := apiBaseURL
	apiBaseURL = strings.TrimRight(raw, "/")
	return func() {
		apiBaseURL = prev
	}
}

func (c *Client) BgRemove(ctx context.Context, imageData []byte, p BgRemoveParams) ([]byte, error) {
	if strings.TrimSpace(p.Model) == "" {
		return nil, fmt.Errorf("%w: empty model", ErrInvalidModel)
	}
	return c.editImage(ctx, imageData, p.Model, "")
}

func (c *Client) Transform(ctx context.Context, imageData []byte, p PromptParams) ([]byte, error) {
	if strings.TrimSpace(p.Model) == "" {
		return nil, fmt.Errorf("%w: empty model", ErrInvalidModel)
	}
	if strings.TrimSpace(p.Prompt) == "" {
		return nil, fmt.Errorf("imagerouter: prompt is required")
	}
	return c.editImage(ctx, imageData, p.Model, p.Prompt)
}

func (c *Client) editImage(ctx context.Context, imageData []byte, model, prompt string) ([]byte, error) {
	if strings.TrimSpace(c.apiKey) == "" {
		return nil, ErrNotConfigured
	}

	result, statusCode, payload, err := c.performEditImage(ctx, imageData, model, prompt)
	if err == nil {
		return result, nil
	}
	if !c.shouldRetryWithoutFreeSuffix(statusCode, model, payload) {
		return nil, err
	}

	retryModel := strings.TrimSuffix(model, ":free")
	slog.Info("imagerouter retrying without free suffix after free-tier limit", "model", model, "retry_model", retryModel)
	result, _, _, retryErr := c.performEditImage(ctx, imageData, retryModel, prompt)
	if retryErr != nil {
		return nil, retryErr
	}
	return result, nil
}

func (c *Client) performEditImage(ctx context.Context, imageData []byte, model, prompt string) ([]byte, int, string, error) {
	filename, contentType, err := detectImageUpload(imageData)
	if err != nil {
		return nil, 0, "", err
	}

	body := &bytes.Buffer{}
	writer := multipart.NewWriter(body)
	if err := writer.WriteField("model", model); err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: write model field: %w", err)
	}
	if err := writer.WriteField("prompt", prompt); err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: write prompt field: %w", err)
	}
	header := make(textproto.MIMEHeader)
	header.Set("Content-Disposition", fmt.Sprintf(`form-data; name="image[]"; filename="%s"`, filename))
	header.Set("Content-Type", contentType)
	part, err := writer.CreatePart(header)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: create image part: %w", err)
	}
	if _, err := part.Write(imageData); err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: write image part: %w", err)
	}
	if err := writer.Close(); err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: close multipart: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, apiBaseURL+"/openai/images/edits", body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: build request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	req.Header.Set("Content-Type", writer.FormDataContentType())

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: request failed: %w", err)
	}
	defer resp.Body.Close()

	payload, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, 0, "", fmt.Errorf("imagerouter: read response: %w", err)
	}
	payloadText := strings.TrimSpace(string(payload))

	if resp.StatusCode == http.StatusNotFound || resp.StatusCode == http.StatusUnprocessableEntity {
		return nil, resp.StatusCode, payloadText, wrapImageRouterAPIError(ErrInvalidModel, payload)
	}
	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return nil, resp.StatusCode, payloadText, wrapImageRouterAPIError(ErrAPIError, payload)
	}

	var envelope imageResponseEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, resp.StatusCode, payloadText, parseImageRouterAPIError(ErrAPIError, payload, fmt.Errorf("imagerouter: decode response: %w", err))
	}
	if len(envelope.Data) == 0 {
		return nil, resp.StatusCode, payloadText, parseImageRouterAPIError(ErrAPIError, payload, &apiError{
			cause:   ErrAPIError,
			message: "ImageRouter generation failed",
		})
	}

	item := envelope.Data[0]
	if strings.TrimSpace(item.URL) != "" {
		downloaded, err := c.fetchImageURL(ctx, item.URL)
		if err != nil {
			return nil, resp.StatusCode, payloadText, err
		}
		return downloaded, resp.StatusCode, payloadText, nil
	}
	if strings.TrimSpace(item.B64JSON) == "" {
		return nil, resp.StatusCode, payloadText, fmt.Errorf("imagerouter: response missing image url")
	}
	decoded, err := base64.StdEncoding.DecodeString(item.B64JSON)
	if err != nil {
		return nil, resp.StatusCode, payloadText, fmt.Errorf("imagerouter: decode image: %w", err)
	}
	return decoded, resp.StatusCode, payloadText, nil
}

func wrapImageRouterAPIError(base error, payload []byte) error {
	return parseImageRouterAPIError(base, payload, nil)
}

func parseImageRouterAPIError(base error, payload []byte, fallback error) error {
	var envelope errorResponseEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		if fallback != nil {
			return fallback
		}
		return &apiError{cause: base, message: "ImageRouter generation failed"}
	}
	if msg := strings.TrimSpace(envelope.Error.Message); msg != "" {
		return &apiError{cause: base, message: msg}
	}
	if fallback != nil {
		return fallback
	}
	return &apiError{cause: base, message: "ImageRouter generation failed"}
}

func (c *Client) fetchImageURL(ctx context.Context, rawURL string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, rawURL, nil)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: build image download request: %w", err)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: download image: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		payload, _ := io.ReadAll(resp.Body)
		return nil, fmt.Errorf("imagerouter: image download failed: status=%d body=%s", resp.StatusCode, strings.TrimSpace(string(payload)))
	}

	downloaded, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, fmt.Errorf("imagerouter: read downloaded image: %w", err)
	}
	if len(downloaded) == 0 {
		return nil, fmt.Errorf("imagerouter: downloaded image is empty")
	}
	return downloaded, nil
}

func detectImageUpload(imageData []byte) (filename string, contentType string, err error) {
	if len(imageData) == 0 {
		return "", "", fmt.Errorf("imagerouter: source image is empty")
	}

	switch http.DetectContentType(imageData) {
	case "image/png":
		return "source.png", "image/png", nil
	case "image/jpeg":
		return "source.jpg", "image/jpeg", nil
	case "image/webp":
		return "source.webp", "image/webp", nil
	default:
		return "", "", fmt.Errorf("imagerouter: unsupported source image format %q; expected PNG, JPEG, or WEBP", http.DetectContentType(imageData))
	}
}

func (c *Client) shouldRetryWithoutFreeSuffix(statusCode int, model, payload string) bool {
	if !c.retryPaidOnFreeLimit429 {
		return false
	}
	if statusCode != http.StatusTooManyRequests {
		return false
	}
	if !strings.HasSuffix(model, ":free") {
		return false
	}

	lowerPayload := strings.ToLower(payload)
	return strings.Contains(lowerPayload, "remove \":free\"") ||
		(strings.Contains(lowerPayload, "free requests reached") && strings.Contains(lowerPayload, "daily limit"))
}
