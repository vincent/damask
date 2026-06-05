package service_test

import (
	"context"
	"encoding/json"
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

type workflowQueueStub struct {
	enqueued int
	types    []string
}

func (q *workflowQueueStub) Register(string, queue.HandlerFunc) {}
func (q *workflowQueueStub) Start(context.Context)              {}
func (q *workflowQueueStub) Stop()                              {}

func (q *workflowQueueStub) Enqueue(_ context.Context, _, jobType, _ string) (dbgen.Job, error) {
	q.enqueued++
	q.types = append(q.types, jobType)
	return dbgen.Job{ID: "job_1"}, nil
}

func newWorkflowSvc(
	t *testing.T,
) (service.WorkflowService, *memory.WorkflowMemoryRepo, *memory.WorkflowRunMemoryRepo, *workflowQueueStub) {
	t.Helper()
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	queue := &workflowQueueStub{}
	svc := service.NewWorkflowService(
		workflows,
		runs,
		memory.NewWorkflowWebhookRepo(),
		queue,
		service.WorkflowServiceDeps{},
	)
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
	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	if runID == "" || queue.enqueued != 1 {
		t.Fatalf("expected run to be created and enqueued, run_id=%q enqueued=%d", runID, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManual_NoAsset_TriggerDataIsManualOnly(t *testing.T) {
	svc, repo, runs, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	run, err := runs.GetByID(context.Background(), runID)
	if err != nil {
		t.Fatalf("run not found: %v", err)
	}
	var td map[string]any
	if err := json.Unmarshal([]byte(run.TriggerData), &td); err != nil {
		t.Fatalf("invalid trigger_data JSON: %v", err)
	}
	if td["trigger"] != "manual" {
		t.Fatalf("trigger: got %v, want manual", td["trigger"])
	}
	if _, ok := td["asset_id"]; ok {
		t.Fatal("expected no asset_id in trigger_data when assetID is empty")
	}
}

func TestWorkflowServiceTriggerManual_WithAsset_TriggerDataHasAssetContext(t *testing.T) {
	svc, repo, runs, assets, versions, _ := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	assets.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
		MimeType:         "image/jpeg",
		Size:             512,
	})
	versions.Seed(repository.AssetVersion{
		ID:          "ver_1",
		AssetID:     "ast_1",
		WorkspaceID: "ws_1",
		VersionNum:  1,
		StorageKey:  "ws_1/ast_1/v1.jpg",
		IsCurrent:   true,
	})

	runID, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "ast_1")
	if err != nil {
		t.Fatalf("TriggerManual() unexpected error: %v", err)
	}
	run, err := runs.GetByID(context.Background(), runID)
	if err != nil {
		t.Fatalf("run not found: %v", err)
	}
	var td map[string]any
	if err := json.Unmarshal([]byte(run.TriggerData), &td); err != nil {
		t.Fatalf("invalid trigger_data JSON: %v", err)
	}
	checks := map[string]any{
		"asset_id":   "ast_1",
		"version_id": "ver_1",
		"mime_type":  "image/jpeg",
		"project_id": "",
		"folder_id":  "",
	}
	for key, want := range checks {
		got, ok := td[key]
		if !ok {
			t.Errorf("trigger_data missing key %q", key)
			continue
		}
		if got != want {
			t.Errorf("trigger_data[%q] = %v, want %v", key, got, want)
		}
	}
}

func TestWorkflowServiceTriggerManual_UnknownAsset_ReturnsNotFound(t *testing.T) {
	svc, repo, _, assets, _, _ := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{
		ID:          "wf_1",
		WorkspaceID: "ws_1",
		Enabled:     true,
		TriggerType: "trigger.manual",
		Graph:       `{"nodes":[{"id":"n1","type":"trigger.manual","config":{},"position":{"x":0,"y":0}}],"edges":[]}`,
	})
	// asset exists in a different workspace
	assets.Seed(repository.Asset{ID: "ast_other", WorkspaceID: "ws_other", OriginalFilename: "x.jpg"})
	_, err := svc.TriggerManual(context.Background(), "ws_1", "wf_1", "ast_other")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkflowServiceListFilterManualEnabled(t *testing.T) {
	svc, repo, _, _ := newWorkflowSvc(t)
	repo.Seed(
		repository.Workflow{
			ID:          "wf_1",
			WorkspaceID: "ws_1",
			Name:        "Manual",
			Enabled:     true,
			TriggerType: "trigger.manual",
		},
		repository.Workflow{
			ID:          "wf_2",
			WorkspaceID: "ws_1",
			Name:        "Disabled",
			Enabled:     false,
			TriggerType: "trigger.manual",
		},
		repository.Workflow{
			ID:          "wf_3",
			WorkspaceID: "ws_1",
			Name:        "Asset",
			Enabled:     true,
			TriggerType: "trigger.asset_created",
		},
		repository.Workflow{
			ID:          "wf_4",
			WorkspaceID: "ws_2",
			Name:        "Other workspace",
			Enabled:     true,
			TriggerType: "trigger.manual",
		},
	)
	triggerType := "trigger.manual"
	got, err := svc.List(context.Background(), "ws_1", service.ListWorkflowsParams{
		TriggerType: &triggerType,
		EnabledOnly: true,
	})
	if err != nil {
		t.Fatalf("List() unexpected error: %v", err)
	}
	if len(got) != 1 || got[0].ID != "wf_1" {
		t.Fatalf("unexpected workflows: %#v", got)
	}
}

func newWorkflowSvcWithAssets(t *testing.T) (
	service.WorkflowService,
	*memory.WorkflowMemoryRepo,
	*memory.WorkflowRunMemoryRepo,
	*memory.AssetRepo,
	*memory.RealVersionRepo,
	*workflowQueueStub,
) {
	t.Helper()
	workflows := memory.NewWorkflowRepo()
	runs := memory.NewWorkflowRunRepo()
	assets := memory.NewAssetRepo()
	versions := memory.NewRealVersionRepo()
	q := &workflowQueueStub{}
	svc := service.NewWorkflowService(
		workflows, runs, memory.NewWorkflowWebhookRepo(), q,
		service.WorkflowServiceDeps{Assets: assets, Versions: versions},
	)
	return svc, workflows, runs, assets, versions, q
}

func TestWorkflowServiceTriggerManualBulkOK(t *testing.T) {
	svc, repo, runs, assets, versions, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{
			ID:               "ast_1",
			WorkspaceID:      "ws_1",
			OriginalFilename: "foo.jpg",
			MimeType:         "image/jpeg",
			Size:             1024,
		},
		repository.Asset{
			ID:               "ast_2",
			WorkspaceID:      "ws_1",
			OriginalFilename: "bar.png",
			MimeType:         "image/png",
			Size:             2048,
		},
		repository.Asset{
			ID:               "ast_3",
			WorkspaceID:      "ws_1",
			OriginalFilename: "baz.mp4",
			MimeType:         "video/mp4",
			Size:             4096,
		},
	)
	versions.Seed(
		repository.AssetVersion{
			ID:          "ver_1",
			AssetID:     "ast_1",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_1/v1.jpg",
			IsCurrent:   true,
		},
		repository.AssetVersion{
			ID:          "ver_2",
			AssetID:     "ast_2",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_2/v1.png",
			IsCurrent:   true,
		},
		repository.AssetVersion{
			ID:          "ver_3",
			AssetID:     "ast_3",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "ws_1/ast_3/v1.mp4",
			IsCurrent:   true,
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1", "ast_2", "ast_3"})
	if err != nil {
		t.Fatalf("TriggerManualBulk() unexpected error: %v", err)
	}
	if len(runIDs) != 3 || q.enqueued != 3 {
		t.Fatalf("expected 3 runs and jobs, runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
	for _, typ := range q.types {
		if typ != queue.JobTypeRunWorkflow {
			t.Fatalf("unexpected job type %q", typ)
		}
	}
	type expectation struct {
		assetID    string
		mimeType   string
		filename   string
		size       float64
		versionID  string
		versionNum float64
		storageKey string
	}
	expectations := []expectation{
		{"ast_1", "image/jpeg", "foo.jpg", 1024, "ver_1", 1, "ws_1/ast_1/v1.jpg"},
		{"ast_2", "image/png", "bar.png", 2048, "ver_2", 1, "ws_1/ast_2/v1.png"},
		{"ast_3", "video/mp4", "baz.mp4", 4096, "ver_3", 1, "ws_1/ast_3/v1.mp4"},
	}
	for i, runID := range runIDs {
		run, err := runs.GetByID(context.Background(), runID)
		if err != nil {
			t.Fatalf("run %q not persisted: %v", runID, err)
		}
		var td map[string]any
		if err := json.Unmarshal([]byte(run.TriggerData), &td); err != nil {
			t.Fatalf("run %d trigger_data invalid JSON: %v", i, err)
		}
		ex := expectations[i]
		// strings.Contains still used to keep "strings" import valid
		if !strings.Contains(run.TriggerData, ex.assetID) {
			t.Fatalf("run %d trigger_data missing asset_id %q: %s", i, ex.assetID, run.TriggerData)
		}
		checks := map[string]any{
			"asset_id":          ex.assetID,
			"workspace_id":      "ws_1",
			"mime_type":         ex.mimeType,
			"original_filename": ex.filename,
			"filename":          ex.filename,
			"size":              ex.size,
			"version_id":        ex.versionID,
			"version_num":       ex.versionNum,
			"storage_key":       ex.storageKey,
		}
		for key, want := range checks {
			got, ok := td[key]
			if !ok {
				t.Errorf("run %d: trigger_data missing key %q", i, key)
				continue
			}
			if got != want {
				t.Errorf("run %d: trigger_data[%q] = %v (%T), want %v (%T)", i, key, got, got, want, want)
			}
		}
	}
}

func TestWorkflowServiceTriggerManualBulkAssetNotFound(t *testing.T) {
	svc, repo, _, assets, _, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(repository.Asset{ID: "ast_alien", WorkspaceID: "ws_other", OriginalFilename: "x.jpg"})
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_alien"})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
	if len(runIDs) != 0 || q.enqueued != 0 {
		t.Fatalf("expected no runs, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkNoCurrentVersion(t *testing.T) {
	svc, repo, _, assets, _, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{
			ID:               "ast_noversion",
			WorkspaceID:      "ws_1",
			OriginalFilename: "ghost.jpg",
			MimeType:         "image/jpeg",
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_noversion"})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if len(runIDs) != 0 || q.enqueued != 0 {
		t.Fatalf("expected no runs, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkPartialSuccess(t *testing.T) {
	svc, repo, _, assets, versions, q := newWorkflowSvcWithAssets(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.manual"})
	assets.Seed(
		repository.Asset{ID: "ast_ok", WorkspaceID: "ws_1", OriginalFilename: "ok.jpg", MimeType: "image/jpeg"},
		repository.Asset{ID: "ast_bad", WorkspaceID: "ws_1", OriginalFilename: "bad.jpg", MimeType: "image/jpeg"},
	)
	versions.Seed(
		repository.AssetVersion{
			ID:          "ver_ok",
			AssetID:     "ast_ok",
			WorkspaceID: "ws_1",
			VersionNum:  1,
			StorageKey:  "k/ok.jpg",
			IsCurrent:   true,
		},
	)
	runIDs, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_ok", "ast_bad"})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
	if len(runIDs) != 1 || q.enqueued != 1 {
		t.Fatalf("expected 1 partial run, got runIDs=%v enqueued=%d", runIDs, q.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkDisabledWorkflow(t *testing.T) {
	svc, repo, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: false, TriggerType: "trigger.manual"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrConflict) || queue.enqueued != 0 {
		t.Fatalf("expected ErrConflict and empty queue, err=%v enqueued=%d", err, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkWrongTriggerType(t *testing.T) {
	svc, repo, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_1", Enabled: true, TriggerType: "trigger.asset_created"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrConflict) || queue.enqueued != 0 {
		t.Fatalf("expected ErrConflict and empty queue, err=%v enqueued=%d", err, queue.enqueued)
	}
}

func TestWorkflowServiceTriggerManualBulkEmptyAssetIDs(t *testing.T) {
	svc, _, _, _ := newWorkflowSvc(t)
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", nil)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkflowServiceTriggerManualBulkTooManyAssets(t *testing.T) {
	svc, _, _, _ := newWorkflowSvc(t)
	assetIDs := make([]string, 501)
	_, err := svc.TriggerManualBulk(context.Background(), "ws_1", "wf_1", assetIDs)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestWorkflowServiceTriggerManualBulkWorkspaceIsolation(t *testing.T) {
	svc, repo, _, queue := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{ID: "wf_1", WorkspaceID: "ws_a", Enabled: true, TriggerType: "trigger.manual"})
	_, err := svc.TriggerManualBulk(context.Background(), "ws_b", "wf_1", []string{"ast_1"})
	if !errors.Is(err, apperr.ErrNotFound) || queue.enqueued != 0 {
		t.Fatalf("expected ErrNotFound and empty queue, err=%v enqueued=%d", err, queue.enqueued)
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
	got, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "", "prj_1", "fld_1")
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
	svc := service.NewWorkflowService(
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

func TestWorkflowServiceCreateFromVariantsAssetScopeTriggerConfig(t *testing.T) {
	workflows := memory.NewWorkflowRepo()
	assets := memory.NewAssetRepo()
	variants := memory.NewRealVariantRepo()
	projectID := "prj_1"
	assets.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", ProjectID: &projectID, MimeType: "image/jpeg"})
	variants.Seed(repository.Variant{ID: "var_1", WorkspaceID: "ws_1", Type: "image_resize"})
	svc := service.NewWorkflowService(
		workflows,
		memory.NewWorkflowRunRepo(),
		memory.NewWorkflowWebhookRepo(),
		&workflowQueueStub{},
		service.WorkflowServiceDeps{Assets: assets, Variants: variants},
	)
	got, err := svc.CreateFromVariants(context.Background(), "ws_1", service.CreateVariantAutomationParams{
		AssetID:   "ast_1",
		CreatedBy: "usr_1",
		Scope:     service.AutomationScopeAsset,
	})
	if err != nil {
		t.Fatalf("CreateFromVariants() unexpected error: %v", err)
	}
	row, err := workflows.GetByID(context.Background(), "ws_1", got.ID)
	if err != nil {
		t.Fatalf("created workflow not found: %v", err)
	}
	if row.TriggerConfig != `{"asset_id":"ast_1"}` {
		t.Fatalf("trigger_config = %s, want {\"asset_id\":\"ast_1\"}", row.TriggerConfig)
	}
}

func TestWorkflowServiceFindCoveringWorkflowAssetScope(t *testing.T) {
	svc, repo, _, _ := newWorkflowSvc(t)
	repo.Seed(repository.Workflow{
		ID:            "wf_asset",
		WorkspaceID:   "ws_1",
		Name:          "Asset automation",
		Enabled:       true,
		TriggerType:   "trigger.version_uploaded",
		TriggerConfig: `{"asset_id":"ast_1"}`,
	})
	got, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "ast_1", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if got == nil || got.ID != "wf_asset" || got.Scope != "asset" {
		t.Fatalf("unexpected covering workflow: %#v", got)
	}
	// must not match a different asset
	none, err := svc.FindCoveringWorkflow(context.Background(), "ws_1", "ast_2", "prj_1", "fld_1")
	if err != nil {
		t.Fatalf("FindCoveringWorkflow() unexpected error: %v", err)
	}
	if none != nil {
		t.Fatalf("expected no workflow for different asset, got %#v", none)
	}
}
