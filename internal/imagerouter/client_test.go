package imagerouter

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func encodePNG(t *testing.T) []byte {
	t.Helper()
	img := image.NewNRGBA(image.Rect(0, 0, 2, 2))
	img.Set(0, 0, color.NRGBA{R: 255, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode png: %v", err)
	}
	return buf.Bytes()
}

func TestBgRemoveSuccess(t *testing.T) {
	expected := []byte("png-bytes")
	source := encodePNG(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/openai/images/edits":
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			files := r.MultipartForm.File["image[]"]
			if len(files) != 1 {
				t.Fatalf("expected 1 uploaded image, got %d", len(files))
			}
			if files[0].Filename != "source.png" {
				t.Fatalf("unexpected filename: %q", files[0].Filename)
			}
			if files[0].Header.Get("Content-Type") != "image/png" {
				t.Fatalf("unexpected content type: %q", files[0].Header.Get("Content-Type"))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1778159234,
				"data": []map[string]any{{"url": srv.URL + "/generated.png", "revised_prompt": nil}},
				"cost": 0.04,
				"latency": 3731,
			})
		case "/generated.png":
			_, _ = w.Write(expected)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	got, err := client.BgRemove(context.Background(), source, BgRemoveParams{Model: "bria/remove-background"})
	if err != nil {
		t.Fatalf("BgRemove: %v", err)
	}
	if string(got) != string(expected) {
		t.Fatalf("unexpected bytes: %q", got)
	}
}

func TestBgRemoveAPIError(t *testing.T) {
	source := encodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "bad request", http.StatusBadGateway)
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	_, err := client.BgRemove(context.Background(), source, BgRemoveParams{Model: "bria/remove-background"})
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
}

func TestTransformSuccess(t *testing.T) {
	expected := []byte("png-bytes")
	source := encodePNG(t)
	var gotPrompt string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/openai/images/edits":
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			gotPrompt = r.FormValue("prompt")
			files := r.MultipartForm.File["image[]"]
			if len(files) != 1 {
				t.Fatalf("expected 1 uploaded image, got %d", len(files))
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1778159234,
				"data": []map[string]any{{"url": srv.URL + "/generated.png", "revised_prompt": nil}},
				"cost": 0.04,
				"latency": 3731,
			})
		case "/generated.png":
			_, _ = w.Write(expected)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	got, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev",
	})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if gotPrompt != "soft matte lighting" {
		t.Fatalf("expected prompt to be forwarded, got %q", gotPrompt)
	}
	if string(got) != string(expected) {
		t.Fatalf("unexpected bytes: %q", got)
	}
}

func TestTransformEmptyPrompt(t *testing.T) {
	source := encodePNG(t)
	client := NewClient("test-key", false)
	_, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "   ",
		Model:  "black-forest-labs/FLUX.1-fill-dev",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}

func TestClientTimeout(t *testing.T) {
	source := encodePNG(t)
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/openai/images/edits":
			time.Sleep(100 * time.Millisecond)
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1778159234,
				"data": []map[string]any{{"url": srv.URL + "/generated.png", "revised_prompt": nil}},
				"cost": 0.04,
				"latency": 3731,
			})
		case "/generated.png":
			_, _ = w.Write([]byte("png"))
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Millisecond)
	defer cancel()
	_, err := client.BgRemove(ctx, source, BgRemoveParams{Model: "bria/remove-background"})
	if err == nil {
		t.Fatal("expected timeout error")
	}
}

func TestTransformRetriesWithoutFreeSuffixOnConfiguredFreeLimit429(t *testing.T) {
	expected := []byte("png-bytes")
	source := encodePNG(t)
	var models []string
	var srv *httptest.Server
	srv = httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch r.URL.Path {
		case "/v1/openai/images/edits":
			if err := r.ParseMultipartForm(1024 * 1024); err != nil {
				t.Fatalf("parse form: %v", err)
			}
			models = append(models, r.FormValue("model"))
			if len(models) == 1 {
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"error":{"message":"Daily limit of 3 free requests reached. Remove \":free\" from the model name to continue with the paid model."}}`))
				return
			}
			_ = json.NewEncoder(w).Encode(map[string]any{
				"created": 1778159234,
				"data": []map[string]any{{"url": srv.URL + "/generated.png", "revised_prompt": nil}},
				"cost": 0.04,
				"latency": 3731,
			})
		case "/generated.png":
			_, _ = w.Write(expected)
		default:
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", true)
	got, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev:free",
	})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if len(models) != 2 {
		t.Fatalf("expected 2 requests, got %d", len(models))
	}
	if models[0] != "black-forest-labs/FLUX.1-fill-dev:free" {
		t.Fatalf("unexpected first model: %q", models[0])
	}
	if models[1] != "black-forest-labs/FLUX.1-fill-dev" {
		t.Fatalf("unexpected retry model: %q", models[1])
	}
	if string(got) != string(expected) {
		t.Fatalf("unexpected bytes: %q", got)
	}
}

func TestTransformDoesNotRetryWithoutConfigOnFreeLimit429(t *testing.T) {
	source := encodePNG(t)
	var requests int
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/openai/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		requests++
		w.WriteHeader(http.StatusTooManyRequests)
		_, _ = w.Write([]byte(`{"error":{"message":"Daily limit of 3 free requests reached. Remove \":free\" from the model name to continue with the paid model."}}`))
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	_, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev:free",
	})
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	if requests != 1 {
		t.Fatalf("expected 1 request, got %d", requests)
	}
}

func TestTransformFallsBackToB64JSONWhenPresent(t *testing.T) {
	expected := []byte("png-bytes")
	source := encodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/openai/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"data": []map[string]string{{"b64_json": base64.StdEncoding.EncodeToString(expected)}},
		})
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	got, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev",
	})
	if err != nil {
		t.Fatalf("Transform: %v", err)
	}
	if string(got) != string(expected) {
		t.Fatalf("unexpected bytes: %q", got)
	}
}

func TestTransformReturnsStructuredAPIErrorWhenSuccessDecodeFails(t *testing.T) {
	source := encodePNG(t)
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/v1/openai/images/edits" {
			t.Fatalf("unexpected path: %s", r.URL.Path)
		}
		_ = json.NewEncoder(w).Encode(map[string]any{
			"status":     http.StatusBadRequest,
			"statusText": "invalidPositivePrompt",
			"error": map[string]any{
				"message": "Invalid value for 'prompt' parameter. Prompt must be a string value between 1 and 10000 characters.",
				"type":    "invalidPositivePrompt",
			},
		})
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	client := NewClient("test-key", false)
	_, err := client.Transform(context.Background(), source, PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev",
	})
	if !errors.Is(err, ErrAPIError) {
		t.Fatalf("expected ErrAPIError, got %v", err)
	}
	want := "Invalid value for 'prompt' parameter. Prompt must be a string value between 1 and 10000 characters."
	if err == nil || err.Error() != want {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestTransformRejectsUnsupportedSourceFormat(t *testing.T) {
	client := NewClient("test-key", false)
	_, err := client.Transform(context.Background(), []byte("not-an-image"), PromptParams{
		Prompt: "soft matte lighting",
		Model:  "black-forest-labs/FLUX.1-fill-dev",
	})
	if err == nil {
		t.Fatal("expected error")
	}
}
