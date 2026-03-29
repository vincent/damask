package api

import (
	"bytes"
	"encoding/json"
	"image"
	"image/color"
	"image/jpeg"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// makeJPEG creates a minimal valid JPEG in memory.
func makeJPEG(width, height int) []byte {
	img := image.NewRGBA(image.Rect(0, 0, width, height))
	for y := range height {
		for x := range width {
			img.Set(x, y, color.RGBA{R: 100, G: 150, B: 200, A: 255})
		}
	}
	var buf bytes.Buffer
	_ = jpeg.Encode(&buf, img, nil)
	return buf.Bytes()
}

// buildUploadRequest creates a multipart/form-data request with a file field.
func buildUploadRequest(t *testing.T, filename string, content []byte, cookie *http.Cookie) *http.Request {
	t.Helper()
	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, err := w.CreateFormFile("file", filename)
	if err != nil {
		t.Fatalf("create form file: %v", err)
	}
	if _, err := fw.Write(content); err != nil {
		t.Fatalf("write form file: %v", err)
	}
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

func TestUploadAsset_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	jpegData := makeJPEG(200, 150)
	req := buildUploadRequest(t, "photo.jpg", jpegData, owner.Cookie)

	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, body)
	}

	var asset assetResponse
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
	if !asset.Width.Valid || asset.Width.Int64 != 200 {
		t.Errorf("width: got %v, want 200", asset.Width)
	}
	if !asset.Height.Valid || asset.Height.Int64 != 150 {
		t.Errorf("height: got %v, want 150", asset.Height)
	}
}

func TestUploadAsset_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)

	req := buildUploadRequest(t, "file.jpg", makeJPEG(10, 10), nil)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUploadAsset_ViewerForbidden(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	viewerToken := mintEditorToken(t, env, owner.WorkspaceID, "viewer")

	var body bytes.Buffer
	w := multipart.NewWriter(&body)
	fw, _ := w.CreateFormFile("file", "file.jpg")
	fw.Write(makeJPEG(10, 10)) //nolint:errcheck
	w.Close()

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	req.Header.Set("Authorization", "Bearer "+viewerToken)

	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestListAssets_Empty(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/assets", nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var result assetListResponse
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
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	// Upload 3 assets with small delays to ensure distinct created_at
	for i := range 3 {
		jpegData := makeJPEG(10, 10)
		req := buildUploadRequest(t, "img"+string(rune('a'+i))+".jpg", jpegData, owner.Cookie)
		resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %d: status %d err %v", i, resp.StatusCode, err)
		}
		time.Sleep(10 * time.Millisecond)
	}

	// Fetch with limit=2
	req := authRequest(http.MethodGet, "/api/v1/assets?limit=2", nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("list page1: %v", err)
	}

	var page1 assetListResponse
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
	req2 := authRequest(http.MethodGet, "/api/v1/assets?limit=2&cursor="+*page1.NextCursor, nil, owner.Cookie)
	resp2, err := env.app.Test(req2)
	if err != nil {
		t.Fatalf("list page2: %v", err)
	}

	var page2 assetListResponse
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
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	uploadReq := buildUploadRequest(t, "test.jpg", makeJPEG(50, 50), owner.Cookie)
	uploadResp, _ := env.app.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded assetResponse
	json.NewDecoder(uploadResp.Body).Decode(&uploaded) //nolint:errcheck

	req := authRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("get asset: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var got assetResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.ID != uploaded.ID {
		t.Errorf("id mismatch: got %q want %q", got.ID, uploaded.ID)
	}
}

func TestGetAsset_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/assets/nonexistent", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetAssetFile(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	jpegData := makeJPEG(20, 20)
	uploadReq := buildUploadRequest(t, "file.jpg", jpegData, owner.Cookie)
	uploadResp, _ := env.app.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded assetResponse
	json.NewDecoder(uploadResp.Body).Decode(&uploaded) //nolint:errcheck

	req := authRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID+"/file", nil, owner.Cookie)
	resp, err := env.app.Test(req)
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

func TestDeleteAsset(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	uploadReq := buildUploadRequest(t, "del.jpg", makeJPEG(10, 10), owner.Cookie)
	uploadResp, _ := env.app.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	var uploaded assetResponse
	json.NewDecoder(uploadResp.Body).Decode(&uploaded) //nolint:errcheck

	delReq := authRequest(http.MethodDelete, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	resp, err := env.app.Test(delReq)
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Verify gone from DB
	getReq := authRequest(http.MethodGet, "/api/v1/assets/"+uploaded.ID, nil, owner.Cookie)
	getResp, _ := env.app.Test(getReq)
	if getResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 after delete, got %d", getResp.StatusCode)
	}
}

func TestSearchAssets(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	// Upload two assets with distinct names
	for _, name := range []string{"sunset_beach.jpg", "mountain_peak.jpg"} {
		req := buildUploadRequest(t, name, makeJPEG(10, 10), owner.Cookie)
		resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
		if err != nil || resp.StatusCode != http.StatusCreated {
			t.Fatalf("upload %s: %v %d", name, err, resp.StatusCode)
		}
	}

	// Search for "sunset"
	req := authRequest(http.MethodGet, "/api/v1/assets?q=sunset", nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("search: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var result assetListResponse
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck
	if len(result.Assets) != 1 {
		t.Fatalf("expected 1 search result, got %d", len(result.Assets))
	}
	if result.Assets[0].OriginalFilename != "sunset_beach.jpg" {
		t.Errorf("expected sunset_beach.jpg, got %q", result.Assets[0].OriginalFilename)
	}
}
