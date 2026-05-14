//go:build integration

package api_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/testutil"

	"github.com/gofiber/fiber/v3"
)

// setupTestSetupService creates an in-memory SetupService backed by memory repos.
func setupTestSetupService(t *testing.T) (*service.SetupService, string) {
	t.Helper()
	tmp := t.TempDir()
	users := memory.NewRealUserRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	workspaces.SetUserRepo(users)
	svc := service.NewSetupServiceWithCounter(users, workspaces, tmp, func(_ context.Context) (int64, error) {
		return users.Count(), nil
	})
	return svc, tmp
}

type setupTestApp struct {
	App *fiber.App
}

func newSetupTestEnv(t *testing.T, svc *service.SetupService) *setupTestApp {
	t.Helper()
	maker, err := auth.NewMaker("test-secret-key-must-be-32chars!!")
	if err != nil {
		t.Fatalf("auth.NewMaker: %v", err)
	}
	_, app := api.NewTestServer(&api.TestServerConfig{
		TokenMaker: maker,
		Setup:      svc,
	})
	return &setupTestApp{App: app}
}

// --- handleSetupStatus ---

func TestHandleSetupStatus_NotConfigured(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Configured  bool `json:"configured"`
		OwnerExists bool `json:"ownerExists"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if body.Configured {
		t.Error("expected configured=false")
	}
	if body.OwnerExists {
		t.Error("expected ownerExists=false")
	}
}

func TestHandleSetupStatus_Configured(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	// Write config so Configured=true.
	_ = svc.WriteConfig(bg(), service.EnvParams{
		Port:          14000,
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	})
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		Configured  bool `json:"configured"`
		OwnerExists bool `json:"ownerExists"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if !body.Configured {
		t.Error("expected configured=true")
	}
}

func TestHandleSetupStatus_BlockedAfterComplete(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	ctx := bg()
	_ = svc.WriteConfig(ctx, service.EnvParams{
		Port:          14000,
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	})
	_ = svc.CreateOwner(ctx, service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	})
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/status", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

// --- handleValidateStorage ---

func TestHandleValidateStorage_InvalidBody(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/validate-storage",
		testutil.JsonBody(map[string]any{"type": ""}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestHandleValidateStorage_StorageFails(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/validate-storage",
		testutil.JsonBody(map[string]any{
			"type":      "local",
			"localPath": "/this/path/absolutely/does/not/exist/9999",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)

	var body struct {
		OK     bool   `json:"ok"`
		Reason string `json:"reason"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if body.OK {
		t.Error("expected ok=false")
	}
	if body.Reason == "" {
		t.Error("expected non-empty reason")
	}
}

func TestHandleValidateStorage_OK(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/validate-storage",
		testutil.JsonBody(map[string]any{
			"type":      "local",
			"localPath": t.TempDir(),
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		OK bool `json:"ok"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if !body.OK {
		t.Error("expected ok=true")
	}
}

// --- handleSetupDeps ---

func TestHandleSetupDeps_ReturnsAllDeps(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/api/setup/deps", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body []map[string]any
	testutil.DecodeJSON(t, resp, &body)
	if len(body) == 0 {
		t.Error("expected at least one dep")
	}
}

// --- handleWriteConfig ---

func TestHandleWriteConfig_MissingFields(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/config",
		testutil.JsonBody(map[string]any{"port": 0}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestHandleWriteConfig_OK(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/config",
		testutil.JsonBody(map[string]any{
			"port":      14000,
			"type":      "local",
			"localPath": t.TempDir(),
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

// --- handleCreateOwner ---

func TestHandleCreateOwner_WeakPassword(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/owner",
		testutil.JsonBody(map[string]any{
			"workspaceName": "Acme",
			"name":          "Alice",
			"email":         "alice@example.com",
			"password":      "weak",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestHandleCreateOwner_OK(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/owner",
		testutil.JsonBody(map[string]any{
			"workspaceName": "Acme",
			"name":          "Alice",
			"email":         "alice@example.com",
			"password":      "hunter2hunter2",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusCreated)
}

func TestHandleCreateOwner_AlreadyExists(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	ctx := bg()
	_ = svc.CreateOwner(ctx, service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	})
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodPost, "/api/setup/owner",
		testutil.JsonBody(map[string]any{
			"workspaceName": "Acme2",
			"name":          "Bob",
			"email":         "bob@example.com",
			"password":      "hunter2hunter2",
		}))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusConflict)
}

// --- handleHealth ---

func TestHandleHealth_OK(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		OK            bool   `json:"ok"`
		Version       string `json:"version"`
		SetupRequired bool   `json:"setupRequired"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if !body.OK {
		t.Error("expected ok=true")
	}
	if !body.SetupRequired {
		t.Error("expected setupRequired=true for fresh server")
	}
}

func TestHandleHealth_SetupRequired(t *testing.T) {
	svc, _ := setupTestSetupService(t)
	ctx := bg()
	_ = svc.WriteConfig(ctx, service.EnvParams{
		Port:          14000,
		StorageParams: service.StorageParams{Type: "local", LocalPath: "/tmp"},
	})
	_ = svc.CreateOwner(ctx, service.OwnerParams{
		WorkspaceName: "Acme",
		Name:          "Alice",
		Email:         "alice@example.com",
		Password:      "hunter2hunter2",
	})
	env := newSetupTestEnv(t, svc)

	req := httptest.NewRequest(http.MethodGet, "/health", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var body struct {
		SetupRequired bool `json:"setupRequired"`
	}
	testutil.DecodeJSON(t, resp, &body)
	if body.SetupRequired {
		t.Error("expected setupRequired=false once configured with owner")
	}
}

// bg returns context.Background() — short alias for test readability.
func bg() context.Context { return context.Background() }
