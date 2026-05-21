//go:build integration

package api_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/imagerouter"
	th "damask/server/internal/testhelpers"
	"damask/server/internal/testutil"
)

func TestWorkspaceImageRouterStatusOwnerAccess(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Owner", "owner-ir@test.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/settings/imagerouter", nil, res.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.WorkspaceImageRouterStatusResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.KeySet {
		t.Fatal("expected key_set=false")
	}
}

func TestWorkspaceImageRouterMemberDenied(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Owner", "member-ir@test.com", "password123")
	editorToken := th.MintEditorToken(t, env, res.WorkspaceID, auth.Editor)

	req := th.BearerRequest(http.MethodGet, "/api/v1/workspace/settings/imagerouter", nil, editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestWorkspaceImageRouterUnauthenticatedDenied(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/settings/imagerouter", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWorkspaceImageRouterPutAndGetOmitPlaintext(t *testing.T) {
	env := th.SetupTestApp(t)
	res := th.Register(t, env, "Owner", "put-ir@test.com", "password123")

	putReq := th.AuthRequest(http.MethodPut, "/api/v1/workspace/settings/imagerouter",
		th.JSONBody(map[string]string{"key": "secret-key"}), res.Cookie)
	putResp, err := env.App.Test(putReq)
	if err != nil {
		t.Fatal(err)
	}
	if putResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", putResp.StatusCode)
	}

	getReq := th.AuthRequest(http.MethodGet, "/api/v1/workspace/settings/imagerouter", nil, res.Cookie)
	getResp, err := env.App.Test(getReq)
	if err != nil {
		t.Fatal(err)
	}
	if getResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", getResp.StatusCode)
	}

	var body map[string]any
	if err := json.NewDecoder(getResp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if _, ok := body["key"]; ok {
		t.Fatal("response should not include plaintext key")
	}
	if body["source"] != string(imagerouter.SourceWorkspace) {
		t.Fatalf("unexpected source: %#v", body["source"])
	}
}

func TestWorkspaceImageRouterTestInvalidKeyMaps422(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workspace.TestImageRouterKeyFn = func(_ context.Context, _ string) error {
		return imagerouter.ErrInvalidKey
	}

	req := testutil.BearerRequest(http.MethodPost, "/api/v1/workspace/settings/imagerouter/test", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}
