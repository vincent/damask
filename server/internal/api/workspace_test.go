package api

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateWorkspace_Success(t *testing.T) {
	env := setupTestApp(t)
	result := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/workspace",
		jsonStr(`{"name":"My New Workspace"}`), result.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body authResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Workspace == nil {
		t.Fatal("expected workspace in response")
	}
	if body.Workspace.Name != "My New Workspace" {
		t.Errorf("workspace name = %q, want %q", body.Workspace.Name, "My New Workspace")
	}
	if body.Workspace.ID == result.WorkspaceID {
		t.Error("new workspace should have a different ID than the original")
	}
}

func TestCreateWorkspace_MissingName(t *testing.T) {
	env := setupTestApp(t)
	result := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/workspace",
		jsonStr(`{"name":""}`), result.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateWorkspace_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace",
		jsonStr(`{"name":"My Workspace"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWorkspaceMe_Authenticated(t *testing.T) {
	env := setupTestApp(t)
	result := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/workspace/me", nil, result.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body workspaceMeResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.User.Email != "alice@example.com" {
		t.Errorf("user email = %q, want %q", body.User.Email, "alice@example.com")
	}
	if body.Role != "owner" {
		t.Errorf("role = %q, want %q", body.Role, "owner")
	}
}

func TestWorkspaceMe_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/me", nil)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_AsOwner(t *testing.T) {
	env := setupTestApp(t)
	result := register(t, env, "Alice", "alice@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/workspace/invites",
		jsonStr(`{"email":"bob@example.com","role":"editor"}`), result.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body inviteResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.InviteToken == "" {
		t.Error("expected non-empty invite_token")
	}
	if body.Email != "bob@example.com" {
		t.Errorf("invite email = %q, want %q", body.Email, "bob@example.com")
	}
}

func TestCreateInvite_AsEditor_Forbidden(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := mintEditorToken(t, env, owner.WorkspaceID, "editor")

	req := bearerRequest(http.MethodPost, "/api/v1/workspace/invites",
		jsonStr(`{"email":"carol@example.com","role":"viewer"}`), editorToken)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/invites",
		jsonStr(`{"email":"bob@example.com","role":"editor"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite as owner
	invReq := authRequest(http.MethodPost, "/api/v1/workspace/invites",
		jsonStr(`{"email":"bob@example.com","role":"editor"}`), owner.Cookie)
	invResp, err := env.app.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	if invResp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite: expected 201, got %d", invResp.StatusCode)
	}

	var invite inviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Accept the invite
	acceptBody := `{"token":"` + invite.InviteToken + `","name":"Bob","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept", jsonStr(acceptBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("accept invite request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	cookie := findCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("expected auth_token cookie after accepting invite")
	}
}

func TestAcceptInvite_InvalidToken(t *testing.T) {
	env := setupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept",
		jsonStr(`{"token":"does-not-exist","name":"Bob","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_ExpiredInvite(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite
	invReq := authRequest(http.MethodPost, "/api/v1/workspace/invites",
		jsonStr(`{"email":"bob@example.com","role":"editor"}`), owner.Cookie)
	invResp, err := env.app.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	var invite inviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Expire the invite directly in the DB
	_, err = env.sqlDB.Exec(
		`UPDATE workspace_invites SET expires_at = datetime('now', '-1 day') WHERE token = ?`,
		invite.InviteToken,
	)
	if err != nil {
		t.Fatalf("expire invite: %v", err)
	}

	// Attempt to accept the expired invite
	acceptBody := `{"token":"` + invite.InviteToken + `","name":"Bob","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept", jsonStr(acceptBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
