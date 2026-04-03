package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"strings"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/ingress"
	"damask/server/internal/queue"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// -- Request / response types

type ingressRuleReq struct {
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

type createIngressSourceReq struct {
	Type            string            `json:"type"`
	Label           string            `json:"label"`
	Config          map[string]any    `json:"config"`
	DestFolderID    *string           `json:"dest_folder_id"`
	DestProjectID   *string           `json:"dest_project_id"`
	Enabled         *bool             `json:"enabled"`
	PollIntervalMin int64             `json:"poll_interval_min"`
	Rules           []ingressRuleReq  `json:"rules"`
}

type updateIngressSourceReq struct {
	Label           string              `json:"label"`
	Config          map[string]any      `json:"config"`
	DestFolderID    *json.RawMessage    `json:"dest_folder_id"`
	DestProjectID   *json.RawMessage    `json:"dest_project_id"`
	Enabled         *bool               `json:"enabled"`
	PollIntervalMin int64               `json:"poll_interval_min"`
}

// rawToNullableString converts a *json.RawMessage field to *string.
// Returns (nil, false) when the field was absent (pointer is nil).
// Returns (nil, true) when the field was explicitly JSON null → clear the value.
// Returns (&s, true) when the field was a JSON string → set to s.
func rawToNullableString(r *json.RawMessage) (value *string, present bool) {
	if r == nil {
		return nil, false
	}
	if string(*r) == "null" {
		return nil, true
	}
	var s string
	if err := json.Unmarshal(*r, &s); err != nil {
		return nil, true // treat malformed as clear
	}
	return &s, true
}

type ingressSourceResponse struct {
	ID              string            `json:"id"`
	WorkspaceID     string            `json:"workspace_id"`
	CreatedBy       string            `json:"created_by"`
	Type            string            `json:"type"`
	Label           string            `json:"label"`
	Config          map[string]any    `json:"config"`
	DestFolderID    *string           `json:"dest_folder_id"`
	DestProjectID   *string           `json:"dest_project_id"`
	Enabled         bool              `json:"enabled"`
	PollIntervalMin int64             `json:"poll_interval_min"`
	LastPolledAt    *time.Time        `json:"last_polled_at"`
	LastError       *string           `json:"last_error"`
	CreatedAt       time.Time         `json:"created_at"`
	UpdatedAt       time.Time         `json:"updated_at"`
}

type ingressLogResponse struct {
	ID         string    `json:"id"`
	SourceID   string    `json:"source_id"`
	RemoteID   string    `json:"remote_id"`
	Filename   string    `json:"filename"`
	AssetID    *string   `json:"asset_id"`
	Status     string    `json:"status"`
	Error      *string   `json:"error"`
	ImportedAt time.Time `json:"imported_at"`
}

// -- Helpers

var sensitiveKeys = []string{"password", "secret", "key", "token"}

func redactConfig(raw map[string]any) map[string]any {
	out := make(map[string]any, len(raw))
	for k, v := range raw {
		kl := strings.ToLower(k)
		redact := false
		for _, s := range sensitiveKeys {
			if strings.Contains(kl, s) {
				redact = true
				break
			}
		}
		if redact {
			out[k] = "***"
		} else {
			out[k] = v
		}
	}
	return out
}

func (s *Server) sourceToResponse(src dbgen.IngressSource) (ingressSourceResponse, error) {
	configJSON, err := ingress.DecryptConfig(s.appSecret, src.Config)
	if err != nil {
		return ingressSourceResponse{}, err
	}
	var configMap map[string]any
	if err := json.Unmarshal(configJSON, &configMap); err != nil {
		configMap = map[string]any{}
	}
	return ingressSourceResponse{
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
		CreatedBy:       src.CreatedBy,
		Type:            src.Type,
		Label:           src.Label,
		Config:          redactConfig(configMap),
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         src.Enabled != 0,
		PollIntervalMin: src.PollIntervalMin,
		LastPolledAt:    src.LastPolledAt,
		LastError:       src.LastError,
		CreatedAt:       src.CreatedAt,
		UpdatedAt:       src.UpdatedAt,
	}, nil
}

func logEntryToResponse(e dbgen.IngressLog) ingressLogResponse {
	return ingressLogResponse{
		ID:         e.ID,
		SourceID:   e.SourceID,
		RemoteID:   e.RemoteID,
		Filename:   e.Filename,
		AssetID:    e.AssetID,
		Status:     e.Status,
		Error:      e.Error,
		ImportedAt: e.ImportedAt,
	}
}

// -- Source CRUD

// POST /api/v1/ingress/sources
func (s *Server) handleCreateIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var req createIngressSourceReq
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Type == "" {
		return errRes(c, fiber.StatusBadRequest, "type is required")
	}
	if req.Label == "" {
		return errRes(c, fiber.StatusBadRequest, "label is required")
	}

	interval := req.PollIntervalMin
	if interval <= 0 {
		interval = 15
	}

	configBytes, err := json.Marshal(req.Config)
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid config")
	}
	encryptedConfig, err := ingress.EncryptConfig(s.appSecret, configBytes)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not encrypt config")
	}

	enabled := int64(1)
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}

	src, err := s.db.CreateIngressSource(c.Context(), dbgen.CreateIngressSourceParams{
		ID:              uuid.NewString(),
		WorkspaceID:     claims.WorkspaceID,
		CreatedBy:       claims.UserID,
		Type:            req.Type,
		Label:           req.Label,
		Config:          encryptedConfig,
		DestFolderID:    req.DestFolderID,
		DestProjectID:   req.DestProjectID,
		Enabled:         enabled,
		PollIntervalMin: interval,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create source")
	}

	for _, rule := range req.Rules {
		_, err := s.db.CreateIngressRule(c.Context(), dbgen.CreateIngressRuleParams{
			ID:       uuid.NewString(),
			SourceID: src.ID,
			Position: rule.Position,
			Field:    rule.Field,
			Operator: rule.Operator,
			Value:    rule.Value,
			Action:   rule.Action,
		})
		if err != nil {
			// Non-fatal: log and continue
			_ = err
		}
	}

	resp, err := s.sourceToResponse(src)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not build response")
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// GET /api/v1/ingress/sources
func (s *Server) handleListIngressSources(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	sources, err := s.db.ListIngressSources(c.Context(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list sources")
	}

	var result []ingressSourceResponse
	for _, src := range sources {
		resp, err := s.sourceToResponse(src)
		if err != nil {
			continue
		}
		result = append(result, resp)
	}
	return c.JSON(result)
}

// GET /api/v1/ingress/sources/:id
func (s *Server) handleGetIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	src, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "source not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	resp, err := s.sourceToResponse(src)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not build response")
	}
	return c.JSON(resp)
}

// PUT /api/v1/ingress/sources/:id
func (s *Server) handleUpdateIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Load existing source to merge config
	existing, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "source not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	var req updateIngressSourceReq
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}

	interval := req.PollIntervalMin
	if interval <= 0 {
		interval = existing.PollIntervalMin
	}

	// Re-encrypt config if provided, otherwise keep existing
	encryptedConfig := existing.Config
	if req.Config != nil {
		configBytes, err := json.Marshal(req.Config)
		if err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid config")
		}
		encryptedConfig, err = ingress.EncryptConfig(s.appSecret, configBytes)
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not encrypt config")
		}
	}

	enabled := existing.Enabled
	if req.Enabled != nil {
		if *req.Enabled {
			enabled = 1
		} else {
			enabled = 0
		}
	}

	label := req.Label
	if label == "" {
		label = existing.Label
	}

	destFolder := existing.DestFolderID
	if val, present := rawToNullableString(req.DestFolderID); present {
		destFolder = val
	}
	destProject := existing.DestProjectID
	if val, present := rawToNullableString(req.DestProjectID); present {
		destProject = val
	}

	src, err := s.db.UpdateIngressSource(c.Context(), dbgen.UpdateIngressSourceParams{
		Label:           label,
		Config:          encryptedConfig,
		DestFolderID:    destFolder,
		DestProjectID:   destProject,
		Enabled:         enabled,
		PollIntervalMin: interval,
		ID:              id,
		WorkspaceID:     claims.WorkspaceID,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update source")
	}

	resp, err := s.sourceToResponse(src)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not build response")
	}
	return c.JSON(resp)
}

// DELETE /api/v1/ingress/sources/:id
func (s *Server) handleDeleteIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if err := s.db.DeleteIngressSource(c.Context(), dbgen.DeleteIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete source")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/ingress/sources/:id/test
func (s *Server) handleTestIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	src, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "source not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	configJSON, err := ingress.DecryptConfig(s.appSecret, src.Config)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not decrypt config")
	}

	source, err := ingress.Build(src.Type, configJSON)
	if err != nil {
		return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
	}

	// 10-second timeout for the test call
	ctx, cancel := context.WithTimeout(c.Context(), 10*time.Second)
	defer cancel()

	if err := source.Validate(ctx); err != nil {
		return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
	}
	return c.JSON(fiber.Map{"ok": true})
}

// POST /api/v1/ingress/sources/:id/poll
func (s *Server) handlePollIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	src, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "source not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	payload, _ := json.Marshal(ingress.PollJobPayload{
		SourceID:    src.ID,
		WorkspaceID: src.WorkspaceID,
	})
	job, err := s.queue.Enqueue(c.Context(), claims.WorkspaceID, queue.JobTypeIngestPoll, string(payload))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue poll job")
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID})
}

// -- Log API

// GET /api/v1/ingress/sources/:id/log
func (s *Server) handleListIngressSourceLog(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	// Verify source belongs to workspace
	if _, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: id, WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "source not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	limit := int64(50)
	offset := int64(0)
	if v, err := c.Queries()["limit"]; err == false && v != "" {
		// ignoring parse error, keeping default
		_ = v
	}

	entries, err := s.db.ListIngressSourceLog(c.Context(), dbgen.ListIngressSourceLogParams{
		SourceID: id,
		Limit:    limit,
		Offset:   offset,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list log")
	}

	result := make([]ingressLogResponse, len(entries))
	for i, e := range entries {
		result[i] = logEntryToResponse(e)
	}
	return c.JSON(result)
}

// GET /api/v1/ingress/log
func (s *Server) handleListIngressLog(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var statusFilter interface{}
	if s := c.Query("status"); s != "" {
		statusFilter = s
	}

	entries, err := s.db.ListWorkspaceIngressLog(c.Context(), dbgen.ListWorkspaceIngressLogParams{
		WorkspaceID: claims.WorkspaceID,
		Status:      statusFilter,
		Limit:       50,
		Offset:      0,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list log")
	}

	result := make([]ingressLogResponse, len(entries))
	for i, e := range entries {
		result[i] = logEntryToResponse(e)
	}
	return c.JSON(result)
}

// DELETE /api/v1/ingress/log/:entry_id
func (s *Server) handleDeleteIngressLogEntry(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	entryID := c.Params("entry_id")

	entry, err := s.db.GetIngressLogEntry(c.Context(), entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "log entry not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get log entry")
	}

	// Verify ownership via source → workspace
	if _, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: entry.SourceID, WorkspaceID: claims.WorkspaceID,
	}); errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusForbidden, "access denied")
	} else if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not verify ownership")
	}

	if err := s.db.DeleteIngressLogEntry(c.Context(), entryID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete log entry")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// POST /api/v1/ingress/log/:entry_id/retry
func (s *Server) handleRetryIngressLogEntry(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	entryID := c.Params("entry_id")

	entry, err := s.db.GetIngressLogEntry(c.Context(), entryID)
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "log entry not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get log entry")
	}

	// Verify ownership
	src, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: entry.SourceID, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusForbidden, "access denied")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get source")
	}

	if entry.Status != "error" && entry.Status != "skipped" {
		return errRes(c, fiber.StatusBadRequest, "only error or skipped entries can be retried")
	}

	// Reset to pending
	if err := s.db.UpdateIngressLogEntry(c.Context(), dbgen.UpdateIngressLogEntryParams{
		Status: "pending",
		ID:     entryID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reset entry")
	}

	payload, _ := json.Marshal(ingress.FetchJobPayload{
		SourceID:    src.ID,
		WorkspaceID: src.WorkspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    entry.RemoteID,
		Filename:    entry.Filename,
	})
	job, err := s.queue.Enqueue(c.Context(), claims.WorkspaceID, queue.JobTypeIngestFetch, string(payload))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not enqueue retry job")
	}
	return c.Status(fiber.StatusAccepted).JSON(fiber.Map{"job_id": job.ID})
}
