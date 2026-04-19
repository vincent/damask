package api_test

import (
	"encoding/json"
	"io"
	"net/http"
	"testing"

	th "damask/server/internal/tests_helpers"

	"github.com/gofiber/fiber/v3"
)

func TestStackMerge_ValidEnqueue(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a1 := th.UploadAsset(t, env, owner.Cookie)
	a2 := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		th.JsonBody(map[string]any{
			"asset_ids":   []string{a1.ID, a2.ID},
			"output_type": "gif",
			"filename":    "my-merge",
		}), owner.Cookie), fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusAccepted {
		body, _ := io.ReadAll(resp.Body)
		t.Fatalf("expected 202, got %d: %s", resp.StatusCode, body)
	}

	var result map[string]any
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		t.Fatal(err)
	}
	if result["job_id"] == "" {
		t.Error("expected non-empty job_id")
	}
}

func TestStackMerge_ValidationTooFewAssets(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		th.JsonBody(map[string]any{
			"asset_ids":   []string{a.ID},
			"output_type": "gif",
		}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestStackMerge_ValidationBadOutputType(t *testing.T) {
	env, owner := th.SetupWithOwner(t)
	a1 := th.UploadAsset(t, env, owner.Cookie)
	a2 := th.UploadAsset(t, env, owner.Cookie)

	resp, err := env.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		th.JsonBody(map[string]any{
			"asset_ids":   []string{a1.ID, a2.ID},
			"output_type": "mp4",
		}), owner.Cookie))
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusUnprocessableEntity {
		t.Errorf("expected 422, got %d", resp.StatusCode)
	}
}

func TestStackMerge_WrongWorkspace(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	otherAsset := th.UploadAsset(t, env2, owner2.Cookie)
	localAsset := th.UploadAsset(t, env1, owner1.Cookie)

	resp, err := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		th.JsonBody(map[string]any{
			"asset_ids":   []string{localAsset.ID, otherAsset.ID},
			"output_type": "gif",
		}), owner1.Cookie), fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403, got %d", resp.StatusCode)
	}
}

func TestStackMerge_AllForeignAssets(t *testing.T) {
	env1, owner1 := th.SetupWithOwner(t)
	env2, owner2 := th.SetupWithOwner(t)

	a1 := th.UploadAsset(t, env2, owner2.Cookie)
	a2 := th.UploadAsset(t, env2, owner2.Cookie)

	resp, err := env1.App.Test(th.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		th.JsonBody(map[string]any{
			"asset_ids":   []string{a1.ID, a2.ID},
			"output_type": "gif",
		}), owner1.Cookie), fiber.TestConfig{Timeout: 5000})
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("expected 403 for all-foreign assets, got %d", resp.StatusCode)
	}
}

func TestStackMerge_Unauthenticated(t *testing.T) {
	env, _ := th.SetupWithOwner(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/stack/merge", nil)
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
