package api

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strings"
	"testing"
)

// createShare is a test helper that POSTs to /api/v1/shares.
func createShare(t *testing.T, env *testEnv, cookie *http.Cookie, body string) shareResponse {
	t.Helper()
	req := authRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(body), cookie)
	resp, err := env.app.Test(req)
	if err != nil {
		t.Fatalf("create share request: %v", err)
	}
	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("expected 201, got %d", resp.StatusCode)
	}
	var s shareResponse
	if err := json.NewDecoder(resp.Body).Decode(&s); err != nil {
		t.Fatalf("decode share: %v", err)
	}
	return s
}

// --- S-2: POST /shares ---

func TestCreateShare_ProjectTarget(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Nike Q3", "#ff0000")

	body := fmt.Sprintf(`{"label":"Nike Q3 delivery","target_type":"project","target_id":%q,"allow_download":true}`, p.ID)
	sh := createShare(t, env, owner.Cookie, body)

	if sh.TargetType != "project" {
		t.Errorf("target_type = %q, want project", sh.TargetType)
	}
	if sh.TargetID != p.ID {
		t.Errorf("target_id mismatch")
	}
	if sh.Label != "Nike Q3 delivery" {
		t.Errorf("label = %q, want Nike Q3 delivery", sh.Label)
	}
	if !sh.AllowDownload {
		t.Errorf("expected allow_download = true")
	}
	if sh.HasPassword {
		t.Errorf("expected has_password = false")
	}
	if sh.PublicURL == "" || !strings.Contains(sh.PublicURL, "/s/"+sh.ID) {
		t.Errorf("public_url = %q, expected to contain /s/%s", sh.PublicURL, sh.ID)
	}
	if sh.WorkspaceID != owner.WorkspaceID {
		t.Errorf("workspace_id mismatch")
	}
}

func TestCreateShare_AssetTarget(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	assetID := uploadTestAsset(t, env, owner)

	body := fmt.Sprintf(`{"target_type":"asset","target_id":%q}`, assetID)
	sh := createShare(t, env, owner.Cookie, body)

	if sh.TargetType != "asset" {
		t.Errorf("target_type = %q, want asset", sh.TargetType)
	}
	if sh.TargetID != assetID {
		t.Errorf("target_id mismatch")
	}
}

func TestCreateShare_WithPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Secret", "#000")

	body := fmt.Sprintf(`{"target_type":"project","target_id":%q,"password":"hunter2"}`, p.ID)
	sh := createShare(t, env, owner.Cookie, body)

	if !sh.HasPassword {
		t.Errorf("expected has_password = true")
	}
}

func TestCreateShare_WithExpiry(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "Expiring", "#000")

	body := fmt.Sprintf(`{"target_type":"project","target_id":%q,"expires_in_days":14}`, p.ID)
	sh := createShare(t, env, owner.Cookie, body)

	if sh.ExpiresAt == nil {
		t.Errorf("expected expires_at to be set")
	}
	if sh.IsExpired {
		t.Errorf("expected is_expired = false for future expiry")
	}
}

func TestCreateShare_InvalidTargetType(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/shares",
		jsonStr(`{"target_type":"unknown","target_id":"abc"}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestCreateShare_TargetNotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodPost, "/api/v1/shares",
		jsonStr(`{"target_type":"project","target_id":"nonexistent"}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestCreateShare_TargetFromOtherWorkspace(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")
	p := createProject(t, env, owner1.Cookie, "Private", "#000")

	// owner2 tries to share owner1's project
	body := fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID)
	req := authRequest(http.MethodPost, "/api/v1/shares", strings.NewReader(body), owner2.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404 for cross-workspace target, got %d", resp.StatusCode)
	}
}

func TestCreateShare_Unauthenticated(t *testing.T) {
	env := setupTestApp(t)
	req := authRequest(http.MethodPost, "/api/v1/shares",
		jsonStr(`{"target_type":"project","target_id":"x"}`), nil)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

// --- S-3: GET /shares ---

func TestListShares_Empty(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/shares", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var items []shareResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 0 {
		t.Errorf("expected 0 shares, got %d", len(items))
	}
}

func TestListShares_WorkspaceIsolation(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")

	p := createProject(t, env, owner1.Cookie, "Alpha", "#000")
	createShare(t, env, owner1.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodGet, "/api/v1/shares", nil, owner2.Cookie)
	resp, _ := env.app.Test(req)
	var items []shareResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck
	if len(items) != 0 {
		t.Errorf("owner2 should see 0 shares, got %d", len(items))
	}
}

// --- S-3: GET /shares/:id ---

func TestGetShare_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got shareResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.ID != sh.ID {
		t.Errorf("id mismatch")
	}
}

func TestGetShare_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodGet, "/api/v1/shares/nonexistent", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestGetShare_OtherWorkspace(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")
	p := createProject(t, env, owner1.Cookie, "P", "#000")
	sh := createShare(t, env, owner1.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, owner2.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- S-3: PUT /shares/:id ---

func TestUpdateShare_Label(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"label":"Old","target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodPut, "/api/v1/shares/"+sh.ID,
		jsonStr(`{"label":"New Label"}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("expected 200, got %d", resp.StatusCode)
	}
	var got shareResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if got.Label != "New Label" {
		t.Errorf("label = %q, want New Label", got.Label)
	}
}

func TestUpdateShare_SetAndClearPassword(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	// Set password
	req := authRequest(http.MethodPut, "/api/v1/shares/"+sh.ID,
		jsonStr(`{"password":"s3cr3t"}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("set password: expected 200, got %d", resp.StatusCode)
	}
	var got shareResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if !got.HasPassword {
		t.Errorf("expected has_password = true after setting password")
	}

	// Clear password
	req2 := authRequest(http.MethodPut, "/api/v1/shares/"+sh.ID,
		jsonStr(`{"clear_password":true}`), owner.Cookie)
	resp2, _ := env.app.Test(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("clear password: expected 200, got %d", resp2.StatusCode)
	}
	var got2 shareResponse
	json.NewDecoder(resp2.Body).Decode(&got2) //nolint:errcheck
	if got2.HasPassword {
		t.Errorf("expected has_password = false after clearing password")
	}
}

func TestUpdateShare_AllowComments(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	if sh.AllowComments {
		t.Fatalf("expected allow_comments = false by default")
	}

	req := authRequest(http.MethodPut, "/api/v1/shares/"+sh.ID,
		jsonStr(`{"allow_comments":true}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	var got shareResponse
	json.NewDecoder(resp.Body).Decode(&got) //nolint:errcheck
	if !got.AllowComments {
		t.Errorf("expected allow_comments = true after update")
	}
}

func TestUpdateShare_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodPut, "/api/v1/shares/nonexistent",
		jsonStr(`{"label":"x"}`), owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

// --- S-3: DELETE /shares/:id (soft revoke) ---

func TestRevokeShare_Success(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNoContent {
		t.Fatalf("expected 204, got %d", resp.StatusCode)
	}

	// Share should still be retrievable (soft delete) with revoked_at set
	req2 := authRequest(http.MethodGet, "/api/v1/shares/"+sh.ID, nil, owner.Cookie)
	resp2, _ := env.app.Test(req2)
	if resp2.StatusCode != http.StatusOK {
		t.Fatalf("share should still be accessible after soft revoke, got %d", resp2.StatusCode)
	}
	var got shareResponse
	json.NewDecoder(resp2.Body).Decode(&got) //nolint:errcheck
	if got.RevokedAt == nil {
		t.Errorf("expected revoked_at to be set after DELETE")
	}
}

func TestRevokeShare_NotFound(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")

	req := authRequest(http.MethodDelete, "/api/v1/shares/nonexistent", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestRevokeShare_OtherWorkspace(t *testing.T) {
	env := setupTestApp(t)
	owner1 := register(t, env, "Owner1", "owner1@example.com", "password123")
	owner2 := register(t, env, "Owner2", "owner2@example.com", "password123")
	p := createProject(t, env, owner1.Cookie, "P", "#000")
	sh := createShare(t, env, owner1.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q}`, p.ID))

	req := authRequest(http.MethodDelete, "/api/v1/shares/"+sh.ID, nil, owner2.Cookie)
	resp, _ := env.app.Test(req)
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}

func TestListShares_IncludesIsExpired(t *testing.T) {
	env := setupTestApp(t)
	owner := register(t, env, "Owner", "owner@example.com", "password123")
	p := createProject(t, env, owner.Cookie, "P", "#000")
	sh := createShare(t, env, owner.Cookie, fmt.Sprintf(`{"target_type":"project","target_id":%q,"expires_in_days":7}`, p.ID))

	// Force expires_at into the past
	_, err := env.sqlDB.Exec(
		`UPDATE shares SET expires_at = datetime('now', '-1 day') WHERE id = ?`, sh.ID,
	)
	if err != nil {
		t.Fatalf("expire share: %v", err)
	}

	req := authRequest(http.MethodGet, "/api/v1/shares", nil, owner.Cookie)
	resp, _ := env.app.Test(req)
	var items []shareResponse
	json.NewDecoder(resp.Body).Decode(&items) //nolint:errcheck

	if len(items) != 1 {
		t.Fatalf("expected 1 share, got %d", len(items))
	}
	if !items[0].IsExpired {
		t.Errorf("expected is_expired = true for past expires_at")
	}
}
