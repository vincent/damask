package api

import (
	"strings"

	"damask/server/internal/auth"
	"damask/server/internal/service"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/gofiber/fiber/v3"
)

const maxRunPageSize = 20

// workflowRequest is used for both create and update. PUT is replace-all:
// all three fields are required.
type workflowRequest struct {
	Name                 string `json:"name"`
	Description          string `json:"description"`
	Graph                string `json:"graph"`
	NotifyOnFailureEmail string `json:"notify_on_failure_email"`
}

type toggleWorkflowRequest struct {
	Enabled bool `json:"enabled"`
}

type bulkManualRunRequest struct {
	AssetIDs []string `json:"asset_ids"`
}

type bulkManualRunResponse struct {
	RunIDs []string `json:"run_ids"`
	Count  int      `json:"count"`
	Error  string   `json:"error,omitempty"`
}

func workflowToResponse(dto service.WorkflowDTO) fiber.Map {
	return fiber.Map{
		"id":                      dto.ID,
		apiWorkspaceIDKey:         dto.WorkspaceID,
		"name":                    dto.Name,
		"description":             dto.Description,
		"enabled":                 dto.Enabled,
		"trigger_type":            dto.TriggerType,
		"graph":                   dto.Graph,
		"notify_on_failure_email": dto.NotifyOnFailureEmail,
		"last_run_at":             dto.LastRunAt,
		apiCreatedAtField:         dto.CreatedAt,
		"updated_at":              dto.UpdatedAt,
	}
}

func workflowRunToResponse(dto service.WorkflowRunDTO) fiber.Map {
	steps := make([]fiber.Map, len(dto.Steps))
	for i, step := range dto.Steps {
		steps[i] = fiber.Map{
			"node_id":      step.NodeID,
			"node_type":    step.NodeType,
			apiStatusKey:   step.Status,
			"attempt":      step.Attempt,
			"input_ctx":    step.InputCtx,
			"output_ctx":   step.OutputCtx,
			apiErrorKey:    step.Error,
			"started_at":   step.StartedAt,
			"completed_at": step.CompletedAt,
		}
	}
	return fiber.Map{
		"id":              dto.ID,
		"workflow_id":     dto.WorkflowID,
		apiStatusKey:      dto.Status,
		"trigger_data":    dto.TriggerData,
		apiErrorKey:       dto.Error,
		"started_at":      dto.StartedAt,
		"completed_at":    dto.CompletedAt,
		"steps":           steps,
		apiCreatedAtField: dto.CreatedAt,
	}
}

func (s *Server) handleListWorkflows(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	params := service.ListWorkflowsParams{}
	if triggerType := c.Query("trigger_type"); triggerType != "" {
		params.TriggerType = &triggerType
	}
	params.EnabledOnly = c.Query("enabled_only") == "true"
	rows, err := s.workflows.List(c.Context(), claims.WorkspaceID, params)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]fiber.Map, len(rows))
	for i, row := range rows {
		out[i] = workflowToResponse(row)
	}
	return c.JSON(out)
}

func (s *Server) handleCreateWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	var req workflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.workflows.Create(c.Context(), claims.WorkspaceID, claims.UserID, service.CreateWorkflowParams{
		Name:                 req.Name,
		Description:          req.Description,
		Graph:                req.Graph,
		NotifyOnFailureEmail: req.NotifyOnFailureEmail,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusCreated).JSON(workflowToResponse(*dto))
}

func (s *Server) handleGetWorkflowNodeSchemas(c fiber.Ctx) error {
	return c.JSON(s.workflows.NodeSchemas())
}

func (s *Server) handleGetWorkflowTemplates(c fiber.Ctx) error {
	return c.JSON(s.workflows.Templates())
}

func (s *Server) handleGetWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	dto, err := s.workflows.Get(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(workflowToResponse(*dto))
}

func (s *Server) handleUpdateWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req workflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	dto, err := s.workflows.Update(c.Context(), claims.WorkspaceID, c.Params("id"), service.UpdateWorkflowParams{
		Name:                 &req.Name,
		Description:          &req.Description,
		Graph:                &req.Graph,
		NotifyOnFailureEmail: &req.NotifyOnFailureEmail,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(workflowToResponse(*dto))
}

func (s *Server) handleToggleWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req toggleWorkflowRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if err := s.workflows.SetEnabled(c.Context(), claims.WorkspaceID, c.Params("id"), req.Enabled); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleDeleteWorkflow(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if err := s.workflows.Delete(c.Context(), claims.WorkspaceID, c.Params("id")); err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleManualWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req struct {
		AssetID string `json:"asset_id"`
	}
	_ = c.Bind().JSON(&req)
	runID, err := s.workflows.TriggerManual(c.Context(), claims.WorkspaceID, c.Params("id"), req.AssetID)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"run_id": runID, apiStatusKey: apiStatusPending})
}

func (s *Server) handleBulkManualWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	var req bulkManualRunRequest
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	runIDs, err := s.workflows.TriggerManualBulk(c.Context(), claims.WorkspaceID, c.Params("id"), req.AssetIDs)
	if err != nil && len(runIDs) == 0 {
		return ErrorStatusResponse(c, err)
	}
	resp := bulkManualRunResponse{
		RunIDs: runIDs,
		Count:  len(runIDs),
	}
	if err != nil {
		resp.Error = err.Error()
	}
	return c.Status(fiber.StatusAccepted).JSON(resp)
}

func (s *Server) handleListAllWorkflowRuns(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	apptelemetry.EnrichSpan(c, claims.WorkspaceID, claims.UserID)
	rows, err := s.workflows.ListAllRuns(c.Context(), claims.WorkspaceID, maxRunPageSize, c.Query("cursor"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]fiber.Map, len(rows))
	for i, row := range rows {
		out[i] = workflowRunToResponse(row)
	}
	var nextCursor string
	if len(rows) == maxRunPageSize {
		last := rows[len(rows)-1]
		nextCursor = last.CreatedAt.UTC().Format("2006-01-02T15:04:05.999999999Z") + "|" + last.ID
	}
	return c.JSON(fiber.Map{"runs": out, "next_cursor": nextCursor})
}

func (s *Server) handleListWorkflowRuns(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	workflowID := c.Params("id")
	if _, err := s.workflows.Get(c.Context(), claims.WorkspaceID, workflowID); err != nil {
		return ErrorStatusResponse(c, err)
	}
	rows, err := s.workflows.ListRuns(c.Context(), workflowID, maxRunPageSize, c.Query("cursor"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	out := make([]fiber.Map, len(rows))
	for i, row := range rows {
		out[i] = workflowRunToResponse(row)
	}
	return c.JSON(out)
}

func (s *Server) handleGetWorkflowRun(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	run, err := s.workflows.GetRun(c.Context(), claims.WorkspaceID, c.Params("rid"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	if run.WorkflowID != c.Params("id") {
		return ErrorStatusResponse(c, fiber.ErrNotFound)
	}
	return c.JSON(workflowRunToResponse(*run))
}

func (s *Server) handleGetWorkflowWebhookToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	token, err := s.workflows.GetWebhookToken(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(fiber.Map{apiTokenKey: token})
}

func (s *Server) handleRegenerateWorkflowWebhookToken(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	token, err := s.workflows.RegenerateWebhookToken(c.Context(), claims.WorkspaceID, c.Params("id"))
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.JSON(fiber.Map{apiTokenKey: token})
}

const webhookBodyLimit = 512 * 1024 // 512 KB

func (s *Server) handleInboundWorkflowWebhook(c fiber.Ctx) error {
	token := strings.TrimSpace(c.Get("X-Workflow-Token"))
	if token == "" {
		authz := strings.TrimSpace(c.Get("Authorization"))
		if strings.HasPrefix(strings.ToLower(authz), "bearer ") {
			token = strings.TrimSpace(authz[7:])
		}
	}
	if token == "" {
		return errRes(c, fiber.StatusUnauthorized, "missing webhook token")
	}
	body := c.Body()
	if len(body) > webhookBodyLimit {
		return errRes(c, fiber.StatusRequestEntityTooLarge, "webhook body too large")
	}
	runID, err := s.workflows.TriggerWebhook(c.Context(), c.Params("id"), token, body)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"run_id": runID, apiStatusKey: apiStatusPending})
}
