package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ── helpers ───────────────────────────────────────────────────────────────────

// accessShare calls POST /s/:id/access with an optional password and returns
// the share session token on success.
func accessShare(t *testing.T, env *testEnv, shareID, password string) string {
	t.Helper()
	var body string
	if password != "" {
		body = fmt.Sprintf(`{"password":%q}`, password)
	} else {
		body = `{}`
	}
	req := httptest.NewRequest(http.MethodPost, "/s/"+shareID+"/access", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("access share: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var res shareAccessResponse
	if err := json.NewDecoder(resp.Body).Decode(&res); err != nil {
		t.Fatalf("decode access response: %v", err)
	}
	return res.Token
}

// shareRequest builds an HTTP request with a Bearer share session token.
func shareRequest(method, path string, body string, token string) *http.Request {
	var req *http.Request
	if body != "" {
		req = httptest.NewRequest(method, path, strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
	} else {
		req = httptest.NewRequest(method, path, nil)
	}
	if token != "" {
		req.Header.Set("Authorization", "Bearer "+token)
	}
	return req
}

// ── S-4: POST /s/:id/access ───────────────────────────────────────────────────

func TestShareAccess_NoPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	token := accessShare(t, env, sh.ID, "")
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestShareAccess_WithCorrectPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q,"password":"s3cr3t"}`, p.ID))

	token := accessShare(t, env, sh.ID, "s3cr3t")
	if token == "" {
		t.Error("expected non-empty token")
	}
}

func TestShareAccess_WrongPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q,"password":"s3cr3t"}`, p.ID))

	req := httptest.NewRequest(http.MethodPost, "/s/"+sh.ID+"/access", jsonStr(`{"password":"wrong"}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareAccess_MissingPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q,"password":"s3cr3t"}`, p.ID))

	req := httptest.NewRequest(http.MethodPost, "/s/"+sh.ID+"/access", jsonStr(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareAccess_NotFound(t *testing.T) {
	env := setupTestApp(t)
	req := httptest.NewRequest(http.MethodPost, "/s/nonexistent/access", jsonStr(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestShareAccess_Revoked(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	// Revoke the share
	req := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	env.app.Test(req) //nolint:errcheck

	req2 := httptest.NewRequest(http.MethodPost, "/s/"+sh.ID+"/access", jsonStr(`{}`))
	req2.Header.Set("Content-Type", "application/json")
	resp, _ := env.app.Test(req2)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for revoked share, got %d", resp.StatusCode)
	}
}

func TestShareAccess_Expired(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q,"expires_in_days":7}`, p.ID))

	// Force expiry
	env.sqlDB.Exec(`UPDATE shares SET expires_at = datetime('now', '-1 day') WHERE id = ?`, sh.ID) //nolint:errcheck

	req := httptest.NewRequest(http.MethodPost, "/s/"+sh.ID+"/access", jsonStr(`{}`))
	req.Header.Set("Content-Type", "application/json")
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 for expired share, got %d", resp.StatusCode)
	}
}

func TestShareAccess_IncrementsViewCount(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	if sh.ViewCount != 0 {
		t.Fatalf("expected view_count = 0, got %d", sh.ViewCount)
	}

	accessShare(t, env, sh.ID, "")

	// Check via owner API
	req := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	var got shareResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.ViewCount != 1 {
		t.Errorf("expected view_count = 1 after access, got %d", got.ViewCount)
	}
}

// ── S-5: public content endpoints ────────────────────────────────────────────

func TestShareListAssets_Project(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Project", "#f00")
	assetID := uploadTestAsset(t, env, owner)
	// Assign asset to project directly via SQL
	if _, err := env.sqlDB.Exec(`UPDATE assets SET project_id = ? WHERE id = ?`, p.ID, assetID); err != nil {
		t.Fatalf("assign asset to project: %v", err)
	}

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets", "", token)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShareListAssets_SingleAsset(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets", "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []assetResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 1 || items[0].ID != assetID {
		t.Errorf("expected 1 asset with id %s, got %v", assetID, items)
	}
}

func TestShareListAssets_NoToken(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := httptest.NewRequest(http.MethodGet, "/s/"+sh.ID+"/assets", nil)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestShareListAssets_WrongShareToken(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p1 := createProject(t, env, owner.Cookie, "P1", "#000")
	p2 := createProject(t, env, owner.Cookie, "P2", "#000")
	sh1 := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p1.ID))
	sh2 := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p2.ID))
	token1 := accessShare(t, env, sh1.ID, "")

	// Use sh1 token on sh2 path
	req := shareRequest(http.MethodGet, "/s/"+sh2.ID+"/assets", "", token1)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for mismatched share token, got %d", resp.StatusCode)
	}
}

func TestShareGetAsset_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets/"+assetID, "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got assetResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.ID != assetID {
		t.Errorf("asset id mismatch: got %s", got.ID)
	}
}

func TestShareGetAsset_NotInShare(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID1 := uploadTestAsset(t, env, owner)
	assetID2 := uploadTestAsset(t, env, owner)

	// Share only targets assetID1
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q}`, assetID1))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets/"+assetID2, "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for asset not in share, got %d", resp.StatusCode)
	}
}

func TestShareGetAssetFile_AllowDownloadFalse(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q,"allow_download":false}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets/"+assetID+"/file", "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when allow_download=false, got %d", resp.StatusCode)
	}
}

func TestShareGetAssetFile_AllowDownloadTrue(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q,"allow_download":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets/"+assetID+"/file", "", token)
	resp, _ := env.app.Test(req)
	// 200 = file streamed successfully
	if resp.StatusCode != http.StatusOK {
		t.Errorf("expected 200, got %d", resp.StatusCode)
	}
}

func TestShareContentRecheck_RevokedMidSession(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	// Obtain a valid token
	token := accessShare(t, env, sh.ID, "")

	// Owner revokes the share
	revokeReq := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	env.app.Test(revokeReq) //nolint:errcheck

	// Subsequent content request should be rejected
	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets", "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusGone {
		t.Errorf("expected 410 after share revoked mid-session, got %d", resp.StatusCode)
	}
}

// ── S-6: public comment endpoints ────────────────────────────────────────────

func TestShareCreateComment_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Looks great!"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var got commentResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.AuthorName != "Sarah" {
		t.Errorf("author_name = %q, want Sarah", got.AuthorName)
	}
	if got.Body != "Looks great!" {
		t.Errorf("body = %q, want 'Looks great!'", got.Body)
	}
	if got.AssetID != assetID {
		t.Errorf("asset_id mismatch")
	}
}

func TestShareCreateComment_CommentsNotAllowed(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	// allow_comments defaults to false
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"asset","target_id":%q}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Hi"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 when comments disabled, got %d", resp.StatusCode)
	}
}

func TestShareCreateComment_MissingAuthorName(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"body":"Hi"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", resp.StatusCode)
	}
}

func TestShareCreateComment_AssetNotInShare(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID1 := uploadTestAsset(t, env, owner)
	assetID2 := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID1))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Sarah","body":"Hi"}`, assetID2)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for asset not in share, got %d", resp.StatusCode)
	}
}

func TestShareListComments_Grouped(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")

	// Upload two assets and assign them to the project directly via SQL.
	a1 := uploadTestAsset(t, env, owner)
	a2 := uploadTestAsset(t, env, owner)
	if _, err := env.sqlDB.Exec(`UPDATE assets SET project_id = ? WHERE id IN (?, ?)`, p.ID, a1, a2); err != nil {
		t.Fatalf("assign assets to project: %v", err)
	}

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"project","target_id":%q,"allow_comments":true}`, p.ID))
	token := accessShare(t, env, sh.ID, "")

	// Post one comment on each asset
	postComment := func(assetID, author, body string) {
		t.Helper()
		b := fmt.Sprintf(`{"asset_id":%q,"author_name":%q,"body":%q}`, assetID, author, body)
		req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", b, token)
		env.app.Test(req) //nolint:errcheck
	}
	postComment(a1, "Alice", "Nice!")
	postComment(a2, "Bob", "Cool")

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/comments", "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}

	type group struct {
		AssetID  string            `json:"asset_id"`
		Comments []commentResponse `json:"comments"`
	}
	var groups []group
	json.NewDecoder(resp.Body).Decode(&groups) //nolint:errcheck
	if len(groups) != 2 {
		t.Errorf("expected 2 groups, got %d", len(groups))
	}
}

func TestShareListAssetComments(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	// Post two comments
	for i := 0; i < 2; i++ {
		body := fmt.Sprintf(`{"asset_id":%q,"author_name":"User%d","body":"Comment %d"}`, assetID, i, i)
		req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
		env.app.Test(req) //nolint:errcheck
	}

	req := shareRequest(http.MethodGet, "/s/"+sh.ID+"/assets/"+assetID+"/comments", "", token)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []commentResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 2 {
		t.Errorf("expected 2 comments, got %d", len(items))
	}
}

// ── S-7: owner moderation ─────────────────────────────────────────────────────

func TestOwnerListComments_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Feedback"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	env.app.Test(req) //nolint:errcheck

	req2 := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner.Cookie)
	resp, _ := env.app.Test(req2)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []commentResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 1 {
		t.Errorf("expected 1 comment, got %d", len(items))
	}
}

func TestOwnerListComments_WrongWorkspace(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")
	p := createProject(t, env, owner1.Cookie, "P", "#000")
	sh := createShare(t, env, owner1.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner2.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestOwnerDeleteComment_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	// Post a comment
	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Delete me"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	var comment commentResponse
	json.NewDecoder(resp.Body).Decode(&comment) //nolint:errcheck

	// Owner deletes the comment
	delReq := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID+"/comments/"+comment.ID, nil, owner.Cookie)
	delResp, _ := env.app.Test(delReq)
	if delResp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", delResp.StatusCode)
	}

	// Confirm deleted
	listReq := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID+"/comments", nil, owner.Cookie)
	listResp, _ := env.app.Test(listReq)
	var items []commentResponse
	json.NewDecoder(listResp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 0 {
		t.Errorf("expected 0 comments after delete, got %d", len(items))
	}
}

func TestOwnerDeleteComment_WrongWorkspace(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner1)

	sh := createShare(t, env, owner1.Cookie, fmt.Sprintf(
		`{"target_type":"asset","target_id":%q,"allow_comments":true}`, assetID))
	token := accessShare(t, env, sh.ID, "")

	body := fmt.Sprintf(`{"asset_id":%q,"author_name":"Client","body":"Feedback"}`, assetID)
	req := shareRequest(http.MethodPost, "/s/"+sh.ID+"/comments", body, token)
	resp, _ := env.app.Test(req)
	var comment commentResponse
	json.NewDecoder(resp.Body).Decode(&comment) //nolint:errcheck

	// owner2 tries to delete it
	delReq := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID+"/comments/"+comment.ID, nil, owner2.Cookie)
	delResp, _ := env.app.Test(delReq)
	if delResp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for cross-workspace delete, got %d", delResp.StatusCode)
	}
}
