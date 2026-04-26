package api_test

import (
	"context"
	"fmt"
	"net/http"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestStackMerge_ValidEnqueue(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Stack.EnqueueMergeFn = func(_ context.Context, _, _ string, _ service.MergeParams) (string, error) {
		return "job_abc123", nil
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		testutil.JsonBody(map[string]any{
			"asset_ids":   []string{"ast_1", "ast_2"},
			"output_type": "gif",
			"filename":    "my-merge",
		}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusAccepted)

	var result map[string]any
	testutil.DecodeJSON(t, resp, &result)
	if result["job_id"] == "" {
		t.Error("expected non-empty job_id")
	}
}

func TestStackMerge_ValidationTooFewAssets(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		testutil.JsonBody(map[string]any{
			"asset_ids":   []string{"ast_1"},
			"output_type": "gif",
		}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestStackMerge_ValidationBadOutputType(t *testing.T) {
	env := testutil.NewTestEnv(t)
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		testutil.JsonBody(map[string]any{
			"asset_ids":   []string{"ast_1", "ast_2"},
			"output_type": "mp4",
		}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnprocessableEntity)
}

func TestStackMerge_WrongWorkspace(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Stack.EnqueueMergeFn = func(_ context.Context, _, _ string, _ service.MergeParams) (string, error) {
		return "", fmt.Errorf("asset not in workspace: %w", apperr.ErrForbidden)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		testutil.JsonBody(map[string]any{
			"asset_ids":   []string{"ast_local", "ast_foreign"},
			"output_type": "gif",
		}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestStackMerge_AllForeignAssets(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Stack.EnqueueMergeFn = func(_ context.Context, _, _ string, _ service.MergeParams) (string, error) {
		return "", fmt.Errorf("asset not in workspace: %w", apperr.ErrForbidden)
	}
	cookie := env.MintCookie(t, "usr_1", "ws_1")

	resp, err := env.App.Test(testutil.AuthRequest(http.MethodPost, "/api/v1/stack/merge",
		testutil.JsonBody(map[string]any{
			"asset_ids":   []string{"ast_f1", "ast_f2"},
			"output_type": "gif",
		}), cookie))
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusForbidden)
}

func TestStackMerge_Unauthenticated(t *testing.T) {
	env := testutil.NewTestEnv(t)

	req, _ := http.NewRequest(http.MethodPost, "/api/v1/stack/merge", nil)
	req.Header.Set("Content-Type", "application/json")
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusUnauthorized)
}
