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

func TestListWorkflowsEmptyWorkspaceReturnsPersistedRowsOnly(t *testing.T) {
	env := testutil.NewTestEnv(t)
	createCalls := 0
	env.Workflows.ListFn = func(_ context.Context, workspaceID string) ([]service.WorkflowDTO, error) {
		if workspaceID != "ws_1" {
			t.Fatalf("workspaceID = %q, want ws_1", workspaceID)
		}
		return []service.WorkflowDTO{}, nil
	}
	env.Workflows.CreateFn = func(_ context.Context, _, _ string, _ service.CreateWorkflowParams) (*service.WorkflowDTO, error) {
		createCalls++
		return nil, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got []map[string]any
	testutil.DecodeJSON(t, resp, &got)
	if len(got) != 0 {
		t.Fatalf("expected no persisted workflows, got %d", len(got))
	}
	if createCalls != 0 {
		t.Fatalf("Create() called %d times during list, want 0", createCalls)
	}
}

func TestGetWorkflowTemplatesOK(t *testing.T) {
	env := testutil.NewTestEnv(t)
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{{
			ID:          "blank-manual",
			Name:        "Start Blank",
			Description: "Manual trigger.",
			TriggerType: "trigger.manual",
			Graph:       `{"nodes":[],"edges":[]}`,
		}}
	}
	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)
}

func TestGetWorkflowTemplatesDoesNotMaterializeWorkflowRows(t *testing.T) {
	env := testutil.NewTestEnv(t)
	createCalls := 0
	env.Workflows.TemplatesFn = func() []service.WorkflowTemplateDTO {
		return []service.WorkflowTemplateDTO{{
			ID:          "blank-manual",
			Name:        "Start Blank",
			Description: "Manual trigger.",
			TriggerType: "trigger.manual",
			Graph:       `{"nodes":[],"edges":[]}`,
		}}
	}
	env.Workflows.CreateFn = func(_ context.Context, _, _ string, _ service.CreateWorkflowParams) (*service.WorkflowDTO, error) {
		createCalls++
		return nil, nil
	}

	req := testutil.BearerRequest(http.MethodGet, "/api/v1/workflows/templates", nil, env.MintToken(t, "usr_1", "ws_1"))
	resp, err := env.App.Test(req)
	if err != nil {
		t.Fatal(err)
	}
	testutil.AssertStatus(t, resp, http.StatusOK)

	var got []map[string]any
	testutil.DecodeJSON(t, resp, &got)
	if len(got) != 1 {
		t.Fatalf("expected 1 template, got %d", len(got))
	}
	if got[0]["id"] != "blank-manual" {
		t.Fatalf("template id = %v, want blank-manual", got[0]["id"])
	}
	if createCalls != 0 {
		t.Fatalf("Create() called %d times during templates fetch, want 0", createCalls)
	}
}
