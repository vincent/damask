package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	th "damask/server/internal/tests_helpers"

	"github.com/gofiber/fiber/v3"
)

// uploadTestJPEG uploads a JPEG with a specified filename and returns the asset ID.
func uploadTestJPEG(t *testing.T, env *th.TestEnv, cookie *http.Cookie, filename string) string {
	t.Helper()
	req := th.BuildUploadRequest(t, filename, th.MakeJPEG(10, 10), cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("upload: %v", err)
	}
	var a api.AssetResponse
	_ = json.NewDecoder(resp.Body).Decode(&a)
	return a.ID
}

func TestRenameAsset_Success(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "photo.jpg")

	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "renamed"}), owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if asset.OriginalFilename != "renamed.jpg" {
		t.Errorf("filename: got %q, want %q", asset.OriginalFilename, "renamed.jpg")
	}
}

func TestRenameAsset_ExtensionPreserved(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "original.jpg")

	// Client sends stem with extension included — backend must not duplicate it.
	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "newname.jpg"}), owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if asset.OriginalFilename != "newname.jpg" {
		t.Errorf("filename: got %q, want %q (no extension duplication)", asset.OriginalFilename, "newname.jpg")
	}
}

func TestRenameAsset_NoOp(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "photo.jpg")

	// Sending the same stem as the current name — should return 200 with no change.
	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "photo"}), owner.Cookie)
	resp, err := env.App.Test(req, fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	var asset api.AssetResponse
	if err := json.NewDecoder(resp.Body).Decode(&asset); err != nil {
		t.Fatalf("decode: %v", err)
	}
	if asset.OriginalFilename != "photo.jpg" {
		t.Errorf("filename: got %q, want %q", asset.OriginalFilename, "photo.jpg")
	}
}

func TestRenameAsset_Unauthenticated(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "photo.jpg")

	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "new"}), nil)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}

func TestRenameAsset_ViewerForbidden(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "photo.jpg")

	viewerToken := th.MintEditorToken(t, env, owner.WorkspaceID, auth.Viewer)
	req := th.BearerRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "new"}), viewerToken)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestRenameAsset_EmptyName(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	assetID := uploadTestJPEG(t, env, owner.Cookie, "photo.jpg")

	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/"+assetID+"/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "   "}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestRenameAsset_NotFound(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	req := th.AuthRequest(http.MethodPut, "/api/v1/assets/nonexistent/rename",
		th.JsonBody(api.RenameAssetRequest{Name: "new"}), owner.Cookie)
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatalf("request: %v", err)
	}
	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("expected 404, got %d", resp.StatusCode)
	}
}
