package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"image"
	"image/color"
	"image/png"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
)

// createTestAsset uploads a small PNG and returns its ID + auth cookie.
func createTestAsset(t *testing.T, env *testEnv) (assetID string, cookie *http.Cookie) {
	t.Helper()
	res := register(t, env, "User", "user@test.com", "password123")

	// Build a minimal PNG in memory.
	img := image.NewRGBA(image.Rect(0, 0, 10, 10))
	img.Set(5, 5, color.RGBA{R: 255, G: 0, B: 0, A: 255})
	var buf bytes.Buffer
	if err := png.Encode(&buf, img); err != nil {
		t.Fatalf("encode test png: %v", err)
	}

	var body bytes.Buffer
	boundary := "TestBoundary"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"test.png\"\r\nContent-Type: image/png\r\n\r\n", boundary)
	_, _ = body.Write(buf.Bytes())
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req.AddCookie(res.Cookie)

	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatalf("upload asset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("upload asset: expected 201, got %d", resp.StatusCode)
	}

	var a assetResponse
	if err := json.NewDecoder(resp.Body).Decode(&a); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	return a.ID, res.Cookie
}

// insertVariantDirectly inserts a variant row into the DB bypassing the queue.
func insertVariantDirectly(t *testing.T, env *testEnv, assetID, workspaceID string) dbgen.Variant {
	t.Helper()
	variantID := "test-variant-id"

	// Store a dummy file so file download works.
	_ = env.storage.Put(
		fmt.Sprintf("%s/%s/variants/%s.jpg", workspaceID, assetID, variantID),
		bytes.NewReader([]byte("dummy variant content")),
	)

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	queries, _, err := openTestDB(t)
	if err != nil {
		t.Fatalf("open queries: %v", err)
	}
	_ = queries // avoid unused; we use env.sqlDB directly

	_, err = env.sqlDB.ExecContext(ctx, `
		INSERT INTO variants (id, asset_id, workspace_id, type, storage_key, transform_params, size)
		VALUES (?, ?, ?, 'resize', ?, '{"width":100}', 1024)
	`, variantID, assetID, workspaceID,
		fmt.Sprintf("%s/%s/variants/%s.jpg", workspaceID, assetID, variantID),
	)
	if err != nil {
		t.Fatalf("insert variant: %v", err)
	}

	row := env.sqlDB.QueryRowContext(ctx, `SELECT id, asset_id, workspace_id, type, storage_key, transform_params, size, created_at FROM variants WHERE id = ?`, variantID)
	var v dbgen.Variant
	if err := row.Scan(&v.ID, &v.AssetID, &v.WorkspaceID, &v.Type, &v.StorageKey, &v.TransformParams, &v.Size, &v.CreatedAt); err != nil {
		t.Fatalf("scan variant: %v", err)
	}
	return v
}

// openTestDB is a test helper that just re-uses the env's sqlDB — here unused
// but kept for clarity.
func openTestDB(t *testing.T) (interface{}, interface{}, error) {
	return nil, nil, nil
}

// ---- Tests ----

func TestListVariants_Empty(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	var variants []variantResponse
	if err := json.NewDecoder(resp.Body).Decode(&variants); err != nil {
		t.Fatal(err)
	}
	if len(variants) != 0 {
		t.Fatalf("expected empty list, got %d", len(variants))
	}
}

func TestListVariants_NotFound(t *testing.T) {
	env := setupTestApp(t)
	res := register(t, env, "U", "u@test.com", "pass1234")

	req := authRequest(http.MethodGet, "/api/v1/assets/nonexistent/variants", nil, res.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListVariants_WithVariant(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	// Get the workspace ID from the asset.
	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.app.Test(req)
	var a assetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	req2 := authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	resp2, err := env.app.Test(req2)
	if err != nil {
		t.Fatal(err)
	}
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp2.StatusCode)
	}

	var variants []variantResponse
	_ = json.NewDecoder(resp2.Body).Decode(&variants)
	if len(variants) != 1 {
		t.Fatalf("expected 1 variant, got %d", len(variants))
	}
	if variants[0].Type != "resize" {
		t.Errorf("expected type resize, got %s", variants[0].Type)
	}
	if variants[0].DownloadURL == "" {
		t.Error("expected non-empty download URL")
	}
}

func TestCreateVariant_InvalidType(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		jsonStr(`{"type":"invalid_type","params":{}}`), cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_VideoOnImage(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		jsonStr(`{"type":"video_thumbnail","params":{}}`), cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d", resp.StatusCode)
	}
}

func TestCreateVariant_WatermarkQueued(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	body := `{"type":"image_watermark","params":{"opacity":50,"quality":80,"format":"jpeg"}}`
	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants", jsonStr(body), cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["job_id"] == "" {
		t.Error("expected job_id in response")
	}
	if result["status"] != "pending" {
		t.Errorf("expected status=pending, got %v", result["status"])
	}
}

func TestCreateVariant_ResizeQueued(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	body := `{"type":"image_resize","params":{"width":200,"height":200,"fit":"contain","quality":80,"format":"jpeg"}}`
	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants", jsonStr(body), cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusAccepted {
		t.Fatalf("expected 202, got %d", resp.StatusCode)
	}

	var result map[string]interface{}
	_ = json.NewDecoder(resp.Body).Decode(&result)
	if result["job_id"] == "" {
		t.Error("expected job_id in response")
	}
	if result["status"] != "pending" {
		t.Errorf("expected status=pending, got %v", result["status"])
	}
}

func TestCreateVariant_BgRemoveNoKey(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodPost, "/api/v1/assets/"+assetID+"/variants",
		jsonStr(`{"type":"image_bg_remove","params":{}}`), cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 (no API key), got %d", resp.StatusCode)
	}
}

func TestDeleteVariant(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.app.Test(req)
	var a assetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	delReq := authRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, v.ID), nil, cookie)
	delResp, err := env.app.Test(delReq)
	if err != nil {
		t.Fatal(err)
	}
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	// Verify deleted
	listReq := authRequest(http.MethodGet, "/api/v1/assets/"+assetID+"/variants", nil, cookie)
	listResp, _ := env.app.Test(listReq)
	var variants []variantResponse
	_ = json.NewDecoder(listResp.Body).Decode(&variants)
	if len(variants) != 0 {
		t.Fatalf("expected 0 variants after delete, got %d", len(variants))
	}
}

func TestDeleteVariant_NotFound(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/nonexistent", assetID), nil, cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Fatalf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetVariantFile(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, cookie)
	resp, _ := env.app.Test(req)
	var a assetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	fileReq := authRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s/file", assetID, v.ID), nil, cookie)
	fileResp, err := env.app.Test(fileReq)
	if err != nil {
		t.Fatal(err)
	}
	if fileResp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", fileResp.StatusCode)
	}
}

func TestPreviewTransform(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	req := authRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/preview?w=50&h=50&fit=contain&format=jpeg&q=80", assetID),
		nil, cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 10000})
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	if ct := resp.Header.Get("Content-Type"); ct != "image/jpeg" {
		t.Errorf("expected image/jpeg, got %s", ct)
	}
}

func TestPreviewTransform_Cached(t *testing.T) {
	env := setupTestApp(t)
	assetID, cookie := createTestAsset(t, env)

	url := fmt.Sprintf("/api/v1/assets/%s/preview?w=50&h=50", assetID)
	for i := 0; i < 3; i++ {
		req := authRequest(http.MethodGet, url, nil, cookie)
		resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 10000})
		if err != nil {
			t.Fatal(err)
		}
		if resp.StatusCode != http.StatusOK {
			t.Fatalf("iteration %d: expected 200, got %d", i, resp.StatusCode)
		}
	}
}

func TestPreviewTransform_NonImage(t *testing.T) {
	env := setupTestApp(t)
	res := register(t, env, "U", "u2@test.com", "pass1234")

	// Upload a non-image file.
	var body bytes.Buffer
	boundary := "TestBoundaryPDF"
	_, _ = fmt.Fprintf(&body, "--%s\r\nContent-Disposition: form-data; name=\"file\"; filename=\"doc.pdf\"\r\nContent-Type: application/pdf\r\n\r\n", boundary)
	_, _ = body.WriteString("%PDF-1.4 fake content")
	_, _ = fmt.Fprintf(&body, "\r\n--%s--\r\n", boundary)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/assets", &body)
	req.Header.Set("Content-Type", fmt.Sprintf("multipart/form-data; boundary=%s", boundary))
	req.AddCookie(res.Cookie)
	resp, _ := env.app.Test(req, fiber.TestConfig{Timeout: 10000})

	var a assetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	previewReq := authRequest(http.MethodGet,
		fmt.Sprintf("/api/v1/assets/%s/preview?w=100", a.ID), nil, res.Cookie)
	previewResp, _ := env.app.Test(previewReq, fiber.TestConfig{Timeout: 5000})
	if previewResp.StatusCode != http.StatusBadRequest {
		t.Fatalf("expected 400 for non-image preview, got %d", previewResp.StatusCode)
	}
}

func TestVariant_ViewerCannotDelete(t *testing.T) {
	env := setupTestApp(t)
	assetID, ownerCookie := createTestAsset(t, env)

	req := authRequest(http.MethodGet, "/api/v1/assets/"+assetID, nil, ownerCookie)
	resp, _ := env.app.Test(req)
	var a assetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)

	v := insertVariantDirectly(t, env, assetID, a.WorkspaceID)

	viewerToken := mintEditorToken(t, env, a.WorkspaceID, "viewer")
	delReq := bearerRequest(http.MethodDelete,
		fmt.Sprintf("/api/v1/assets/%s/variants/%s", assetID, v.ID), nil, viewerToken)
	delResp, _ := env.app.Test(delReq)
	if delResp.StatusCode != http.StatusForbidden {
		t.Fatalf("expected 403 for viewer delete, got %d", delResp.StatusCode)
	}
}
