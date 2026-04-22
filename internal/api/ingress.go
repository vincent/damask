package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"log/slog"
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

// RawToNullableString converts a *json.RawMessage field to *string.
// Returns (nil, false) when the field was absent (pointer is nil).
// Returns (nil, true) when the field was explicitly JSON null → clear the value.
// Returns (&s, true) when the field was a JSON string → set to s.
func RawToNullableString(r *json.RawMessage) (value *string, present bool) {
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
	ID              string         `json:"id"`
	WorkspaceID     string         `json:"workspace_id"`
	CreatedBy       string         `json:"created_by"`
	Type            string         `json:"type"`
	Label           string         `json:"label"`
	PublicToken     string         `json:"public_token"`
	Config          map[string]any `json:"config"`
	DestFolderID    *string        `json:"dest_folder_id"`
	DestProjectID   *string        `json:"dest_project_id"`
	Enabled         bool           `json:"enabled"`
	PollIntervalMin int64          `json:"poll_interval_min"`
	LastPolledAt    *time.Time     `json:"last_polled_at"`
	LastError       *string        `json:"last_error"`
	ErrorCount      int64          `json:"error_count"`
	CreatedAt       time.Time      `json:"created_at"`
	UpdatedAt       time.Time      `json:"updated_at"`
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

func RedactConfig(raw map[string]any) map[string]any {
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
	configJSON, err := ingress.DecryptConfig(s.cfg.AppSecret, src.Config)
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
		PublicToken:     src.PublicToken,
		Config:          RedactConfig(configMap),
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         src.Enabled != 0,
		PollIntervalMin: src.PollIntervalMin,
		LastPolledAt:    src.LastPolledAt,
		LastError:       src.LastError,
		ErrorCount:      src.ErrorCount,
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

type ingressRuleResponse struct {
	ID       string `json:"id"`
	SourceID string `json:"source_id"`
	Position int64  `json:"position"`
	Field    string `json:"field"`
	Operator string `json:"operator"`
	Value    string `json:"value"`
	Action   string `json:"action"`
}

func ruleToResponse(r dbgen.IngressRule) ingressRuleResponse {
	return ingressRuleResponse{
		ID:       r.ID,
		SourceID: r.SourceID,
		Position: r.Position,
		Field:    r.Field,
		Operator: r.Operator,
		Value:    r.Value,
		Action:   r.Action,
	}
}

// requireSourceOwnership loads a source by id scoped to the caller's workspace.
// Returns the source or writes an error response and returns false.
func (s *Server) requireSourceOwnership(c fiber.Ctx, sourceID string) (dbgen.IngressSource, bool) {
	claims := auth.GetClaims(c)
	src, err := s.db.GetIngressSource(c.Context(), dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: claims.WorkspaceID,
	})
	if errors.Is(err, sql.ErrNoRows) {
		_ = errRes(c, fiber.StatusNotFound, "source not found")
		return dbgen.IngressSource{}, false
	}
	if err != nil {
		_ = errRes(c, fiber.StatusInternalServerError, "could not get source")
		return dbgen.IngressSource{}, false
	}
	return src, true
}

// -- Rules CRUD

// GET /api/v1/ingress/sources/:id/rules
func (s *Server) handleListIngressRules(c fiber.Ctx) error {
	if _, ok := s.requireSourceOwnership(c, c.Params("id")); !ok {
		return nil
	}
	rules, err := s.db.ListIngressRules(c.Context(), c.Params("id"))
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list rules")
	}
	result := make([]ingressRuleResponse, len(rules))
	for i, r := range rules {
		result[i] = ruleToResponse(r)
	}
	return c.JSON(result)
}

// POST /api/v1/ingress/sources/:id/rules
func (s *Server) handleCreateIngressRule(c fiber.Ctx) error {
	if _, ok := s.requireSourceOwnership(c, c.Params("id")); !ok {
		return nil
	}
	req, ok := decodeAndValidate(c, &IngressRuleReq{})
	if !ok {
		return nil
	}
	r, err := s.db.CreateIngressRule(c.Context(), dbgen.CreateIngressRuleParams{
		ID:       uuid.NewString(),
		SourceID: c.Params("id"),
		Position: req.Position,
		Field:    req.Field,
		Operator: req.Operator,
		Value:    req.Value,
		Action:   req.Action,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create rule")
	}
	return c.Status(fiber.StatusCreated).JSON(ruleToResponse(r))
}

// PUT /api/v1/ingress/sources/:id/rules/:rid
func (s *Server) handleUpdateIngressRule(c fiber.Ctx) error {
	if _, ok := s.requireSourceOwnership(c, c.Params("id")); !ok {
		return nil
	}
	rid := c.Params("rid")
	existing, err := s.db.GetIngressRule(c.Context(), rid)
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "rule not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get rule")
	}
	// Ensure the rule belongs to this source
	if existing.SourceID != c.Params("id") {
		return errRes(c, fiber.StatusNotFound, "rule not found")
	}

	var req IngressRuleReq
	if err := c.Bind().JSON(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	// Merge: keep existing values for empty fields
	field := req.Field
	if field == "" {
		field = existing.Field
	}
	operator := req.Operator
	if operator == "" {
		operator = existing.Operator
	}
	value := req.Value
	if value == "" {
		value = existing.Value
	}
	action := req.Action
	if action == "" {
		action = existing.Action
	}

	r, err := s.db.UpdateIngressRule(c.Context(), dbgen.UpdateIngressRuleParams{
		Position: req.Position,
		Field:    field,
		Operator: operator,
		Value:    value,
		Action:   action,
		ID:       rid,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update rule")
	}
	return c.JSON(ruleToResponse(r))
}

// DELETE /api/v1/ingress/sources/:id/rules/:rid
func (s *Server) handleDeleteIngressRule(c fiber.Ctx) error {
	if _, ok := s.requireSourceOwnership(c, c.Params("id")); !ok {
		return nil
	}
	rid := c.Params("rid")
	existing, err := s.db.GetIngressRule(c.Context(), rid)
	if errors.Is(err, sql.ErrNoRows) {
		return errRes(c, fiber.StatusNotFound, "rule not found")
	}
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not get rule")
	}
	if existing.SourceID != c.Params("id") {
		return errRes(c, fiber.StatusNotFound, "rule not found")
	}
	if err := s.db.DeleteIngressRule(c.Context(), rid); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete rule")
	}
	return c.SendStatus(fiber.StatusNoContent)
}

// PUT /api/v1/ingress/sources/:id/rules/reorder
func (s *Server) handleReorderIngressRules(c fiber.Ctx) error {
	if _, ok := s.requireSourceOwnership(c, c.Params("id")); !ok {
		return nil
	}
	var entries []ReorderRuleEntry
	if err := c.Bind().JSON(&entries); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	sourceID := c.Params("id")
	for _, e := range entries {
		existing, err := s.db.GetIngressRule(c.Context(), e.ID)
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "rule not found: "+e.ID)
		}
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not get rule")
		}
		if existing.SourceID != sourceID {
			return errRes(c, fiber.StatusNotFound, "rule not found: "+e.ID)
		}
		_, err = s.db.UpdateIngressRule(c.Context(), dbgen.UpdateIngressRuleParams{
			Position: e.Position,
			Field:    existing.Field,
			Operator: existing.Operator,
			Value:    existing.Value,
			Action:   existing.Action,
			ID:       e.ID,
		})
		if err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not reorder rules")
		}
	}
	// Return the updated list in position order
	rules, err := s.db.ListIngressRules(c.Context(), sourceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list rules")
	}
	result := make([]ingressRuleResponse, len(rules))
	for i, r := range rules {
		result[i] = ruleToResponse(r)
	}
	return c.JSON(result)
}

// -- Source CRUD

// handleCreateIngressSource creates a new ingress source.
//
// @Summary Create an ingress source
// @Description Creates a new ingress source for automated asset import. The <code>config</code> object is source-type-specific (see type documentation). Sensitive fields (passwords, keys, tokens) are encrypted at rest with AES-256-GCM and redacted (<code>"***"</code>) in all API responses.<br><br> Supported source types: <ul> <li><strong>imap</strong> — Poll an IMAP mailbox for attachments.</li> <li><strong>sftp</strong> — Poll a remote SFTP directory.</li> <li><strong>webdav</strong> — Poll a WebDAV endpoint.</li> <li><strong>s3</strong> — Poll an S3-compatible bucket.</li> <li><strong>email_api</strong> — Receive assets via SMTP push (uses <code>public_token</code>).</li> </ul> Optional <code>rules</code> array in the body bootstraps ingress rules for the source in a single request. Rules can also be managed later via the <code>/rules</code> sub-resource.
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateIngressSourceReq true "Source configuration"
// @Success 201 {object} ingressSourceResponse
// @Failure 400 {object} ErrorResponse "Invalid config JSON"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/ingress/sources [post]
// POST /api/v1/ingress/sources
func (s *Server) handleCreateIngressSource(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &CreateIngressSourceReq{})
	if !ok {
		return nil
	}

	interval := req.PollIntervalMin
	if interval <= 0 {
		interval = 15
	}

	// Stamp workspace_id so source constructors can use it without trusting the client to supply it.
	if req.Config == nil {
		req.Config = map[string]any{}
	}
	req.Config["workspace_id"] = claims.WorkspaceID

	mutatedConfig, err := ingress.RunOnCreateHook(req.Type, req.Config)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not prepare config")
	}
	configBytes, err := json.Marshal(mutatedConfig)
	if err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid config")
	}
	encryptedConfig, err := ingress.EncryptConfig(s.cfg.AppSecret, configBytes)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not encrypt config")
	}

	enabled := int64(1)
	if req.Enabled != nil && !*req.Enabled {
		enabled = 0
	}

	publicToken, err := ingress.GenerateToken(20)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not generate public token")
	}

	tx, err := s.sqlDB.BeginTx(c.Context(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	src, err := qtx.CreateIngressSource(c.Context(), dbgen.CreateIngressSourceParams{
		ID:              uuid.NewString(),
		WorkspaceID:     claims.WorkspaceID,
		CreatedBy:       claims.UserID,
		Type:            req.Type,
		Label:           req.Label,
		Config:          encryptedConfig,
		PublicToken:     publicToken,
		DestFolderID:    req.DestFolderID,
		DestProjectID:   req.DestProjectID,
		Enabled:         enabled,
		PollIntervalMin: interval,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create source")
	}

	for _, rule := range req.Rules {
		if _, err := qtx.CreateIngressRule(c.Context(), dbgen.CreateIngressRuleParams{
			ID:       uuid.NewString(),
			SourceID: src.ID,
			Position: rule.Position,
			Field:    rule.Field,
			Operator: rule.Operator,
			Value:    rule.Value,
			Action:   rule.Action,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not create rule")
		}
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit source creation")
	}

	if creator, err := s.db.GetUserByID(c.Context(), claims.UserID); err == nil {
		if err := s.mailer.SendIngressSourceAdded(c.Context(), creator.Email, src.Label, claims.WorkspaceID); err != nil {
			slog.ErrorContext(c.Context(), "failed to send ingress source added mail", "error", err)
		}
	}

	resp, err := s.sourceToResponse(src)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not build response")
	}
	return c.Status(fiber.StatusCreated).JSON(resp)
}

// handleListIngressSources returns all ingress sources in the workspace.
//
// @Summary List ingress sources
// @Description Returns all ingress sources in the workspace. Sensitive config fields are redacted with <code>"***"</code> in every response.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Success 200 {array} ingressSourceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/ingress/sources [get]
// GET /api/v1/ingress/sources
func (s *Server) handleListIngressSources(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	sources, err := s.db.ListIngressSources(c.Context(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list sources")
	}

	result := make([]ingressSourceResponse, 0, len(sources))
	for _, src := range sources {
		resp, err := s.sourceToResponse(src)
		if err != nil {
			continue
		}
		result = append(result, resp)
	}
	return c.JSON(result)
}

// handleGetIngressSource returns a single ingress source by ID.
//
// @Summary Get an ingress source
// @Description Returns the source record including its current enabled state and decrypted (but redacted) config. Sensitive config values are always replaced with <code>"***"</code>.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {object} ingressSourceResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Source not found"
// @Router /api/v1/ingress/sources/{id} [get]
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

// handleUpdateIngressSource updates an ingress source.
//
// @Summary Update an ingress source
// @Description Updates the source label, config, destination folder/project, enabled state, and poll interval. Omitted fields keep their current values.<br><br> To update the config, supply a full replacement object — partial config merging is not supported. Sensitive fields provided in the new config are encrypted and stored; omitting them removes them. To clear <code>dest_folder_id</code> or <code>dest_project_id</code> pass an explicit <code>null</code> value.
// @Tags Ingress
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Param body body UpdateIngressSourceReq true "Fields to update"
// @Success 200 {object} ingressSourceResponse
// @Failure 400 {object} ErrorResponse "Invalid config JSON"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Source not found"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/ingress/sources/{id} [put]
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

	req, ok := decodeAndValidate(c, &UpdateIngressSourceReq{})
	if !ok {
		return nil
	}

	interval := req.PollIntervalMin
	if interval <= 0 {
		interval = existing.PollIntervalMin
	}

	// Re-encrypt config if provided, otherwise keep existing
	encryptedConfig := existing.Config
	if req.Config != nil {
		req.Config["workspace_id"] = claims.WorkspaceID
		configBytes, err := json.Marshal(req.Config)
		if err != nil {
			return errRes(c, fiber.StatusBadRequest, "invalid config")
		}
		encryptedConfig, err = ingress.EncryptConfig(s.cfg.AppSecret, configBytes)
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
	if val, present := RawToNullableString(req.DestFolderID); present {
		destFolder = val
	}
	destProject := existing.DestProjectID
	if val, present := RawToNullableString(req.DestProjectID); present {
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

// handleDeleteIngressSource permanently deletes an ingress source.
//
// @Summary Delete an ingress source
// @Description Permanently removes the source and all its associated rules. Log entries are retained for auditing. This action is irreversible.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/ingress/sources/{id} [delete]
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

// handleTestIngressSource tests connectivity for an ingress source.
//
// @Summary Test an ingress source
// @Description Validates that the source can successfully connect to the remote system using its current configuration. The check runs with a 10-second timeout. Returns <code>{"ok": true}</code> on success, or a descriptive error message on failure.<br><br> Use this after creating or updating a source to confirm credentials and network access are correct before relying on scheduled polling.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {object} object{ok=bool}
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Source not found"
// @Failure 422 {object} ErrorResponse "Connection test failed — body contains the error message"
// @Router /api/v1/ingress/sources/{id}/test [post]
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

	configJSON, err := ingress.DecryptConfig(s.cfg.AppSecret, src.Config)
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

// handlePollIngressSource manually triggers a poll for an ingress source.
//
// @Summary Trigger a poll
// @Description Immediately enqueues an <code>ingest_poll</code> job for the source, regardless of its scheduled poll interval. Use this to force a check for new assets without waiting for the next scheduled run. Returns the queued job ID for tracking.<br><br> The job runs asynchronously; 202 Accepted means the job was enqueued, not that the poll completed. Monitor the ingress log to see results.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 202 {object} object{job_id=string}
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Source not found"
// @Router /api/v1/ingress/sources/{id}/poll [post]
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

// handleListIngressSourceLog returns ingress log entries for a specific source.
//
// @Summary List source log entries
// @Description Returns the most recent 50 ingress log entries for the given source. Each entry represents one remote item that the source discovered, with its current ingestion status (<code>pending</code>, <code>fetching</code>, <code>done</code>, <code>error</code>, <code>skipped</code>).<br><br> The <code>(source_id, remote_id)</code> pair is unique — re-encountering the same remote file on a subsequent poll is a no-op unless the log entry is deleted or retried.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param id path string true "Source ID"
// @Success 200 {array} ingressLogResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Source not found"
// @Router /api/v1/ingress/sources/{id}/log [get]
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
	if v, ok := c.Queries()["limit"]; ok && v != "" {
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

// handleListIngressLog returns workspace-wide ingress log entries.
//
// @Summary List workspace ingress log
// @Description Returns the most recent 50 ingress log entries across all sources in the workspace. Filter by status using the <code>status</code> query parameter to focus on entries that need attention (e.g. <code>error</code> or <code>skipped</code>).
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param status query string false "Filter by status (pending, fetching, done, error, skipped)"
// @Success 200 {array} ingressLogResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/ingress/log [get]
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

// handleDeleteIngressLogEntry deletes an ingress log entry.
//
// @Summary Delete a log entry
// @Description Permanently removes a log entry. Because ingress deduplication is based on <code>(source_id, remote_id)</code>, deleting an entry allows the same remote file to be re-ingested on the next poll. Use this to force a re-import of a previously processed file.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param entry_id path string true "Log entry ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Entry belongs to a different workspace"
// @Failure 404 {object} ErrorResponse "Log entry not found"
// @Router /api/v1/ingress/log/{entry_id} [delete]
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

// handleRetryIngressLogEntry retries a failed or skipped ingress log entry.
//
// @Summary Retry a log entry
// @Description Resets the log entry status to <code>pending</code> and enqueues a new <code>ingest_fetch</code> job to re-attempt asset import. Only entries with status <code>error</code> or <code>skipped</code> can be retried; attempting to retry a <code>done</code> or <code>pending</code> entry returns 400.<br><br> Returns the queued job ID. The retry runs asynchronously — poll the log entry to track completion.
// @Tags Ingress
// @Produce json
// @Security BearerAuth
// @Param entry_id path string true "Log entry ID"
// @Success 202 {object} object{job_id=string}
// @Failure 400 {object} ErrorResponse "Entry is not in error or skipped state"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 403 {object} ErrorResponse "Entry belongs to a different workspace"
// @Failure 404 {object} ErrorResponse "Log entry not found"
// @Router /api/v1/ingress/log/{entry_id}/retry [post]
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
