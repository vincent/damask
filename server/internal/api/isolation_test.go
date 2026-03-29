package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// setupTwoWorkspaces registers two independent owners, each in their own workspace.
func setupTwoWorkspaces(t *testing.T) (env *testEnv, ws1 authResult, ws2 authResult) {
	t.Helper()
	env = setupTestApp(t)
	ws1 = register(t, env, "Alice", "alice@example.com", "password123")
	ws2 = register(t, env, "Bob", "bob@example.com", "password456")
	return
}

// --- Projects ---

func TestIsolation_ListProjects(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := authRequest(http.MethodGet, "/api/v1/projects", nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var projects []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&projects); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(projects) != 0 {
		t.Errorf("ws2 should see 0 projects, got %d", len(projects))
	}
}

func TestIsolation_GetProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := authRequest(http.MethodGet, "/api/v1/projects/"+p.ID, nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := authRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		jsonStr(`{"name":"Hacked"}`), ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := authRequest(http.MethodDelete, "/api/v1/projects/"+p.ID, nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Folders ---

func TestIsolation_CreateFolderInOtherProject(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")

	req := authRequest(http.MethodPost, fmt.Sprintf("/api/v1/projects/%s/folders", p.ID),
		jsonStr(`{"name":"Intruder"}`), ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_GetFolders(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	createFolderHelper(t, env, ws1.Cookie, p.ID, "Folder A", nil)

	req := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/projects/%s/folders", p.ID), nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	folder := createFolderHelper(t, env, ws1.Cookie, p.ID, "Folder A", nil)
	folderID, _ := folder["id"].(string)

	req := authRequest(http.MethodPut, "/api/v1/folders/"+folderID,
		jsonStr(`{"name":"Hacked"}`), ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	p := createProject(t, env, ws1.Cookie, "WS1 Project", "#ff0000")
	folder := createFolderHelper(t, env, ws1.Cookie, p.ID, "Folder A", nil)
	folderID, _ := folder["id"].(string)

	req := authRequest(http.MethodDelete, "/api/v1/folders/"+folderID, nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Assets ---

func TestIsolation_ListAssets(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodGet, "/api/v1/assets", nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Data []interface{} `json:"data"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(body.Data) != 0 {
		t.Errorf("ws2 should see 0 assets, got %d", len(body.Data))
	}
}

func TestIsolation_GetAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_GetAssetFile(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/file", nil, ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_UpdateAssetFolder(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodPatch, "/api/v1/assets/"+assetID,
		jsonStr(`{"folder_id":null}`), ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_DeleteAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodDelete, "/api/v1/assets/"+assetID, nil, ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Tags ---

func TestIsolation_ListTags(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	// Add a tag to ws1's asset so there is something to be isolated
	assetID := uploadTestAsset(t, env, ws1)
	tagReq := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags",
		jsonStr(`{"name":"secret-tag"}`), ws1.Cookie)
	env.app.Test(tagReq, fiber.TestConfig{Timeout: 5000}) //nolint:errcheck

	req := authRequest(http.MethodGet, "/api/v1/tags", nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var tags []interface{}
	if err := json.NewDecoder(resp.Body).Decode(&tags); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(tags) != 0 {
		t.Errorf("ws2 should see 0 tags, got %d", len(tags))
	}
}

func TestIsolation_GetAssetTags(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestIsolation_AddTagToAsset(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	assetID := uploadTestAsset(t, env, ws1)

	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags",
		jsonStr(`{"name":"intruder"}`), ws2.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- Workspace ---

func TestIsolation_WorkspaceMe(t *testing.T) {
	env, ws1, ws2 := setupTwoWorkspaces(t)

	req := authRequest(http.MethodGet, "/api/v1/workspace/me", nil, ws2.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var body struct {
		Workspace struct {
			ID string `json:"id"`
		} `json:"workspace"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&body); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if body.Workspace.ID != ws2.WorkspaceID {
		t.Errorf("ws2 got workspace %q, want %q", body.Workspace.ID, ws2.WorkspaceID)
	}
	if body.Workspace.ID == ws1.WorkspaceID {
		t.Error("ws2 must not see ws1's workspace")
	}
}
