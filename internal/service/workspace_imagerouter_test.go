package service

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/imagerouter"
	"damask/server/internal/ingress"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
)

type stubImageRouterValidator struct {
	err error
}

func (v stubImageRouterValidator) Validate(context.Context) error { return v.err }

func newWorkspaceServiceForImageRouterTests(
	t *testing.T,
	envKey string,
) (*workspaceService, *memory.RealWorkspaceRepo) {
	t.Helper()
	repo := memory.NewRealWorkspaceRepo()
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme"})
	return &workspaceService{
		workspaces: repo,
		users:      memory.NewUserRepo(),
		appSecret:  "test-secret",
		envIRKey:   envKey,
		newIRClient: func(string) imageRouterValidator {
			return stubImageRouterValidator{}
		},
	}, repo
}

func TestWorkspaceServiceSetImageRouterKeyEncryptsAtRest(t *testing.T) {
	t.Parallel()
	svc, repo := newWorkspaceServiceForImageRouterTests(t, "")

	if err := svc.SetImageRouterKey(context.Background(), "ws_1", "plain-key"); err != nil {
		t.Fatalf("SetImageRouterKey: %v", err)
	}

	encKey, err := repo.GetImageRouterKey(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("GetImageRouterKey: %v", err)
	}
	if encKey == "" || encKey == "plain-key" {
		t.Fatalf("expected encrypted key, got %q", encKey)
	}

	plain, err := ingress.DecryptConfig("test-secret", encKey)
	if err != nil {
		t.Fatalf("DecryptConfig: %v", err)
	}
	if string(plain) != "plain-key" {
		t.Fatalf("decrypted key = %q, want plain-key", string(plain))
	}
}

func TestWorkspaceServiceGetImageRouterKeyStatusUsesEnvFallback(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")

	status, err := svc.GetImageRouterKeyStatus(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("GetImageRouterKeyStatus: %v", err)
	}
	if !status.KeySet || status.Source != imagerouter.SourceEnv {
		t.Fatalf("unexpected status: %+v", status)
	}
}

func TestWorkspaceServiceListImageRouterModelsReturnsHardcodedWhenNotConfigured(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "")

	models, status, err := svc.ListImageRouterModels(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("ListImageRouterModels: %v", err)
	}
	if status.KeySet || status.Source != imagerouter.SourceNone {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(models) != len(imagerouter.HardcodedModels) {
		t.Fatalf("expected %d hardcoded models, got %d", len(imagerouter.HardcodedModels), len(models))
	}
}

func TestWorkspaceServiceListImageRouterModelsUsesResolvedKey(t *testing.T) {
	t.Parallel()
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Authorization"); got != "Bearer env-key" {
			t.Fatalf("Authorization header = %q", got)
		}
		_, _ = w.Write([]byte(`[{"id":"bfl/flux","price":{"average":0.2}}]`))
	}))
	defer srv.Close()

	restore := imagerouter.SetBaseURLForTest(srv.URL + "/v1")
	defer restore()

	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")

	models, status, err := svc.ListImageRouterModels(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("ListImageRouterModels: %v", err)
	}
	if !status.KeySet || status.Source != imagerouter.SourceEnv {
		t.Fatalf("unexpected status: %+v", status)
	}
	if len(models) != 1 || models[0].ID != "bfl/flux" {
		t.Fatalf("unexpected models: %#v", models)
	}
}

func TestWorkspaceServiceClearImageRouterKey(t *testing.T) {
	t.Parallel()
	svc, repo := newWorkspaceServiceForImageRouterTests(t, "")
	if err := svc.SetImageRouterKey(context.Background(), "ws_1", "plain-key"); err != nil {
		t.Fatalf("SetImageRouterKey: %v", err)
	}

	if err := svc.ClearImageRouterKey(context.Background(), "ws_1"); err != nil {
		t.Fatalf("ClearImageRouterKey: %v", err)
	}

	key, err := repo.GetImageRouterKey(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("GetImageRouterKey: %v", err)
	}
	if key != "" {
		t.Fatalf("expected cleared key, got %q", key)
	}
}

func TestWorkspaceServiceSetImageRouterKeyRejectsEmpty(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "")

	err := svc.SetImageRouterKey(context.Background(), "ws_1", "   ")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkspaceServiceTestImageRouterKeyUsesResolvedKey(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")
	var gotKey string
	svc.newIRClient = func(apiKey string) imageRouterValidator {
		gotKey = apiKey
		return stubImageRouterValidator{}
	}

	if err := svc.TestImageRouterKey(context.Background(), "ws_1"); err != nil {
		t.Fatalf("TestImageRouterKey: %v", err)
	}
	if gotKey != "env-key" {
		t.Fatalf("got key %q, want env-key", gotKey)
	}
}

func TestWorkspaceServiceTestImageRouterKeyInvalidKey(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "env-key")
	svc.newIRClient = func(string) imageRouterValidator {
		return stubImageRouterValidator{err: imagerouter.ErrInvalidKey}
	}

	err := svc.TestImageRouterKey(context.Background(), "ws_1")
	if !errors.Is(err, imagerouter.ErrInvalidKey) {
		t.Fatalf("expected ErrInvalidKey, got %v", err)
	}
}

func TestWorkspaceServiceTestImageRouterKeyNoConfiguredKey(t *testing.T) {
	t.Parallel()
	svc, _ := newWorkspaceServiceForImageRouterTests(t, "")

	err := svc.TestImageRouterKey(context.Background(), "ws_1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}
