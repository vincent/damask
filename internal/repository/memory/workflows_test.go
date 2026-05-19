package memory_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
)

func TestFindCoveringWorkflowPriority(t *testing.T) {
	repo := memory.NewWorkflowRepo()
	repo.Seed(
		repository.Workflow{ID: "wf_workspace", WorkspaceID: "ws_1", Name: "Workspace", Enabled: true, TriggerType: "trigger.version_uploaded", TriggerConfig: "{}"},
		repository.Workflow{ID: "wf_project", WorkspaceID: "ws_1", Name: "Project", Enabled: true, TriggerType: "trigger.version_uploaded", TriggerConfig: `{"project_id":"prj_1"}`},
		repository.Workflow{ID: "wf_folder", WorkspaceID: "ws_1", Name: "Folder", Enabled: true, TriggerType: "trigger.version_uploaded", TriggerConfig: `{"folder_id":"fld_1"}`},
	)
	got, err := repo.FindCoveringWorkflow(context.Background(), "ws_1", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if got.ID != "wf_folder" {
		t.Fatalf("expected folder workflow, got %q", got.ID)
	}
}

func TestFindCoveringWorkflowIgnoresDisabledWrongScopeAndWorkspace(t *testing.T) {
	repo := memory.NewWorkflowRepo()
	repo.Seed(
		repository.Workflow{ID: "wf_disabled", WorkspaceID: "ws_1", Enabled: false, TriggerType: "trigger.version_uploaded", TriggerConfig: "{}"},
		repository.Workflow{ID: "wf_wrong_trigger", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual", TriggerConfig: "{}"},
		repository.Workflow{ID: "wf_other_ws", WorkspaceID: "ws_2", Enabled: true, TriggerType: "trigger.version_uploaded", TriggerConfig: "{}"},
		repository.Workflow{ID: "wf_other_project", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.version_uploaded", TriggerConfig: `{"project_id":"prj_2"}`},
	)
	_, err := repo.FindCoveringWorkflow(context.Background(), "ws_1", "prj_1", "fld_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
