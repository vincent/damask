package memory

import (
	"context"
	"encoding/json"
	"sort"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

type WorkflowMemoryRepo struct {
	mu        sync.RWMutex
	workflows map[string]repository.Workflow
}

func NewWorkflowRepo() *WorkflowMemoryRepo {
	return &WorkflowMemoryRepo{workflows: map[string]repository.Workflow{}}
}

func (r *WorkflowMemoryRepo) Seed(wfs ...repository.Workflow) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, wf := range wfs {
		r.workflows[wf.ID] = wf
	}
}

func (r *WorkflowMemoryRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	wf, ok := r.workflows[id]
	if !ok || wf.WorkspaceID != workspaceID {
		return repository.Workflow{}, apperr.ErrNotFound
	}
	return wf, nil
}

func (r *WorkflowMemoryRepo) List(_ context.Context, workspaceID string) ([]repository.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.Workflow{}
	for _, wf := range r.workflows {
		if wf.WorkspaceID == workspaceID {
			out = append(out, wf)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].CreatedAt.After(out[j].CreatedAt) })
	return out, nil
}

func (r *WorkflowMemoryRepo) ListByTrigger(_ context.Context, triggerType string) ([]repository.Workflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.Workflow{}
	for _, wf := range r.workflows {
		if wf.Enabled && wf.TriggerType == triggerType {
			out = append(out, wf)
		}
	}
	return out, nil
}

func (r *WorkflowMemoryRepo) Create(_ context.Context, p repository.CreateWorkflowParams) (repository.Workflow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	now := time.Now().UTC()
	wf := repository.Workflow{
		ID:                   p.ID,
		WorkspaceID:          p.WorkspaceID,
		Name:                 p.Name,
		Description:          p.Description,
		Enabled:              p.Enabled,
		TriggerType:          p.TriggerType,
		TriggerConfig:        defaultTriggerConfig(p.TriggerConfig),
		Graph:                p.Graph,
		NotifyOnFailureEmail: p.NotifyOnFailureEmail,
		CreatedBy:            p.CreatedBy,
		CreatedAt:            now,
		UpdatedAt:            now,
	}
	r.workflows[wf.ID] = wf
	return wf, nil
}

func (r *WorkflowMemoryRepo) Update(_ context.Context, p repository.UpdateWorkflowParams) (repository.Workflow, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	wf, ok := r.workflows[p.ID]
	if !ok || wf.WorkspaceID != p.WorkspaceID {
		return repository.Workflow{}, apperr.ErrNotFound
	}
	if p.Name != nil {
		wf.Name = *p.Name
	}
	if p.Description != nil {
		wf.Description = *p.Description
	}
	if p.TriggerType != nil {
		wf.TriggerType = *p.TriggerType
	}
	if p.TriggerConfig != nil {
		wf.TriggerConfig = defaultTriggerConfig(*p.TriggerConfig)
	}
	if p.Graph != nil {
		wf.Graph = *p.Graph
	}
	if p.NotifyOnFailureEmail != nil {
		wf.NotifyOnFailureEmail = *p.NotifyOnFailureEmail
	}
	now := time.Now().UTC()
	wf.UpdatedAt = now
	r.workflows[p.ID] = wf
	return wf, nil
}

func (r *WorkflowMemoryRepo) FindCoveringWorkflow(
	_ context.Context,
	workspaceID, assetID, assetProjectID, assetFolderID string,
) (*repository.CoveringWorkflow, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	type candidate struct {
		wf    repository.Workflow
		score int
	}
	var candidates []candidate
	for _, wf := range r.workflows {
		if wf.WorkspaceID != workspaceID || wf.TriggerType != "trigger.version_uploaded" || !wf.Enabled {
			continue
		}
		var cfg struct {
			ProjectID string `json:"project_id"`
			FolderID  string `json:"folder_id"`
			AssetID   string `json:"asset_id"`
		}
		_ = json.Unmarshal([]byte(defaultTriggerConfig(wf.TriggerConfig)), &cfg)
		switch {
		case cfg.AssetID != "" && cfg.AssetID == assetID:
			candidates = append(candidates, candidate{wf: wf, score: 0})
		case cfg.FolderID != "" && cfg.FolderID == assetFolderID:
			candidates = append(candidates, candidate{wf: wf, score: 1})
		case cfg.ProjectID != "" && cfg.ProjectID == assetProjectID:
			candidates = append(candidates, candidate{wf: wf, score: 2})
		case cfg.AssetID == "" && cfg.FolderID == "" && cfg.ProjectID == "":
			candidates = append(candidates, candidate{wf: wf, score: 3})
		}
	}
	if len(candidates) == 0 {
		return nil, apperr.ErrNotFound
	}
	sort.SliceStable(candidates, func(i, j int) bool { return candidates[i].score < candidates[j].score })
	best := candidates[0].wf
	return &repository.CoveringWorkflow{
		ID:            best.ID,
		Name:          best.Name,
		TriggerType:   best.TriggerType,
		TriggerConfig: defaultTriggerConfig(best.TriggerConfig),
		Enabled:       best.Enabled,
	}, nil
}

func (r *WorkflowMemoryRepo) SetEnabled(_ context.Context, workspaceID, id string, enabled bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	wf, ok := r.workflows[id]
	if !ok || wf.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	wf.Enabled = enabled
	wf.UpdatedAt = time.Now().UTC()
	r.workflows[id] = wf
	return nil
}

func (r *WorkflowMemoryRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	wf, ok := r.workflows[id]
	if !ok || wf.WorkspaceID != workspaceID {
		return apperr.ErrNotFound
	}
	delete(r.workflows, id)
	return nil
}

func (r *WorkflowMemoryRepo) TouchLastRunAt(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	wf, ok := r.workflows[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	wf.LastRunAt = &now
	wf.UpdatedAt = now
	r.workflows[id] = wf
	return nil
}

func (r *WorkflowMemoryRepo) RunInTx(_ context.Context, fn func(repository.WorkflowRepository) error) error {
	return fn(r)
}

func defaultTriggerConfig(v string) string {
	if v == "" {
		return "{}"
	}
	return v
}

type WorkflowRunMemoryRepo struct {
	mu    sync.RWMutex
	runs  map[string]repository.WorkflowRun
	steps map[string]repository.WorkflowRunStep
}

func NewWorkflowRunRepo() *WorkflowRunMemoryRepo {
	return &WorkflowRunMemoryRepo{
		runs:  map[string]repository.WorkflowRun{},
		steps: map[string]repository.WorkflowRunStep{},
	}
}

func (r *WorkflowRunMemoryRepo) GetByID(_ context.Context, id string) (repository.WorkflowRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	run, ok := r.runs[id]
	if !ok {
		return repository.WorkflowRun{}, apperr.ErrNotFound
	}
	return run, nil
}

func (r *WorkflowRunMemoryRepo) List(
	_ context.Context,
	workflowID string,
	limit int,
	cursor string,
) ([]repository.WorkflowRun, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.WorkflowRun{}
	for _, run := range r.runs {
		if run.WorkflowID != workflowID {
			continue
		}
		if cursor != "" && run.CreatedAt.Format(time.RFC3339Nano)+"|"+run.ID >= cursor {
			continue
		}
		out = append(out, run)
	}
	sort.Slice(out, func(i, j int) bool {
		if out[i].CreatedAt.Equal(out[j].CreatedAt) {
			return out[i].ID > out[j].ID
		}
		return out[i].CreatedAt.After(out[j].CreatedAt)
	})
	if limit > 0 && len(out) > limit {
		out = out[:limit]
	}
	return out, nil
}

func (r *WorkflowRunMemoryRepo) Create(
	_ context.Context,
	p repository.CreateWorkflowRunParams,
) (repository.WorkflowRun, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	run := repository.WorkflowRun{
		ID:          p.ID,
		WorkflowID:  p.WorkflowID,
		WorkspaceID: p.WorkspaceID,
		Status:      p.Status,
		TriggerData: p.TriggerData,
		Context:     p.Context,
		Error:       p.Error,
		StartedAt:   p.StartedAt,
		CompletedAt: p.CompletedAt,
		CreatedAt:   time.Now().UTC(),
	}
	r.runs[run.ID] = run
	return run, nil
}

func (r *WorkflowRunMemoryRepo) SetStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[id]
	if !ok {
		return apperr.ErrNotFound
	}
	run.Status = status
	if status == "running" {
		now := time.Now().UTC()
		run.StartedAt = &now
	}
	r.runs[id] = run
	return nil
}

func (r *WorkflowRunMemoryRepo) SetFinal(_ context.Context, p repository.SetWorkflowRunFinalParams) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	run, ok := r.runs[p.ID]
	if !ok {
		return apperr.ErrNotFound
	}
	run.Status = p.Status
	run.Context = p.Context
	run.Error = p.Error
	if p.CompletedAt != nil {
		run.CompletedAt = p.CompletedAt
	}
	r.runs[p.ID] = run
	return nil
}

func (r *WorkflowRunMemoryRepo) ListSteps(_ context.Context, runID string) ([]repository.WorkflowRunStep, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	out := []repository.WorkflowRunStep{}
	for _, step := range r.steps {
		if step.RunID == runID {
			out = append(out, step)
		}
	}
	sort.Slice(out, func(i, j int) bool { return out[i].StartedAt.Before(*out[j].StartedAt) })
	return out, nil
}

func (r *WorkflowRunMemoryRepo) CreateStep(
	_ context.Context,
	p repository.CreateWorkflowRunStepParams,
) (repository.WorkflowRunStep, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	started := p.StartedAt
	if started == nil {
		now := time.Now().UTC()
		started = &now
	}
	step := repository.WorkflowRunStep{
		ID:          p.ID,
		RunID:       p.RunID,
		NodeID:      p.NodeID,
		NodeType:    p.NodeType,
		Status:      p.Status,
		Attempt:     p.Attempt,
		InputCtx:    p.InputCtx,
		OutputCtx:   p.OutputCtx,
		Error:       p.Error,
		StartedAt:   started,
		CompletedAt: p.CompletedAt,
	}
	r.steps[step.ID] = step
	return step, nil
}

func (r *WorkflowRunMemoryRepo) SetStepStatus(_ context.Context, id, status string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	step.Status = status
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunMemoryRepo) SetStepFailed(_ context.Context, id, errMsg string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	step.Status = "failed"
	step.Error = &errMsg
	step.CompletedAt = &now
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunMemoryRepo) SetStepCompleted(_ context.Context, id, outputCtx string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	now := time.Now().UTC()
	step.Status = "completed"
	step.OutputCtx = &outputCtx
	step.CompletedAt = &now
	r.steps[id] = step
	return nil
}

func (r *WorkflowRunMemoryRepo) IncrementStepAttempt(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	step, ok := r.steps[id]
	if !ok {
		return apperr.ErrNotFound
	}
	step.Attempt++
	r.steps[id] = step
	return nil
}

type WorkflowWebhookMemoryRepo struct {
	mu     sync.RWMutex
	tokens map[string]string
}

func NewWorkflowWebhookRepo() *WorkflowWebhookMemoryRepo {
	return &WorkflowWebhookMemoryRepo{tokens: map[string]string{}}
}

func (r *WorkflowWebhookMemoryRepo) GetTokenHash(_ context.Context, workflowID string) (string, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	tokenHash, ok := r.tokens[workflowID]
	if !ok {
		return "", apperr.ErrNotFound
	}
	return tokenHash, nil
}

func (r *WorkflowWebhookMemoryRepo) Upsert(_ context.Context, workflowID, tokenHash string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.tokens[workflowID] = tokenHash
	return nil
}

func (r *WorkflowWebhookMemoryRepo) Delete(_ context.Context, workflowID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.tokens, workflowID)
	return nil
}
