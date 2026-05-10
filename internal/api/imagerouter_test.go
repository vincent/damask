//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/imagerouter"
	th "damask/server/internal/tests_helpers"
	"damask/server/internal/testutil"
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

func TestListImageRouterModelsUsesWorkspaceService(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.ListImageRouterModelsFn = func(_ context.Context, workspaceID string) ([]imagerouter.Model, imagerouter.KeyStatus, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q", workspaceID)
		}
		return []imagerouter.Model{{ID: "svc/model"}}, imagerouter.KeyStatus{
			KeySet: true,
			Source: imagerouter.SourceWorkspace,
		}, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/imagerouter/models", nil, env.MintToken(t, "usr_1", "ws_1"))
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
	if len(body.Models) != 1 || body.Models[0].ID != "svc/model" {
		t.Fatalf("unexpected models: %#v", body.Models)
	}
}
