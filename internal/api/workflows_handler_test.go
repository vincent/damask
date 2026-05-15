package api_test

import (
	"context"
	"net/http"
	"testing"

	"damask/server/internal/service"
	"damask/server/internal/testutil"
)

func TestListWorkflowsOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.ListFn = func(_ context.Context, workspaceID string) ([]service.WorkflowDTO, error) {
		return []service.WorkflowDTO{{ID: "wf_1", WorkspaceID: workspaceID, Name: "Auto Resize"}}, nil
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}
