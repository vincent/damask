package api_test

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/imagerouter"
	th "damask/server/internal/tests_helpers"
)

func TestListImageRouterModelsReturnsHardcodedWhenNotConfigured(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "User", "models@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/imagerouter/models", nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.ImageRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if body.Configured {
		t.Fatal("expected configured=false")
	}
	if len(body.Models) != len(imagerouter.HardcodedModels) {
		t.Fatalf("expected hardcoded models, got %d", len(body.Models))
	}
}

func TestListImageRouterModelsProxiesAPIWhenConfigured(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		_, _ = w.Write([]byte(`[{"id":"bfl/flux","price":{"average":0.2}}]`))
	}))
	defer srv.Close()

	restore := imagerouter.SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	env := th.SetupTestApp(t, th.WithImageRouterAPIKey("test-key"))
	res := th.Register(t, env, "User", "models-config@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/imagerouter/models", nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.ImageRouterModelsResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if !body.Configured {
		t.Fatal("expected configured=true")
	}
	if len(body.Models) != 1 || body.Models[0].ID != "bfl/flux" {
		t.Fatalf("unexpected models: %#v", body.Models)
	}
}

func TestListImageRouterModelsRequiresAuth(t *testing.T) {
	env := th.SetupTestApp(t)
	req := httptest.NewRequest(http.MethodGet, "/api/v1/imagerouter/models", nil)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}
