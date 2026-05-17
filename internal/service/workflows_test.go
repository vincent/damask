package service_test

import (
	"context"
	"errors"
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

func newWorkflowSvc(t *testing.T) (service.WorkflowService, *memory.WorkflowMemoryRepo, *memory.WorkflowRunMemoryRepo, *workflowQueueStub) {
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
