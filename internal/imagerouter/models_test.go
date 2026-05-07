package imagerouter

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"
)

func TestFetchModelsSuccess(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.URL.Path + "?" + r.URL.RawQuery; got != "/v2/models?inputType=image&outputType=image" {
			t.Fatalf("unexpected URL: %s", got)
		}
		_ = json.NewEncoder(w).Encode([]map[string]any{
			{"id": "provider/b-model", "price": map[string]any{"average": 0.2}},
			{"id": "provider/a-model", "price": map[string]any{"average": 0.1}},
		})
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	models, err := FetchModels(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != 2 || models[0].ID != "provider/a-model" {
		t.Fatalf("unexpected models: %#v", models)
	}
	if models[0].Name != "A Model" || models[0].Provider != "provider" {
		t.Fatalf("unexpected normalized model: %#v", models[0])
	}
}

func TestFetchModelsFallbackOnError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		http.Error(w, "boom", http.StatusBadGateway)
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	models, err := FetchModels(context.Background(), "test-key")
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != len(HardcodedModels) {
		t.Fatalf("expected hardcoded fallback, got %d models", len(models))
	}
}

func TestFetchModelsFallbackOnTimeout(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(200 * time.Millisecond)
		_ = json.NewEncoder(w).Encode([]any{})
	}))
	defer srv.Close()

	restore := SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()
	models, err := FetchModels(ctx, "test-key")
	if err != nil {
		t.Fatalf("FetchModels: %v", err)
	}
	if len(models) != len(HardcodedModels) {
		t.Fatalf("expected hardcoded fallback, got %d models", len(models))
	}
}
