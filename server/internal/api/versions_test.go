package api

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/gofiber/fiber/v3"
)

// buildVersionUploadRequest creates a multipart upload request for POST /assets/:id/versions.
func buildVersionUploadRequest(t *testing.T, assetID string, filename string, content []byte, comment string, cookie *http.Cookie) *http.Request {
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
	if comment != "" {
		if err := w.WriteField("comment", comment); err != nil {
			t.Fatalf("write comment field: %v", err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatalf("close writer: %v", err)
	}

	req := httptest.NewRequest(http.MethodPost, fmt.Sprintf("/api/v1/assets/%s/versions", assetID), &body)
	req.Header.Set("Content-Type", w.FormDataContentType())
	if cookie != nil {
		req.AddCookie(cookie)
	}
	return req
}

// uploadAsset is a helper that uploads an asset and seeds the initial v1 version row.
// The migration 000009 runs for existing rows at migration time; new assets created in
// tests need their v1 seeded manually since AV-2.1 (refactoring POST /assets) is out
// of scope for this phase.
func uploadAsset(t *testing.T, env *testEnv, cookie *http.Cookie) assetResponse {
	t.Helper()
	req := buildUploadRequest(t, "original.jpg", makeJPEG(100, 100), cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload asset: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("upload asset: expected 201, got %d: %s", resp.StatusCode, b)
	}
	var asset assetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode asset: %v", err)
	}
	seedVersionV1(t, env, asset)
	return asset
}

// seedVersionV1 inserts a v1 asset_versions row for assets created via the old upload
// path (before AV-2.1 integration). It also sets assets.current_version_id.
func seedVersionV1(t *testing.T, env *testEnv, asset assetResponse) string {
	t.Helper()
	versionID := "ver_v1_" + asset.ID

	// Resolve the owner user ID from workspace membership.
	var createdBy string
	err := env.sqlDB.QueryRow(
		`SELECT user_id FROM workspace_members WHERE workspace_id = ? ORDER BY created_at LIMIT 1`,
		asset.WorkspaceID,
	).Scan(&createdBy)
	if err != nil {
		t.Fatalf("resolve owner for seed: %v", err)
	}

	// Look up the real storage_key from the assets table so the file endpoint works.
	var storageKey string
	if err := env.sqlDB.QueryRow(
		`SELECT storage_key FROM assets WHERE id = ?`, asset.ID,
	).Scan(&storageKey); err != nil {
		t.Fatalf("lookup storage key: %v", err)
	}

	_, err = env.sqlDB.Exec(`
		INSERT OR IGNORE INTO asset_versions (
			id, asset_id, workspace_id, version_num, storage_key, content_hash,
			mime_type, size, width, height, created_by, created_at, is_current
		) VALUES (?, ?, ?, 1, ?, ?, ?, ?, ?, ?, ?, datetime('now'), 1)
	`,
		versionID, asset.ID, asset.WorkspaceID,
		storageKey,
		"seed-hash-"+asset.ID,
		asset.MimeType, asset.Size,
		asset.Width, asset.Height,
		createdBy,
	)
	if err != nil {
		t.Fatalf("seed v1 version: %v", err)
	}

	_, err = env.sqlDB.Exec(
		`UPDATE assets SET current_version_id = ? WHERE id = ?`, versionID, asset.ID,
	)
	if err != nil {
		t.Fatalf("set current_version_id: %v", err)
	}
	return versionID
}

// --- AV-1.4: List versions ---

func TestListVersions_InitiallyOneVersion(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	asset := uploadAsset(t, env, owner.Cookie)

	req := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("list versions: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var versions []versionResponse
	if err := json.NewDecoder(resp.Body).Decode(&versions); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if len(versions) != 1 {
		t.Fatalf("expected 1 version after upload, got %d", len(versions))
	}
	if !versions[0].IsCurrent {
		t.Error("v1 should be current")
	}
	if versions[0].VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", versions[0].VersionNum)
	}
}

func TestListVersions_AssetNotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/assets/nonexistent/versions", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListVersions_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	req := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, nil)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- AV-1.3: Upload new version ---

func TestUploadVersion_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	asset := uploadAsset(t, env, owner.Cookie)
	time.Sleep(5 * time.Millisecond)

	req := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(200, 200), "second version", owner.Cookie)
	resp, err := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload version: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 201, got %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Version versionResponse `json:"version"`
		Asset   assetResponse   `json:"asset"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if result.Version.VersionNum != 2 {
		t.Errorf("expected version_num=2, got %d", result.Version.VersionNum)
	}
	if !result.Version.IsCurrent {
		t.Error("new version should be current")
	}
	if result.Version.Comment == nil || *result.Version.Comment != "second version" {
		t.Errorf("expected comment 'second version', got %v", result.Version.Comment)
	}
	if result.Asset.ID != asset.ID {
		t.Errorf("asset ID mismatch: got %q", result.Asset.ID)
	}

	// List should now show 2 versions.
	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck
	if len(versions) != 2 {
		t.Errorf("expected 2 versions, got %d", len(versions))
	}
	// Most recent first
	if versions[0].VersionNum != 2 {
		t.Errorf("expected v2 first, got v%d", versions[0].VersionNum)
	}
}

func TestUploadVersion_DuplicateCurrentRejected(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	// Re-upload the exact same bytes as the first version — the data migration
	// uses a placeholder hash, so we need to upload a real version first.
	jpegData := makeJPEG(50, 50)

	// First real upload as v2.
	r1 := buildVersionUploadRequest(t, asset.ID, "dup.jpg", jpegData, "", owner.Cookie)
	resp1, _ := env.app.Test(r1, fiber.TestConfig{Timeout: 5000})
	if resp1.StatusCode != http.StatusCreated {
		b, _ := io.ReadAll(resp1.Body)
		t.Skipf("v2 upload failed: %d %s", resp1.StatusCode, b)
	}

	// Upload the exact same bytes again — should 409.
	r2 := buildVersionUploadRequest(t, asset.ID, "dup.jpg", jpegData, "", owner.Cookie)
	resp2, _ := env.app.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusConflict {
		b, _ := io.ReadAll(resp2.Body)
		t.Errorf("expected 409 for duplicate current version, got %d: %s", resp2.StatusCode, b)
	}
}

func TestUploadVersion_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	req := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(10, 10), "", nil)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_ViewerForbidden(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)
	viewerToken := mintEditorToken(t, env, owner.WorkspaceID, "viewer")

	req := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(10, 10), "", nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_AssetNotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := buildVersionUploadRequest(t, "nonexistent", "v2.jpg", makeJPEG(10, 10), "", owner.Cookie)
	resp, _ := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestUploadVersion_CommentTooLong(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	longComment := string(make([]byte, 501))
	req := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(10, 10), longComment, owner.Cookie)
	resp, _ := env.app.Test(req, fiber.TestConfig{Timeout: 5000})
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for long comment, got %d", resp.StatusCode)
	}
}

// --- AV-1.5: Restore ---

func TestRestoreVersion_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	// Upload v2.
	r2 := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(200, 200), "", owner.Cookie)
	resp2, _ := env.app.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusCreated {
		t.Skip("v2 upload failed")
	}

	// Get v1's ID from the list.
	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck

	var v1ID string
	for _, v := range versions {
		if v.VersionNum == 1 {
			v1ID = v.ID
		}
	}
	if v1ID == "" {
		t.Fatal("could not find v1 ID")
	}

	// Restore v1.
	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, v1ID)
	restoreReq := authRequest(http.MethodPost, restoreURL, nil, owner.Cookie)
	resp, err := env.app.Test(restoreReq)
	if err != nil {
		t.Fatalf("restore: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}

	var result struct {
		Version versionResponse `json:"version"`
		Asset   assetResponse   `json:"asset"`
	}
	json.NewDecoder(resp.Body).Decode(&result) //nolint:errcheck

	if !result.Version.IsCurrent {
		t.Error("restored version should be current")
	}
	if result.Version.VersionNum != 1 {
		t.Errorf("expected version_num=1, got %d", result.Version.VersionNum)
	}
}

func TestRestoreVersion_AlreadyCurrent(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	// Get the single version ID.
	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, versions[0].ID)
	resp, _ := env.app.Test(authRequest(http.MethodPost, restoreURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusConflict {
		t.Errorf("expected 409 for already-current, got %d", resp.StatusCode)
	}
}

func TestRestoreVersion_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/nonexistent/restore", asset.ID)
	resp, _ := env.app.Test(authRequest(http.MethodPost, restoreURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- AV-1.6: Delete version ---

func TestDeleteVersion_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	// Upload v2 so we can delete v1 later.
	r2 := buildVersionUploadRequest(t, asset.ID, "v2.jpg", makeJPEG(200, 200), "", owner.Cookie)
	resp2, _ := env.app.Test(r2, fiber.TestConfig{Timeout: 5000})
	if resp2.StatusCode != http.StatusCreated {
		t.Skip("v2 upload failed")
	}

	// Restore v1 to make v2 non-current so we can delete it.
	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck

	// Find v2 (non-current after we restore v1).
	var v2ID, v1ID string
	for _, v := range versions {
		if v.VersionNum == 2 {
			v2ID = v.ID
		}
		if v.VersionNum == 1 {
			v1ID = v.ID
		}
	}

	// First restore v1 to make v2 non-current.
	restoreURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/restore", asset.ID, v1ID)
	env.app.Test(authRequest(http.MethodPost, restoreURL, nil, owner.Cookie)) //nolint:errcheck

	// Now delete v2 (non-current).
	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, v2ID)
	deleteResp, err := env.app.Test(authRequest(http.MethodDelete, deleteURL, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("delete: %v", err)
	}
	if deleteResp.StatusCode != http.StatusNoContent {
		b, _ := io.ReadAll(deleteResp.Body)
		t.Fatalf("expected 204, got %d: %s", deleteResp.StatusCode, b)
	}

	// Deleted version should not appear in active list.
	listResp2, _ := env.app.Test(authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie))
	var versions2 []versionResponse
	json.NewDecoder(listResp2.Body).Decode(&versions2) //nolint:errcheck
	for _, v := range versions2 {
		if v.ID == v2ID {
			t.Error("deleted version still appears in active list")
		}
	}
}

func TestDeleteVersion_CannotDeleteCurrent(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, versions[0].ID)
	resp, _ := env.app.Test(authRequest(http.MethodDelete, deleteURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422 for deleting current version, got %d", resp.StatusCode)
	}
}

func TestDeleteVersion_ViewerForbidden(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)
	viewerToken := mintEditorToken(t, env, owner.WorkspaceID, "viewer")

	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck

	deleteURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s", asset.ID, versions[0].ID)
	req := httptest.NewRequest(http.MethodDelete, deleteURL, nil)
	req.Header.Set("Authorization", "Bearer "+viewerToken)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

// --- AV-1.7: File + thumb endpoints ---

func TestGetVersionFile_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck
	if len(versions) == 0 {
		t.Fatal("no versions")
	}

	fileURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/file", asset.ID, versions[0].ID)
	resp, err := env.app.Test(authRequest(http.MethodGet, fileURL, nil, owner.Cookie))
	if err != nil {
		t.Fatalf("get version file: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		b, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, b)
	}
	body, _ := io.ReadAll(resp.Body)
	if len(body) == 0 {
		t.Error("expected non-empty file content")
	}
}

func TestGetVersionFile_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	fileURL := fmt.Sprintf("/api/v1/assets/%s/versions/nonexistent/file", asset.ID)
	resp, _ := env.app.Test(authRequest(http.MethodGet, fileURL, nil, owner.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetVersionThumb_NotReady(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	asset := uploadAsset(t, env, owner.Cookie)

	listReq := authRequest(http.MethodGet, fmt.Sprintf("/api/v1/assets/%s/versions", asset.ID), nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var versions []versionResponse
	json.NewDecoder(listResp.Body).Decode(&versions) //nolint:errcheck

	thumbURL := fmt.Sprintf("/api/v1/assets/%s/versions/%s/thumb", asset.ID, versions[0].ID)
	resp, _ := env.app.Test(authRequest(http.MethodGet, thumbURL, nil, owner.Cookie))
	// Thumbnail won't be ready immediately (job is async) — expect 404.
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 (thumb not ready), got %d", resp.StatusCode)
	}
}

// --- Workspace isolation ---

func TestVersions_WorkspaceIsolation(t *testing.T) {
	env := setupTestApp(t)
	alice := register(t, env, "Alice", "alice@example.com", "password123")
	bob := register(t, env, "Bob", "bob@example.com", "password123")

	aliceAsset := uploadAsset(t, env, alice.Cookie)

	// Bob cannot list Alice's asset's versions.
	listURL := fmt.Sprintf("/api/v1/assets/%s/versions", aliceAsset.ID)
	resp, _ := env.app.Test(authRequest(http.MethodGet, listURL, nil, bob.Cookie))
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}

	// Bob cannot upload a version to Alice's asset.
	uploadReq := buildVersionUploadRequest(t, aliceAsset.ID, "v2.jpg", makeJPEG(10, 10), "", bob.Cookie)
	uploadResp, _ := env.app.Test(uploadReq, fiber.TestConfig{Timeout: 5000})
	if uploadResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", uploadResp.StatusCode)
	}
}

// --- Retention policy helpers ---

func TestNextRunAt(t *testing.T) {
	// 01:59 UTC — next run is 02:00 same day
	t1 := time.Date(2026, 4, 5, 1, 59, 0, 0, time.UTC)
	next1 := nextRunAt(t1)
	if next1.Hour() != 2 || next1.Day() != 5 {
		t.Errorf("expected 02:00 same day, got %v", next1)
	}

	// 02:01 UTC — next run is 02:00 next day
	t2 := time.Date(2026, 4, 5, 2, 1, 0, 0, time.UTC)
	next2 := nextRunAt(t2)
	if next2.Day() != 6 {
		t.Errorf("expected 02:00 next day, got %v", next2)
	}
}
