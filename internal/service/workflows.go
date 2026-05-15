package service

import (
	"context"
	"crypto/subtle"
	"encoding/json"
	"fmt"

	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/workflow"

	"github.com/google/uuid"
)

type workflowService struct {
	workflows repository.WorkflowRepository
	runs      repository.WorkflowRunRepository
	webhooks  repository.WorkflowWebhookRepository
	queue     queue.JobQueue
}

func NewWorkflowService(workflows repository.WorkflowRepository, runs repository.WorkflowRunRepository, webhooks repository.WorkflowWebhookRepository, queue queue.JobQueue) WorkflowService {
	return &workflowService{workflows: workflows, runs: runs, webhooks: webhooks, queue: queue}
}

func (s *workflowService) List(ctx context.Context, workspaceID string) ([]WorkflowDTO, error) {
	rows, err := s.workflows.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowDTO, len(rows))
	for i, row := range rows {
		out[i] = toWorkflowDTO(row)
	}
	return out, nil
}

func (s *workflowService) Get(ctx context.Context, workspaceID, id string) (*WorkflowDTO, error) {
	row, err := s.workflows.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) Create(ctx context.Context, workspaceID, createdBy string, p CreateWorkflowParams) (*WorkflowDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	triggerType, err := extractTriggerType(p.Graph)
	if err != nil {
		return nil, err
	}
	row, err := s.workflows.Create(ctx, repository.CreateWorkflowParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Name:        p.Name,
		Description: p.Description,
		Enabled:     true,
		TriggerType: triggerType,
		Graph:       p.Graph,
		CreatedBy:   createdBy,
	})
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) Update(ctx context.Context, workspaceID, id string, p UpdateWorkflowParams) (*WorkflowDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	update := repository.UpdateWorkflowParams{
		ID:          id,
		WorkspaceID: workspaceID,
		Name:        p.Name,
		Description: p.Description,
		Graph:       p.Graph,
	}
	if p.Graph != nil {
		triggerType, err := extractTriggerType(*p.Graph)
		if err != nil {
			return nil, err
		}
		update.TriggerType = &triggerType
	}
	row, err := s.workflows.Update(ctx, update)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowDTO(row)
	return &dto, nil
}

func (s *workflowService) SetEnabled(ctx context.Context, workspaceID, id string, enabled bool) error {
	return s.workflows.SetEnabled(ctx, workspaceID, id, enabled)
}

func (s *workflowService) Delete(ctx context.Context, workspaceID, id string) error {
	return s.workflows.Delete(ctx, workspaceID, id)
}

func (s *workflowService) TriggerManual(ctx context.Context, workspaceID, id string) (string, error) {
	wf, err := s.workflows.GetByID(ctx, workspaceID, id)
	if err != nil {
		return "", err
	}
	if !wf.Enabled {
		return "", fmt.Errorf("workflow is disabled: %w", apperr.ErrConflict)
	}
	return s.enqueueRun(ctx, wf, map[string]any{"trigger": "manual"})
}

func (s *workflowService) TriggerWebhook(ctx context.Context, id, token string, body []byte) (string, error) {
	wfs, err := s.workflows.ListByTrigger(ctx, "trigger.webhook")
	if err != nil {
		return "", err
	}
	var wf repository.Workflow
	found := false
	for _, candidate := range wfs {
		if candidate.ID == id {
			wf = candidate
			found = true
			break
		}
	}
	if !found {
		return "", apperr.ErrNotFound
	}
	tokenHash, err := s.webhooks.GetTokenHash(ctx, id)
	if err != nil {
		return "", err
	}
	if subtle.ConstantTimeCompare([]byte(workflow.Sha256Hex(token)), []byte(tokenHash)) != 1 {
		return "", fmt.Errorf("invalid webhook token: %w", apperr.ErrForbidden)
	}
	payload := map[string]any{"trigger": "webhook", "raw_body": string(body)}
	var bodyFields map[string]any
	if json.Unmarshal(body, &bodyFields) == nil {
		payload["body"] = bodyFields
	}
	return s.enqueueRun(ctx, wf, payload)
}

func (s *workflowService) GetRun(ctx context.Context, workspaceID, runID string) (*WorkflowRunDTO, error) {
	run, err := s.runs.GetByID(ctx, runID)
	if err != nil {
		return nil, err
	}
	if run.WorkspaceID != workspaceID {
		return nil, apperr.ErrNotFound
	}
	steps, err := s.runs.ListSteps(ctx, runID)
	if err != nil {
		return nil, err
	}
	dto := toWorkflowRunDTO(run, steps)
	return &dto, nil
}

func (s *workflowService) ListRuns(ctx context.Context, workflowID string, limit int, cursor string) ([]WorkflowRunDTO, error) {
	rows, err := s.runs.List(ctx, workflowID, limit, cursor)
	if err != nil {
		return nil, err
	}
	out := make([]WorkflowRunDTO, len(rows))
	for i, row := range rows {
		out[i] = toWorkflowRunDTO(row, nil)
	}
	return out, nil
}

// GetWebhookToken returns an existing plaintext token if one was just generated
// this session, or generates a fresh one. Because only the hash is persisted,
// the plaintext is only available immediately after (re)generation — callers
// that missed it must call RegenerateWebhookToken to get a new one.
func (s *workflowService) GetWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if _, err := s.workflows.GetByID(ctx, workspaceID, id); err != nil {
		return "", err
	}
	_, err := s.webhooks.GetTokenHash(ctx, id)
	if err == nil {
		// Token already exists; plaintext is not stored — caller must regenerate.
		return "", nil
	}
	if err != apperr.ErrNotFound {
		return "", err
	}
	return s.regenerateWebhookToken(ctx, id)
}

func (s *workflowService) RegenerateWebhookToken(ctx context.Context, workspaceID, id string) (string, error) {
	if _, err := s.workflows.GetByID(ctx, workspaceID, id); err != nil {
		return "", err
	}
	return s.regenerateWebhookToken(ctx, id)
}

func (s *workflowService) NodeSchemas() []WorkflowNodeSchema {
	nodes := workflow.NodeSchemas()
	out := make([]WorkflowNodeSchema, len(nodes))
	for i, node := range nodes {
		out[i] = WorkflowNodeSchema{
			Type:         node.Type,
			Label:        node.Label,
			Category:     node.Category,
			Description:  node.Description,
			Inputs:       toWorkflowNodePorts(node.Inputs),
			Outputs:      toWorkflowNodePorts(node.Outputs),
			ConfigSchema: node.ConfigSchema,
		}
	}
	return out
}

func (s *workflowService) enqueueRun(ctx context.Context, wf repository.Workflow, triggerData map[string]any) (string, error) {
	runID := uuid.NewString()
	_, err := s.runs.Create(ctx, repository.CreateWorkflowRunParams{
		ID:          runID,
		WorkflowID:  wf.ID,
		WorkspaceID: wf.WorkspaceID,
		Status:      "pending",
		TriggerData: mustJSONMap(triggerData),
		Context:     "{}",
	})
	if err != nil {
		return "", err
	}
	payload, _ := json.Marshal(workflow.RunWorkflowPayload{RunID: runID})
	if _, err := s.queue.Enqueue(ctx, wf.WorkspaceID, queue.JobTypeRunWorkflow, string(payload)); err != nil {
		return "", err
	}
	return runID, nil
}

func (s *workflowService) regenerateWebhookToken(ctx context.Context, id string) (string, error) {
	token, err := workflow.NewToken()
	if err != nil {
		return "", err
	}
	if err := s.webhooks.Upsert(ctx, id, workflow.Sha256Hex(token)); err != nil {
		return "", err
	}
	return token, nil
}

func extractTriggerType(raw string) (string, error) {
	var graph workflow.Graph
	if err := json.Unmarshal([]byte(raw), &graph); err != nil {
		return "", fmt.Errorf("graph is not valid JSON: %w", apperr.ErrInvalidInput)
	}
	if err := graph.Validate(); err != nil {
		return "", fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	trigger, err := graph.TriggerNode()
	if err != nil {
		return "", fmt.Errorf("graph: %w: %w", err, apperr.ErrInvalidInput)
	}
	return trigger.Type, nil
}

func toWorkflowDTO(row repository.Workflow) WorkflowDTO {
	return WorkflowDTO{
		ID:          row.ID,
		WorkspaceID: row.WorkspaceID,
		Name:        row.Name,
		Description: row.Description,
		Enabled:     row.Enabled,
		TriggerType: row.TriggerType,
		Graph:       row.Graph,
		LastRunAt:   row.LastRunAt,
		CreatedAt:   row.CreatedAt,
		UpdatedAt:   row.UpdatedAt,
	}
}

func toWorkflowRunDTO(run repository.WorkflowRun, steps []repository.WorkflowRunStep) WorkflowRunDTO {
	dto := WorkflowRunDTO{
		ID:          run.ID,
		WorkflowID:  run.WorkflowID,
		Status:      run.Status,
		TriggerData: parseMap(run.TriggerData),
		Error:       run.Error,
		StartedAt:   run.StartedAt,
		CompletedAt: run.CompletedAt,
		CreatedAt:   run.CreatedAt,
	}
	for _, step := range steps {
		dto.Steps = append(dto.Steps, WorkflowRunStepDTO{
			NodeID:      step.NodeID,
			NodeType:    step.NodeType,
			Status:      step.Status,
			Attempt:     step.Attempt,
			InputCtx:    parseMap(step.InputCtx),
			OutputCtx:   parseMapPtr(step.OutputCtx),
			Error:       step.Error,
			StartedAt:   step.StartedAt,
			CompletedAt: step.CompletedAt,
		})
	}
	return dto
}

func parseMap(raw string) map[string]any {
	var out map[string]any
	_ = json.Unmarshal([]byte(raw), &out)
	if out == nil {
		out = map[string]any{}
	}
	return out
}

func parseMapPtr(raw *string) map[string]any {
	if raw == nil {
		return map[string]any{}
	}
	return parseMap(*raw)
}

func mustJSONMap(v map[string]any) string {
	b, _ := json.Marshal(v)
	return string(b)
}

func toWorkflowNodePorts(in []workflow.Port) []WorkflowNodePort {
	out := make([]WorkflowNodePort, len(in))
	for i, port := range in {
		out[i] = WorkflowNodePort{ID: port.ID, Label: port.Label}
	}
	return out
}
