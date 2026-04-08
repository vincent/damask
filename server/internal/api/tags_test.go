package api_test

import (
	"damask/server/internal/api"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// addTag is a test helper that adds a tag to an asset.
func addTag(t *testing.T, env *th.TestEnv, cookie *http.Cookie, assetID, name string) int {
	t.Helper()
	body := fmt.Sprintf(`{"name":%q}`, name)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/tags", strings.NewReader(body), cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("add tag: %v", err)
	}
	return resp.StatusCode
}

func TestAddTag_AutoCreates(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	code := addTag(t, env, owner.Cookie, assetID, "summer")
	if code != http.StatusCreated {
		t.Fatalf("expected 201, got %d", code)
	}

	// Tag should appear in /api/v1/tags
	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)
	if len(tags) != 1 || tags[0].Name != "summer" {
		t.Errorf("expected tag 'summer', got %+v", tags)
	}
}

func TestAddTag_Idempotent(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "beach")
	addTag(t, env, owner.Cookie, assetID, "beach") // duplicate — should not error

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var names []string
	_ = json.NewDecoder(resp.Body).Decode(&names)
	if len(names) != 1 {
		t.Errorf("expected 1 tag after duplicate add, got %d", len(names))
	}
}

func TestRemoveTag_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "travel")

	req := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+assetID+"/tags/travel", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Confirm removed
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp2, _ := env.App.Test(req2)
	var names []string
	_ = json.NewDecoder(resp2.Body).Decode(&names)
	if len(names) != 0 {
		t.Errorf("expected 0 tags after removal, got %d", len(names))
	}
}

func TestListTags_WithCounts(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "nature")
	addTag(t, env, owner.Cookie, a2, "nature")
	addTag(t, env, owner.Cookie, a1, "sunset")

	req := th.AuthRequest(http.MethodGet, "/api/v1/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var tags []api.TagResponse
	_ = json.NewDecoder(resp.Body).Decode(&tags)

	counts := map[string]int64{}
	for _, tag := range tags {
		counts[tag.Name] = tag.AssetCount
	}
	if counts["nature"] != 2 {
		t.Errorf("nature count = %d, want 2", counts["nature"])
	}
	if counts["sunset"] != 1 {
		t.Errorf("sunset count = %d, want 1", counts["sunset"])
	}
}

func TestGetAssetTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "alpha")
	addTag(t, env, owner.Cookie, assetID, "beta")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/tags", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var names []string
	_ = json.NewDecoder(resp.Body).Decode(&names)

	if len(names) != 2 {
		t.Errorf("expected 2 tags, got %d", len(names))
	}
}

func TestGetAsset_IncludesTags(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	assetID := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, assetID, "coast")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	if len(a.Tags) != 1 || a.Tags[0] != "coast" {
		t.Errorf("expected tags=[coast], got %v", a.Tags)
	}
}

func TestListAssets_FilterBySingleTag(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	addTag(t, env, owner.Cookie, a1, "forest")
	addTag(t, env, owner.Cookie, a2, "city")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?tags=forest", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset for tag=forest, got %d", len(result.Assets))
	}
	if result.Assets[0].ID != a1 {
		t.Errorf("wrong asset returned: got %s, want %s", result.Assets[0].ID, a1)
	}
}

func TestListAssets_FilterByMultiTagAND(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	// a1 has both tags
	addTag(t, env, owner.Cookie, a1, "red")
	addTag(t, env, owner.Cookie, a1, "blue")
	// a2 only has red
	addTag(t, env, owner.Cookie, a2, "red")

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?tags=red,blue", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)

	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 asset with both tags, got %d", len(result.Assets))
	}
	if result.Assets[0].ID != a1 {
		t.Errorf("wrong asset: got %s, want %s", result.Assets[0].ID, a1)
	}
}

func TestBulkTag(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q],"tag_name":"promo"}`, a1, a2)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/tag", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify both have the tag
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id+"/tags", nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		var names []string
		_ = json.NewDecoder(resp2.Body).Decode(&names)
		if len(names) != 1 || names[0] != "promo" {
			t.Errorf("asset %s: expected tag promo, got %v", id, names)
		}
	}
}

func TestBulkProject(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	p := createProject(t, env, owner.Cookie, "Batch", "#abcdef")

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q],"project_id":%q}`, a1, a2, p.ID)
	req := th.AuthRequest(http.MethodPost, "/api/v1/assets/bulk/project", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify assets are assigned
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id, nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		var a api.AssetResponse
		_ = json.NewDecoder(resp2.Body).Decode(&a)
		if a.ProjectID == nil || *a.ProjectID != p.ID {
			t.Errorf("asset %s: expected project_id=%s, got %v", id, p.ID, a.ProjectID)
		}
	}
}

func TestBulkDelete(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"asset_ids":[%q,%q]}`, a1, a2)
	req := th.AuthRequest(http.MethodDelete, "/api/v1/assets/bulk", strings.NewReader(body), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Both should be gone
	for _, id := range []string{a1, a2} {
		req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+id, nil, owner.Cookie)
		resp2, _ := env.App.Test(req2)
		if resp2.StatusCode != http.StatusNotFound {
			t.Errorf("asset %s: expected 404 after bulk delete, got %d", id, resp2.StatusCode)
		}
	}
}
