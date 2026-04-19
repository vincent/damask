package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	th "damask/server/internal/tests_helpers"

	"github.com/gofiber/fiber/v3"
)

func TestCollections_Create(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "My Collection"}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var col map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if col["name"] != "My Collection" {
		t.Errorf("expected name My Collection, got %v", col["name"])
	}
}

func TestCollections_CreateWithAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Stack Save", "asset_ids": []string{asset.ID}}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var col map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if col["asset_count"].(float64) != 1 {
		t.Errorf("expected asset_count=1, got %v", col["asset_count"])
	}
}

func TestCollections_CreateValidation(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": ""}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCollections_List(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Create two collections.
	for _, name := range []string{"Alpha", "Beta"} {
		if _, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
			th.JsonBody(map[string]any{"name": name}), owner.Cookie)); err != nil {
			t.Fatal(err)
		}
	}

	resp, err := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/collections", nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var cols []map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&cols); err != nil {
		t.Fatal(err)
	}
	if len(cols) != 2 {
		t.Errorf("expected 2 collections, got %d", len(cols))
	}
}

func TestCollections_Get(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Create collection with one asset.
	createResp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Detail Test", "asset_ids": []string{asset.ID}}), owner.Cookie))
	var col map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if err := createResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	colID := col["id"].(string)

	resp, err := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/collections/"+colID, nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var detail map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&detail); err != nil {
		t.Fatal(err)
	}
	assets := detail["assets"].([]any)
	if len(assets) != 1 {
		t.Errorf("expected 1 asset, got %d", len(assets))
	}
}

func TestCollections_GetNotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	resp, err := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/collections/nonexistent", nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCollections_Update(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	createResp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Before"}), owner.Cookie))
	var col map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if err := createResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	colID := col["id"].(string)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPut, "/api/v1/collections/"+colID,
		th.JsonBody(map[string]any{"name": "After", "description": "desc"}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}
	var updated map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&updated); err != nil {
		t.Fatal(err)
	}
	if updated["name"] != "After" {
		t.Errorf("expected name After, got %v", updated["name"])
	}
}

func TestCollections_Delete(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	createResp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "To Delete"}), owner.Cookie))
	var col map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if err := createResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	colID := col["id"].(string)

	resp, err := env.App.Test(th.AuthRequest(http.MethodDelete, "/api/v1/collections/"+colID, nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204, got %d", resp.StatusCode)
	}

	getResp, _ := env.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/collections/"+colID, nil, owner.Cookie))
	defer getResp.Body.Close()
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestCollections_AddRemoveAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	createResp, _ := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Asset Test"}), owner.Cookie))
	var col map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if err := createResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	colID := col["id"].(string)

	// Add asset.
	addResp, err := env.App.Test(th.AuthRequest(http.MethodPost,
		"/api/v1/collections/"+colID+"/assets/"+asset.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	if err := addResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if addResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 on add, got %d", addResp.StatusCode)
	}

	// Remove asset.
	rmResp, err := env.App.Test(th.AuthRequest(http.MethodDelete,
		"/api/v1/collections/"+colID+"/assets/"+asset.ID, nil, owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	if err := rmResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	if rmResp.StatusCode != http.StatusNoContent {
		t.Errorf("expected 204 on remove, got %d", rmResp.StatusCode)
	}
}

func TestCollections_CreateWithForeignAsset(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	foreignAsset := th.UploadAsset(t, env2, owner2.Cookie)

	resp, err := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Bad", "asset_ids": []string{foreignAsset.ID}}), owner1.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for foreign asset, got %d: %s", resp.StatusCode, body)
	}
}

func TestCollections_CreateMixedOwnership(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	localAsset := th.UploadAsset(t, env1, owner1.Cookie)
	foreignAsset := th.UploadAsset(t, env2, owner2.Cookie)

	resp, err := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Mixed", "asset_ids": []string{localAsset.ID, foreignAsset.ID}}), owner1.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		body, _ := io.ReadAll(resp.Body)
		t.Errorf("expected 403 for mixed ownership, got %d: %s", resp.StatusCode, body)
	}
}

func TestCollections_WorkspaceIsolation(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	// Create collection in env1.
	createResp, _ := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/collections",
		th.JsonBody(map[string]any{"name": "Private"}), owner1.Cookie))
	var col map[string]any
	if err := json.NewDecoder(createResp.Body).Decode(&col); err != nil {
		t.Fatal(err)
	}
	if err := createResp.Body.Close(); err != nil {
		t.Fatal(err)
	}
	colID := col["id"].(string)

	// Try to read from env2.
	resp, err := env2.App.Test(th.AuthRequest(http.MethodGet, "/api/v1/collections/"+colID, nil, owner2.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for cross-workspace access, got %d", resp.StatusCode)
	}
}

func TestCollections_Unauthenticated(t *testing.T) {
	env, _ := th.SetupWithOwner(t)

	req, _ := http.NewRequest(http.MethodGet, "/api/v1/collections", nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
