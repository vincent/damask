package api_test

import (
	"archive/zip"
	"bytes"
	"encoding/json"
	"io"
	"net/http"
	"strings"
	"testing"

	th "damask/server/internal/tests_helpers"

	"github.com/gofiber/fiber/v3"
)

func TestStackExport_EmptyAssetIDs(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		th.JsonBody(map[string]any{"asset_ids": []string{}}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestStackExport_WrongWorkspace(t *testing.T) {
	// Two separate apps = two separate workspaces.
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	// Upload an asset in workspace 2.
	otherAsset := th.UploadAsset(t, env2, owner2.Cookie)

	// Request it via workspace 1's session — asset ID does not exist in ws1 → 403.
	resp, err := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		th.JsonBody(map[string]any{"asset_ids": []string{otherAsset.ID}}), owner1.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestStackExport_ValidZip(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	asset := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		th.JsonBody(map[string]any{"asset_ids": []string{asset.ID}, "filename": "my-export"}), owner.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	if ct := resp.Header.Get("Content-Type"); ct != "application/zip" {
		t.Errorf("Content-Type: got %q, want application/zip", ct)
	}
	if cd := resp.Header.Get("Content-Disposition"); !strings.Contains(cd, "my-export.zip") {
		t.Errorf("Content-Disposition: got %q, want my-export.zip", cd)
	}

	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}
	if len(zr.File) == 0 {
		t.Error("expected at least one file in zip")
	}
}

func TestStackExport_DuplicateFilenames(t *testing.T) {
	env, owner := th.SetupWithOwner(t)

	// Upload two assets — both get the same filename "original.jpg" from the helper.
	a1 := th.UploadAsset(t, env, owner.Cookie)
	a2 := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		th.JsonBody(map[string]any{"asset_ids": []string{a1.ID, a2.ID}}), owner.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 200, got %d: %s", resp.StatusCode, body)
	}

	data, _ := io.ReadAll(resp.Body)
	zr, err := zip.NewReader(bytes.NewReader(data), int64(len(data)))
	if err != nil {
		t.Fatalf("not a valid zip: %v", err)
	}

	names := map[string]bool{}
	for _, f := range zr.File {
		if names[f.Name] {
			t.Errorf("duplicate filename in zip: %s", f.Name)
		}
		names[f.Name] = true
	}
	if len(names) != 2 {
		t.Errorf("expected 2 unique files, got %d", len(names))
	}
}

func TestStackExport_FilenameSanitised(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	asset := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/export",
		th.JsonBody(map[string]any{"asset_ids": []string{asset.ID}, "filename": "../../../etc/passwd"}), owner.Cookie),
		fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	cd := resp.Header.Get("Content-Disposition")
	if strings.Contains(cd, "/") || strings.Contains(cd, "\\") {
		t.Errorf("Content-Disposition contains path separators: %s", cd)
	}
}

func TestStackExport_Unauthenticated(t *testing.T) {
	env, _ := th.SetupWithOwner(t)

	body, _ := json.Marshal(map[string]any{"asset_ids": []string{"x"}})
	req, _ := http.NewRequest(http.MethodPost, "/api/v1/stack/export", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")

	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", resp.StatusCode)
	}
}
