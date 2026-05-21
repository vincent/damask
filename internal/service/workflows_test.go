package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

type workflowQueueStub struct{ enqueued int }

func (q *workflowQueueStub) Register(string, queue.HandlerFunc) {}
func (q *workflowQueueStub) Start(context.Context)              {}
func (q *workflowQueueStub) Stop()                              {}

func (q *workflowQueueStub) Enqueue(_ context.Context, _, _, _ string) (dbgen.Job, error) {
	q.enqueued++
	return dbgen.Job{ID: "job_1"}, nil
}

func newWorkflowSvc(
	t *testing.T,
) (service.WorkflowService, *memory.WorkflowMemoryRepo, *memory.WorkflowRunMemoryRepo, *workflowQueueStub) {
	t.Helper()
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	queue := &workflowQueueStub{}
	svc := service.NewWorkflowService(workflows, runs, memory.NewWorkflowWebhookRepo(), queue)
	return svc, workflows, runs, queue
}

func TestWorkflowServiceGetWrongWorkspace(t *testing.T) {
	svc, repo, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_a"})
	_, err := svc.Get(context.Background(), "ws_b", "wf_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkflowServiceCreateOK(t *testing.T) {
	svc, _, _, _ := newWorkflowSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", "usr_1", service.CreateWorkflowParams{
		Name:                 "My Workflow",
		Graph:                `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
		NotifyOnFailureEmail: "ops@example.com",
	})
	if err != nil {
		t.Fatalf("Create() unexpected error: %v", err)
	}
	if !dto.Enabled {
		t.Fatal("expected new workflow to be enabled")
	}
	if dto.NotifyOnFailureEmail != "ops@example.com" {
		t.Fatalf("expected notify email to be persisted, got %q", dto.NotifyOnFailureEmail)
	}
}

func TestWorkflowServiceTriggerManualEnqueuesRun(t *testing.T) {
	svc, repo, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	if runID == "" || queue.enqueued != 1 {
		t.Fatalf("expected run to be created and enqueued, run_id=%q enqueued=%d", runID, queue.enqueued)
	}
}

func TestWorkflowServiceTemplates(t *testing.T) {
	svc, _, _, _ := newWorkflowSvc(t)
	templates := svc.Templates()
	if len(templates) < 5 {
		t.Fatalf("expected embedded templates, got %d", len(templates))
	}
	if templates[0].Graph == "" {
		t.Fatal("expected template graph payload")
	}
}

func TestWorkflowServiceFindCoveringWorkflowScope(t *testing.T) {
	svc, repo, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:            "wf_1",
		WorkspaceID:   "ws_1",
		Name:          "Folder automation",
		Enabled:       true,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: `{"folder_id":"fld_1"}`,
	})
	got, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if got == nil || got.ID != "wf_1" || got.Scope != "folder" {
		t.Fatalf("unexpected covering workflow: %#v", got)
	}
}

func TestWorkflowServiceCreateFromVariantsCreatesDisabledWorkflow(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	assets := memory.NewAssetRepo()
	variants := memory.NewRealVariantRepo()
	queue := &workflowQueueStub{}
	projectID := "prj_1"
	assets.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", ProjectID: &projectID, MimeType: "image/jpeg"})
	params := `{"width":1200}`
	variants.Seed(
		repository.Variant{ID: "var_1", WorkspaceID: "ws_1", Type: "manual"},
		repository.Variant{ID: "var_2", WorkspaceID: "ws_1", Type: "image_resize", TransformParams: &params},
	)
	svc := service.NewWorkflowServiceWithDeps(
		workflows,
		runs,
		memory.NewWorkflowWebhookRepo(),
		queue,
		service.WorkflowServiceDeps{Assets: assets, Variants: variants},
	)
	got, err := svc.CreateFromVariants(context.Background(), "ws_1", service.CreateVariantAutomationParams{
		AssetID:   "ast_1",
		CreatedBy: "usr_1",
		Scope:     service.AutomationScopeProject,
	})
	if err != nil {
		t.Fatalf("CreateFromVariants() unexpected error: %v", err)
	}
	if got.Enabled || got.TriggerType != "trigger.version_uploaded" {
		t.Fatalf("unexpected workflow: enabled=%v trigger=%q", got.Enabled, got.TriggerType)
	}
	row, err := workflows.GetByID(context.Background(), "ws_1", got.ID)
	if err != nil {
		t.Fatalf("created workflow not found: %v", err)
	}
	if row.TriggerConfig != `{"project_id":"prj_1"}` {
		t.Fatalf("trigger_config = %s", row.TriggerConfig)
	}
	if got.Graph == "" || !strings.Contains(got.Graph, `"filter.mime"`) || strings.Contains(got.Graph, `"manual"`) {
		t.Fatalf("unexpected graph: %s", got.Graph)
	}
}
