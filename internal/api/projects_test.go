package api_test

import (
	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"net/http"
	"testing"
)

func TestCreateProject_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	color := "#3b82f6"
	description := "Summer campaign"
	req := th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{
			Name:        "Campaign 2024",
			Color:       &color,
			Description: &description,
		}),
		owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}

	var p api.ProjectResponse
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
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: ""}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateProject_ViewerRejected(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)

	req := th.BearerRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "My Project"}), viewerToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListProjects_Empty(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var items []api.ProjectResponse
	if err := json.NewDecoder(resp.Body).Decode(&items); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(items) != 0 {
		t.Errorf("expected 0 projects, got %d", len(items))
	}
}

func TestListProjects_WithCount(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	p := th.CreateProject(t, env, owner.Cookie, "Alpha", "#ff0000")

	// Upload an asset and assign it to the project
	assetID := env.UploadTestAsset(t, owner.Cookie)
	_, err := env.SqlDB.Exec(
		`UPDATE assets SET project_id = ? WHERE id = ?`,
		p.ID, assetID,
	)
	if err != nil {
		t.Fatalf("assign project: %v", err)
	}

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var items []api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&items)

	if len(items) != 1 {
		t.Fatalf("expected 1 project, got %d", len(items))
	}
	if items[0].AssetCount != 1 {
		t.Errorf("asset_count = %d, want 1", items[0].AssetCount)
	}
}

func TestGetProject_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	p := th.CreateProject(t, env, owner.Cookie, "MyProject", "#00ff00")

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects/"+p.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != p.ID {
		t.Errorf("id mismatch: got %s, want %s", got.ID, p.ID)
	}
}

func TestGetProject_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/projects/nonexistent", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUpdateProject(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	p := th.CreateProject(t, env, owner.Cookie, "Old Name", "#aabbcc")

	newName := "New Name"
	newColor := "#112233"
	req := th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Name: &newName, Color: &newColor}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Name != "New Name" {
		t.Errorf("name = %q, want New Name", got.Name)
	}
	if got.Color == nil || *got.Color != "#112233" {
		t.Errorf("color = %v, want #112233", got.Color)
	}
}

func TestUpdateProject_PartialName(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	color := "#aabbcc"
	description := "original desc"
	p := th.CreateProject(t, env, owner.Cookie, "Original Name", color)
	// Set description via full update first
	req := th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Name: &p.Name, Color: &color, Description: &description}), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup update: expected 200, got %d", resp.StatusCode)
	}

	// Partial update: only name, color and description should be preserved
	newName := "Updated Name"
	req = th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Name: &newName}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Name != "Updated Name" {
		t.Errorf("name = %q, want Updated Name", got.Name)
	}
	if got.Color == nil || *got.Color != color {
		t.Errorf("color = %v, want %s (should be preserved)", got.Color, color)
	}
	if got.Description == nil || *got.Description != description {
		t.Errorf("description = %v, want %q (should be preserved)", got.Description, description)
	}
}

func TestUpdateProject_PartialColor(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	p := th.CreateProject(t, env, owner.Cookie, "My Project", "#aabbcc")

	newColor := "#112233"
	req := th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Color: &newColor}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Color == nil || *got.Color != "#112233" {
		t.Errorf("color = %v, want #112233", got.Color)
	}
	if got.Name != "My Project" {
		t.Errorf("name = %q, want My Project (should be preserved)", got.Name)
	}
}

func TestUpdateProject_PartialDescription_PreservesCoverAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	p := th.CreateProject(t, env, owner.Cookie, "My Project", "#aabbcc")
	assetID := env.UploadTestAsset(t, owner.Cookie)

	// Set cover_asset_id via full update
	desc := "original"
	req := th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Name: &p.Name, CoverAssetID: &assetID, Description: &desc}), owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("setup update: expected 200, got %d", resp.StatusCode)
	}

	// Partial update: only description, cover_asset_id should be preserved
	newDesc := "updated desc"
	req = th.AuthRequest(http.MethodPut, "/api/v1/projects/"+p.ID,
		th.JsonBody(api.UpdateProjectRequest{Description: &newDesc}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got api.ProjectResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.Description == nil || *got.Description != "updated desc" {
		t.Errorf("description = %v, want updated desc", got.Description)
	}
	if got.CoverAssetID == nil || *got.CoverAssetID != assetID {
		t.Errorf("cover_asset_id = %v, want %s (should be preserved)", got.CoverAssetID, assetID)
	}
}

func TestDeleteProject_UnlinksAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	p := th.CreateProject(t, env, owner.Cookie, "Temp", "#000000")

	assetID := env.UploadTestAsset(t, owner.Cookie)
	env.SqlDB.Exec(`UPDATE assets SET project_id = ? WHERE id = ?`, p.ID, assetID) //nolint:errcheck

	// Delete the project
	req := th.AuthRequest(http.MethodDelete, "/api/v1/projects/"+p.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify asset still exists but project_id is NULL
	var projectID *string
	row := env.SqlDB.QueryRow(`SELECT project_id FROM assets WHERE id = ?`, assetID)
	if err := row.Scan(&projectID); err != nil {
		t.Fatalf("scan: %v", err)
	}
	if projectID != nil {
		t.Errorf("expected project_id to be NULL after deletion, got %v", *projectID)
	}
}
