package api_test

import (
	"bytes"
	"damask/server/internal/api"
	th "damask/server/internal/tests_helpers"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

func TestUploadAsset_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	jpegData := th.MakeJPEG(200, 150)
	req := th.BuildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)

	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode response: %v", err)
	}
	if asset.ID == "" {
		t.Error("expected non-empty asset ID")
	}
	if asset.OriginalFilename != "photo.jpg" {
		t.Errorf("filename: got %q, want %q", asset.OriginalFilename, "photo.jpg")
	}
	if !strings.HasPrefix(asset.MimeType, "image/") {
		t.Errorf("mime_type: got %q, expected image prefix", asset.MimeType)
	}
	if asset.Size <= 0 {
		t.Error("expected positive size")
	}
	if asset.Width == nil || *asset.Width != 200 {
		t.Errorf("width: got %v, want 200", asset.Width)
	}
	if asset.Height == nil || *asset.Height != 150 {
		t.Errorf("height: got %v, want 150", asset.Height)
	}
}

func TestUploadAsset_InFolder(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Create a project
	projRes, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/projects",
		th.JsonBody(api.CreateProjectRequest{Name: "My Project"}), owner.Cookie))
	if err != nil {
		t.Fatalf("create project: %v", err)
	}
	defer projRes.Body.Close()
	if projRes.StatusCode != http.StatusCreated {
		t.Fatalf("create project: got %d", projRes.StatusCode)
	}
	var proj api.ProjectResponse
	if err := json.NewDecoder(projRes.Body).Decode(&proj); err != nil {
		t.Fatalf("decode project: %v", err)
	}

	// Create a folder inside the project
	folder := createFolder(t, env, owner.Cookie, proj.ID, "Photos", nil)

	// Upload an asset into the folder
	req := th.BuildUploadRequest(t, "photo.jpg", th.MakeJPEG(10, 10), owner.Cookie,
		map[string]string{"folder_id": folder.ID})
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	if asset.FolderID == nil || *asset.FolderID != folder.ID {
		t.Errorf("folder_id = %v, want %s", asset.FolderID, folder.ID)
	}
}

func TestUploadAsset_Unauthenticated(t *testing.T) {
	env := th.SetupTestApp(t)

	req := th.BuildUploadRequest(t, "file.jpg", th.MakeJPEG(10, 10), nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUploadAsset_ViewerForbidden(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, "viewer")

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("file", "file.jpg")
	fw.Write(th.MakeJPEG(10, 10)) //nolint:errcheck
	err := w.Close()
	if err != nil {
		t.Fatalf("close form: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+viewerToken)

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListAssets_Empty(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(result.Assets) != 0 {
		t.Errorf("expected empty list, got %d assets", len(result.Assets))
	}
	if result.NextCursor != nil {
		t.Error("expected nil next_cursor for empty list")
	}
}

func TestListAssets_Pagination(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload 3 assets with small delays to ensure distinct created_at
	for i := range 3 {
		jpegData := th.MakeJPEG(10, 10)
		req := th.BuildUploadRequest(t, "img"+string(rune('a'+i))+".jpg", jpegData, owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d: status %d err %v", i, resp.StatusCode, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Fetch with limit=2
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?limit=2", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}

	var page1 api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1.Assets) != 2 {
		t.Fatalf("expected 2 assets on page1, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("expected next_cursor on page1")
	}

	// Fetch page 2
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets?limit=2&cursor="+*page1.NextCursor, nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}

	var page2 api.AssetListResponse
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2.Assets) != 1 {
		t.Fatalf("expected 1 asset on page2, got %d", len(page2.Assets))
	}
	if page2.NextCursor != nil {
		t.Error("expected nil next_cursor on last page")
	}
}

func TestGetAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadReq := th.BuildUploadRequest(t, "test.jpg", th.MakeJPEG(50, 50), owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&got)
	if got.ID != uploaded.ID {
		t.Errorf("id mismatch: got %q want %q", got.ID, uploaded.ID)
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/nonexistent", nil, owner.Cookie)
	resp, _ := env.App.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetAssetFile(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	jpegData := th.MakeJPEG(20, 20)
	uploadReq := th.BuildUploadRequest(t, "file.jpg", jpegData, owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty file content")
	}
	ct := resp.Header.Get("Content-Type")
	if !strings.HasPrefix(ct, "image/") {
		t.Errorf("expected image content-type, got %q", ct)
	}
}

func TestGetAssetFile_ServesCurrentVersion(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload original asset (100×100).
	asset := th.UploadAsset(t, env, owner.Cookie)

	// Upload a second version with different dimensions (200×200).
	v2Data := th.MakeJPEG(200, 200)
	vReq := th.BuildVersionUploadRequest(t, asset.ID, "v2.jpg", v2Data, "", owner.Cookie)
	vResp, err := env.App.Test(vReq, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload v2: %v", err)
	}
	if vResp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(vResp.Body)
		t.Fatalf("expected 201 for v2 upload, got %d: %s", vResp.StatusCode, b)
	}

	// GET /api/v1/assets/:id/file must return v2 bytes (larger than v1).
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+asset.ID+"/file", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("get asset file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	fileBytes, _ := io.ReadAll(resp.Body)
	if len(fileBytes) == 0 {
		t.Fatal("expected non-empty file content")
	}
	// v2 (200×200) must be larger than v1 (100×100).
	v1Bytes := th.MakeJPEG(100, 100)
	if len(fileBytes) <= len(v1Bytes) {
		t.Errorf("expected v2 file (%d bytes) to be larger than v1 (%d bytes)", len(fileBytes), len(v1Bytes))
	}
}

func TestDeleteAsset(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	uploadReq := th.BuildUploadRequest(t, "del.jpg", th.MakeJPEG(10, 10), owner.Cookie)
	uploadResp, _ := env.App.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded api.AssetResponse
	_ = json.NewDecoder(uploadResp.Body).Decode(&uploaded)

	delReq := th.AuthRequest(http.MethodDelete, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.App.Test(delReq)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify gone from DB
	getReq := th.AuthRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	getResp, _ := env.App.Test(getReq)
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestListAssets_Sort(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload assets with distinct sizes: 10x10, 50x50, 100x100
	sizes := []int{10, 50, 100}
	for _, s := range sizes {
		req := th.BuildUploadRequest(t, "img.jpg", th.MakeJPEG(s, s), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %dx%d: status %d err %v", s, s, resp.StatusCode, err)
		}
		var a api.AssetResponse
		_ = json.NewDecoder(resp.Body).Decode(&a)
		time.Sleep(10 * time.Millisecond)
	}

	cases := []struct {
		sort  string
		check func(t *testing.T, assets []api.AssetResponse)
	}{
		{
			sort: "id_desc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].ID < assets[1].ID || assets[1].ID < assets[2].ID {
					t.Errorf("id_desc: IDs not descending: %v", []string{assets[0].ID, assets[1].ID, assets[2].ID})
				}
			},
		},
		{
			sort: "id_asc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].ID > assets[1].ID || assets[1].ID > assets[2].ID {
					t.Errorf("id_asc: IDs not ascending: %v", []string{assets[0].ID, assets[1].ID, assets[2].ID})
				}
			},
		},
		{
			sort: "size_asc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].Size > assets[1].Size || assets[1].Size > assets[2].Size {
					t.Errorf("size_asc: sizes not ascending: %v", []int64{assets[0].Size, assets[1].Size, assets[2].Size})
				}
			},
		},
		{
			sort: "size_desc",
			check: func(t *testing.T, assets []api.AssetResponse) {
				if assets[0].Size < assets[1].Size || assets[1].Size < assets[2].Size {
					t.Errorf("size_desc: sizes not descending: %v", []int64{assets[0].Size, assets[1].Size, assets[2].Size})
				}
			},
		},
	}

	for _, tc := range cases {
		t.Run(tc.sort, func(t *testing.T) {
			req := th.AuthRequest(http.MethodGet, "/api/v1/assets?sort="+tc.sort, nil, owner.Cookie)
			resp, err := env.App.Test(req)
			if err != nil {
				t.Fatalf("list: %v", err)
			}
			if resp.StatusCode != http.StatusOK {
				t.Fatalf("expected 200, got %d", resp.StatusCode)
			}
			var result api.AssetListResponse
			_ = json.NewDecoder(resp.Body).Decode(&result)
			if len(result.Assets) != 3 {
				t.Fatalf("expected 3 assets, got %d", len(result.Assets))
			}
			tc.check(t, result.Assets)
		})
	}
}

func TestSearchAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload two assets with distinct names
	for _, name := range []string{"sunset_beach.jpg", "mountain_peak.jpg"} {
		req := th.BuildUploadRequest(t, name, th.MakeJPEG(10, 10), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %s: %v %d", name, err, resp.StatusCode)
		}
	}

	// Search for "sunset"
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=sunset", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result api.AssetListResponse
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(result.Assets))
	}
	if result.Assets[0].OriginalFilename != "sunset_beach.jpg" {
		t.Errorf("expected sunset_beach.jpg, got %q", result.Assets[0].OriginalFilename)
	}
}

func TestSearchAssets_Pagination(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload 3 assets whose names all match "photo"
	for i := range 3 {
		name := "photo_" + string(rune('a'+i)) + ".jpg"
		req := th.BuildUploadRequest(t, name, th.MakeJPEG(10, 10), owner.Cookie)
		resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %s: %v %d", name, err, resp.StatusCode)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Page 1: limit=2, expect next_cursor
	req := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=photo&limit=2", nil, owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("search page1: %v", err)
	}
	var page1 api.AssetListResponse
	if err := json.NewDecoder(resp.Body).Decode(&page1); err != nil {
		t.Fatalf("decode page1: %v", err)
	}
	if len(page1.Assets) != 2 {
		t.Fatalf("expected 2 assets on page1, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("expected next_cursor on page1")
	}

	// Page 2: use cursor, expect 1 remaining asset and no next_cursor
	req2 := th.AuthRequest(http.MethodGet, "/api/v1/assets?q=photo&limit=2&cursor="+*page1.NextCursor, nil, owner.Cookie)
	resp2, err := env.App.Test(req2)
	if err != nil {
		t.Fatalf("search page2: %v", err)
	}
	var page2 api.AssetListResponse
	if err := json.NewDecoder(resp2.Body).Decode(&page2); err != nil {
		t.Fatalf("decode page2: %v", err)
	}
	if len(page2.Assets) != 1 {
		t.Fatalf("expected 1 asset on page2, got %d", len(page2.Assets))
	}
	if page2.NextCursor != nil {
		t.Error("expected nil next_cursor on last page")
	}

	// Ensure no asset ID appears in both pages
	seen := make(map[string]bool)
	for _, a := range page1.Assets {
		seen[a.ID] = true
	}
	for _, a := range page2.Assets {
		if seen[a.ID] {
			t.Errorf("duplicate asset %s returned on both pages", a.ID)
		}
	}
}

// insertAssetWithSize inserts a minimal asset row directly via SQL with a known size value.
func insertAssetWithSize(t *testing.T, env *th.TestEnv, workspaceID string, size int64) string {
	t.Helper()
	id := fmt.Sprintf("asset-%d", size)
	_, err := env.SqlDB.Exec(`
		INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size, created_at, updated_at)
		VALUES (?, ?, ?, ?, ?, ?, datetime('now'), datetime('now'))
	`, id, workspaceID, fmt.Sprintf("file-%d.bin", size), fmt.Sprintf("key-%d", size), "application/octet-stream", size)
	if err != nil {
		t.Fatalf("insert asset with size %d: %v", size, err)
	}
	return id
}

func TestListAssets_PaginationSortBySizeDesc(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Insert 25 assets with distinct sizes 1..25 bytes
	var inserted []string
	for i := int64(1); i <= 25; i++ {
		inserted = append(inserted, insertAssetWithSize(t, env, owner.WorkspaceID, i))
	}

	getPage := func(cursor string) api.AssetListResponse {
		url := "/api/v1/assets?sort=size_desc&limit=10"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp, err := env.App.Test(th.AuthRequest(http.MethodGet, url, nil, owner.Cookie), fiber.TestConfig{Timeout: 5000})
		if err != nil {
			t.Fatalf("list assets: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var result api.AssetListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return result
	}

	page1 := getPage("")
	if len(page1.Assets) != 10 {
		t.Fatalf("page1: expected 10, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("page1: expected next_cursor")
	}

	page2 := getPage(*page1.NextCursor)
	if len(page2.Assets) != 10 {
		t.Fatalf("page2: expected 10, got %d", len(page2.Assets))
	}
	if page2.NextCursor == nil {
		t.Fatal("page2: expected next_cursor")
	}

	page3 := getPage(*page2.NextCursor)
	if len(page3.Assets) != 5 {
		t.Fatalf("page3: expected 5, got %d", len(page3.Assets))
	}
	if page3.NextCursor != nil {
		t.Error("page3: expected nil next_cursor")
	}

	// Collect all IDs and check for duplicates
	seen := make(map[string]bool)
	allAssets := append(append(page1.Assets, page2.Assets...), page3.Assets...)
	if len(allAssets) != 25 {
		t.Fatalf("expected 25 total assets, got %d", len(allAssets))
	}
	for _, a := range allAssets {
		if seen[a.ID] {
			t.Errorf("duplicate asset ID: %s", a.ID)
		}
		seen[a.ID] = true
	}

	// Verify non-increasing size order across all pages
	for i := 1; i < len(allAssets); i++ {
		if allAssets[i].Size > allAssets[i-1].Size {
			t.Errorf("size not non-increasing at position %d: %d > %d", i, allAssets[i].Size, allAssets[i-1].Size)
		}
	}

	// Verify all 25 inserted assets are present
	for _, id := range inserted {
		if !seen[id] {
			t.Errorf("inserted asset %s missing from results", id)
		}
	}
}

func TestListAssets_PaginationSortBySizeAsc(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	var inserted []string
	for i := int64(1); i <= 25; i++ {
		inserted = append(inserted, insertAssetWithSize(t, env, owner.WorkspaceID, i))
	}

	getPage := func(cursor string) api.AssetListResponse {
		url := "/api/v1/assets?sort=size_asc&limit=10"
		if cursor != "" {
			url += "&cursor=" + cursor
		}
		resp, err := env.App.Test(th.AuthRequest(http.MethodGet, url, nil, owner.Cookie), fiber.TestConfig{Timeout: 5000})
		if err != nil {
			t.Fatalf("list assets: %v", err)
		}
		if resp.StatusCode != http.StatusOK {
			body, _ := io.ReadAll(resp.Body)
			t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
		}
		var result api.AssetListResponse
		if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
			t.Fatalf("decode: %v", err)
		}
		return result
	}

	page1 := getPage("")
	if len(page1.Assets) != 10 {
		t.Fatalf("page1: expected 10, got %d", len(page1.Assets))
	}
	if page1.NextCursor == nil {
		t.Fatal("page1: expected next_cursor")
	}

	page2 := getPage(*page1.NextCursor)
	if len(page2.Assets) != 10 {
		t.Fatalf("page2: expected 10, got %d", len(page2.Assets))
	}
	if page2.NextCursor == nil {
		t.Fatal("page2: expected next_cursor")
	}

	page3 := getPage(*page2.NextCursor)
	if len(page3.Assets) != 5 {
		t.Fatalf("page3: expected 5, got %d", len(page3.Assets))
	}
	if page3.NextCursor != nil {
		t.Error("page3: expected nil next_cursor")
	}

	seen := make(map[string]bool)
	allAssets := append(append(page1.Assets, page2.Assets...), page3.Assets...)
	if len(allAssets) != 25 {
		t.Fatalf("expected 25 total assets, got %d", len(allAssets))
	}
	for _, a := range allAssets {
		if seen[a.ID] {
			t.Errorf("duplicate asset ID: %s", a.ID)
		}
		seen[a.ID] = true
	}

	// Verify non-decreasing size order across all pages
	for i := 1; i < len(allAssets); i++ {
		if allAssets[i].Size < allAssets[i-1].Size {
			t.Errorf("size not non-decreasing at position %d: %d < %d", i, allAssets[i].Size, allAssets[i-1].Size)
		}
	}

	for _, id := range inserted {
		if !seen[id] {
			t.Errorf("inserted asset %s missing from results", id)
		}
	}
}
