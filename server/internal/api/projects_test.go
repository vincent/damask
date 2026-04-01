package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"

	"github.com/gofiber/fiber/v3"
)

// createProject is a test helper that POSTs to /api/v1/projects.
func createProject(t *testing.T, env *testEnv, cookie *http.Cookie, name, color string) projectResponse {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q,"color":%q}`, name, color)
	req := authRequest(http.MethodPost, "/api/v1/projects", strings.NewReader(body), cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var p projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode project: %v", err)
	}
	return p
}

func TestCreateProject_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/projects",
		jsonStr(`{"name":"Campaign 2024","color":"#3b82f6","description":"Summer campaign"}`),
		owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var p projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&p); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if p.Name != "Campaign 2024" {
		t.Errorf("name = %q, want Campaign 2024", p.Name)
	}
	if p.Color == nil || *p.Color != "#3b82f6" {
		t.Errorf("color = %v, want #3b82f6", p.Color)
	}
	if p.Description == nil || *p.Description != "Summer campaign" {
		t.Errorf("description = %v, want Summer campaign", p.Description)
	}
	if p.WorkspaceID != owner.WorkspaceID {
		t.Errorf("workspace_id mismatch")
	}
}

func TestCreateProject_MissingName(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/projects", jsonStr(`{"name":""}`), owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateProject_ViewerRejected(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	viewerToken := mintEditorToken(t, env, owner.WorkspaceID, "viewer")

	req := bearerRequest(http.MethodPost, "/api/v1/projects", jsonStr(`{"name":"My Project"}`), viewerToken)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListProjects_Empty(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/projects", nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var items []projectResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 projects, got %d", len(items))
	}
}

func TestListProjects_WithCount(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	p := createProject(t, env, owner.Cookie, "Alpha", "#ff0000")

	// Upload an asset and assign it to the project
	assetID := uploadTestAsset(t, env, owner)
	_, err := env.sqlDB.Exec(
		`UPDATE assets SET project_id = ? WHERE id = ?`,
		p.ID, assetID,
	)
	if err != nil {
		t.Fatalf("assign project: %v", err)
	}

	req := authRequest(http.MethodGet, "/api/v1/projects", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	var items []projectResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck

	if len(items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(items))
	}
	if items[0].AssetCount != 1 {
		t.Errorf("asset_count = %d, want 1", items[0].AssetCount)
	}
}

func TestGetProject_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "MyProject", "#00ff00")

	req := authRequest(http.MethodGet, "/api/v1/projects/"+p.ID, nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got projectResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.ID != p.ID {
		t.Errorf("id mismatch: got %s, want %s", got.ID, p.ID)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/projects/nonexistent", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateProject(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Old Name", "#aabbcc")

	req := authRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		jsonStr(`{"name":"New Name","color":"#112233"}`), owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got projectResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.Name != "New Name" {
		t.Errorf("name = %q, want New Name", got.Name)
	}
	if got.Color == nil || *got.Color != "#112233" {
		t.Errorf("color = %v, want #112233", got.Color)
	}
}

func TestDeleteProject_UnlinksAssets(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Temp", "#000000")

	assetID := uploadTestAsset(t, env, owner)
	env.sqlDB.Exec(`UPDATE assets SET project_id = ? WHERE id = ?`, p.ID, assetID) //nolint:errcheck

	// Delete the project
	req := authRequest(http.MethodDelete, "/api/v1/projects/"+p.ID, nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify asset still exists but project_id is NULL
	var projectID *string
	row := env.sqlDB.QueryRow(`SELECT project_id FROM assets WHERE id = ?`, assetID)
	if err := row.Scan(&projectID); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if projectID != nil {
		t.Errorf("expected project_id to be NULL after deletion, got %v", *projectID)
	}
}

// uploadTestAsset is a small helper that uploads a JPEG and returns its ID.
func uploadTestAsset(t *testing.T, env *testEnv, owner authResult) string {
	t.Helper()
	jpegData := makeJPEG(10, 10)
	req := buildUploadRequest(t, "test.jpg", jpegData, owner.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	var a assetResponse
	json.NewDecoder(resp.Body).Decode(&a) //nolint:errcheck
	return a.ID
}
