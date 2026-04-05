package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestCreateWorkspace_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JsonStr(`{"name":"My New Workspace"}`), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body api.AuthResponse
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
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JsonStr(`{"name":""}`), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateWorkspace_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace",
		th.JsonStr(`{"name":"My Workspace"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestWorkspaceMe_Authenticated(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body api.WorkspaceMeResponse
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
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspace/me", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_AsOwner(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JsonStr(`{"email":"bob@example.com","role":"editor"}`), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var body api.InviteResponse
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
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")
	editorToken := th.MintEditorToken(t, env, owner.WorkspaceID, "editor")

	req := th.BearerRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JsonStr(`{"email":"carol@example.com","role":"viewer"}`), editorToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestCreateInvite_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JsonStr(`{"email":"bob@example.com","role":"editor"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite as owner
	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JsonStr(`{"email":"bob@example.com","role":"editor"}`), owner.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	if invResp.StatusCode != http.StatusCreated {
		t.Fatalf("create invite: expected 201, got %d", invResp.StatusCode)
	}

	var invite api.InviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Accept the invite
	acceptBody := `{"token":"` + invite.InviteToken + `","name":"Bob","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept", th.JsonStr(acceptBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("accept invite request: %v", err)
	}

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	cookie := th.FindCookie(resp, "auth_token")
	if cookie == nil {
		t.Fatal("expected auth_token cookie after accepting invite")
	}
}

func TestAcceptInvite_InvalidToken(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept",
		th.JsonStr(`{"token":"does-not-exist","name":"Bob","password":"password123"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListWorkspaces_SingleWorkspace(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodGet, "/api/v1/workspaces", nil, result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []api.WorkspaceWithRoleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 1 {
		t.Fatalf("expected 1 workspace, got %d", len(body))
	}
	if body[0].Role != "owner" {
		t.Errorf("role = %q, want %q", body[0].Role, "owner")
	}
	if body[0].ID != result.WorkspaceID {
		t.Errorf("workspace id = %q, want %q", body[0].ID, result.WorkspaceID)
	}
}

func TestListWorkspaces_MultipleWorkspaces(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a second workspace
	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JsonStr(`{"name":"Second Workspace"}`), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("create workspace request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("create workspace: expected 201, got %d", resp.StatusCode)
	}

	req = th.AuthRequest(http.MethodGet, "/api/v1/workspaces", nil, result.Cookie)
	resp, err = env.App.Test(req)
	if err != nil {
		t.Fatalf("list request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var body []api.WorkspaceWithRoleResponse
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body) != 2 {
		t.Fatalf("expected 2 workspaces, got %d", len(body))
	}
}

func TestListWorkspaces_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/workspaces", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_Success(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create a second workspace
	createReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace",
		th.JsonStr(`{"name":"Second Workspace"}`), result.Cookie)
	createResp, err := env.App.Test(createReq)
	if err != nil {
		t.Fatalf("create workspace request: %v", err)
	}
	var created api.AuthResponse
	if err := json.NewDecoder(createResp.Body).Decode(&created); err != nil {
		t.Fatalf("decode create: %v", err)
	}
	secondID := created.Workspace.ID

	// Switch to second workspace
	switchReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JsonStr(`{"workspace_id":"`+secondID+`"}`), result.Cookie)
	switchResp, err := env.App.Test(switchReq)
	if err != nil {
		t.Fatalf("switch request: %v", err)
	}
	if switchResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", switchResp.StatusCode)
	}

	var switchBody api.SwitchWorkspaceResponse
	if err := json.NewDecoder(switchResp.Body).Decode(&switchBody); err != nil {
		t.Fatalf("decode switch: %v", err)
	}
	if switchBody.Workspace.ID != secondID {
		t.Errorf("workspace id = %q, want %q", switchBody.Workspace.ID, secondID)
	}
	if switchBody.Role != "owner" {
		t.Errorf("role = %q, want owner", switchBody.Role)
	}

	// Verify new cookie reflects the new workspace
	newCookie := th.FindCookie(switchResp, "auth_token")
	if newCookie == nil {
		t.Fatal("expected new auth_token cookie")
	}
	meReq := th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, newCookie)
	meResp, err := env.App.Test(meReq)
	if err != nil {
		t.Fatalf("me request: %v", err)
	}
	var meBody api.WorkspaceMeResponse
	if err := json.NewDecoder(meResp.Body).Decode(&meBody); err != nil {
		t.Fatalf("decode me: %v", err)
	}
	if meBody.Workspace.ID != secondID {
		t.Errorf("me workspace id = %q, want %q", meBody.Workspace.ID, secondID)
	}
}

func TestSwitchWorkspace_NotMember(t *testing.T) {
	env := th.SetupTestApp(t)
	th.Register(t, env, "Alice", "alice@example.com", "password123")
	bob := th.Register(t, env, "Bob", "bob@example.com", "password123")

	// Alice's workspace ID is stored in result; Bob tries to access it
	alice, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/workspace/me", nil, bob.Cookie))
	var bobMe api.WorkspaceMeResponse
	json.NewDecoder(alice.Body).Decode(&bobMe) //nolint:errcheck

	// Re-register alice to get her workspace ID
	aliceResult := th.Register(t, env, "Alice2", "alice2@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JsonStr(`{"workspace_id":"`+aliceResult.WorkspaceID+`"}`), bob.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_InvalidWorkspaceID(t *testing.T) {
	env := th.SetupTestApp(t)
	result := th.Register(t, env, "Alice", "alice@example.com", "password123")

	req := th.AuthRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JsonStr(`{"workspace_id":"does-not-exist"}`), result.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403, got %d", resp.StatusCode)
	}
}

func TestSwitchWorkspace_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/workspace/switch",
		th.JsonStr(`{"workspace_id":"anything"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", resp.StatusCode)
	}
}

func TestAcceptInvite_ExpiredInvite(t *testing.T) {
	env := th.SetupTestApp(t)
	owner := th.Register(t, env, "Alice", "alice@example.com", "password123")

	// Create an invite
	invReq := th.AuthRequest(http.MethodPost, "/api/v1/workspace/invites",
		th.JsonStr(`{"email":"bob@example.com","role":"editor"}`), owner.Cookie)
	invResp, err := env.App.Test(invReq)
	if err != nil {
		t.Fatalf("create invite request: %v", err)
	}
	var invite api.InviteResponse
	if err := json.NewDecoder(invResp.Body).Decode(&invite); err != nil {
		t.Fatalf("decode invite: %v", err)
	}

	// Expire the invite directly in the DB
	_, err = env.SqlDB.Exec(
		`UPDATE workspace_invites SET expires_at = datetime('now', '-1 day') WHERE token = ?`,
		invite.InviteToken,
	)
	if err != nil {
		t.Fatalf("expire invite: %v", err)
	}

	// Attempt to accept the expired invite
	acceptBody := `{"token":"` + invite.InviteToken + `","name":"Bob","password":"password123"}`
	req := httptest.NewRequest(http.MethodPost, "/auth/invite/accept", th.JsonStr(acceptBody))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}

	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}
