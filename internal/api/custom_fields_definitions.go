package api

import (
	"database/sql"
	"encoding/json"
	"errors"
	"regexp"
	"strings"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

var keyRegexp = regexp.MustCompile(`^[a-z0-9_]+$`)

const maxFieldDefinitionsPerScope = 50

type FieldDefinitionResponse struct {
	ID                 string  `json:"id"`
	WorkspaceID        string  `json:"workspace_id"`
	Scope              string  `json:"scope"`
	Name               string  `json:"name"`
	Key                string  `json:"key"`
	FieldType          string  `json:"field_type"`
	Options            *string `json:"options"`
	Required           bool    `json:"required"`
	Position           int64   `json:"position"`
	InheritFromProject bool    `json:"inherit_from_project"`
	CreatedAt          string  `json:"created_at"`
	UpdatedAt          string  `json:"updated_at"`
	DeletedAt          *string `json:"deleted_at,omitempty"`
}

type FieldDefinitionStatsResponse struct {
	AssetCount   int64 `json:"asset_count"`
	ProjectCount int64 `json:"project_count"`
}

func fieldDefToResponse(f dbgen.FieldDefinition) FieldDefinitionResponse {
	return FieldDefinitionResponse{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required != 0,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject != 0,
		CreatedAt:          f.CreatedAt,
		UpdatedAt:          f.UpdatedAt,
		DeletedAt:          f.DeletedAt,
	}
}

// handleListFieldDefinitions returns all field definitions for the workspace.
//
// @Summary List field definitions
// @Description Returns all custom field definitions for the workspace filtered by scope. Field definitions describe the schema for structured metadata that can be attached to assets (<code>scope=asset</code>) or projects (<code>scope=project</code>).<br><br> Supported field types: <code>text</code>, <code>number</code>, <code>date</code>, <code>boolean</code>, <code>select</code>, <code>url</code>.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param scope query string false "Scope filter: asset (default) or project"
// @Success 200 {array} FieldDefinitionResponse
// @Failure 400 {object} ErrorResponse "Invalid scope value"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/field-definitions [get]
func (s *Server) handleListFieldDefinitions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	scope := c.Query("scope", "asset")
	if scope != "asset" && scope != "project" {
		return errRes(c, fiber.StatusBadRequest, "scope must be 'asset' or 'project'")
	}

	defs, err := s.db.ListFieldDefinitions(c.RequestCtx(), dbgen.ListFieldDefinitionsParams{
		WorkspaceID: claims.WorkspaceID,
		Scope:       scope,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list field definitions")
	}

	items := make([]FieldDefinitionResponse, len(defs))
	for i, d := range defs {
		items[i] = fieldDefToResponse(d)
	}
	return c.JSON(items)
}

// handleCreateFieldDefinition creates a new custom field definition.
//
// @Summary Create a field definition
// @Description Creates a new custom field definition for assets or projects. The <code>key</code> must be lowercase alphanumeric with underscores and is immutable after creation. A maximum of 50 definitions per scope is enforced.<br><br> For <code>select</code> fields, supply an <code>options</code> array of string choices. The <code>inherit_from_project</code> flag (asset-scope only) causes the field to automatically pre-populate from the assigned project's value.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body CreateFieldDefinitionRequest true "Field definition"
// @Success 201 {object} FieldDefinitionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 409 {object} ErrorResponse "Key already exists in this scope"
// @Failure 422 {object} ValidationErrorResponse "Validation failed or 50-field limit reached"
// @Router /api/v1/field-definitions [post]
func (s *Server) handleCreateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &CreateFieldDefinitionRequest{})
	if !ok {
		return nil
	}

	// Clear options for non-select types
	if body.FieldType != "select" {
		body.Options = nil
	}

	// Enforce max 50 fields per (workspace, scope)
	count, err := s.db.CountFieldDefinitions(c.RequestCtx(), dbgen.CountFieldDefinitionsParams{
		WorkspaceID: claims.WorkspaceID,
		Scope:       body.Scope,
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not count field definitions")
	}
	if count >= maxFieldDefinitionsPerScope {
		return errRes(c, fiber.StatusUnprocessableEntity, "maximum of 50 field definitions per scope reached")
	}

	var required int64
	if body.Required {
		required = 1
	}
	var inheritFromProject int64
	if body.InheritFromProject {
		inheritFromProject = 1
	}

	def, err := s.db.CreateFieldDefinition(c.RequestCtx(), dbgen.CreateFieldDefinitionParams{
		ID:                 uuid.NewString(),
		WorkspaceID:        claims.WorkspaceID,
		CreatedBy:          claims.UserID,
		Scope:              body.Scope,
		Name:               body.Name,
		Key:                body.Key,
		FieldType:          body.FieldType,
		Options:            body.Options,
		Required:           required,
		Position:           body.Position,
		InheritFromProject: inheritFromProject,
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return errRes(c, fiber.StatusConflict, "a field with this key already exists in this scope")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not create field definition")
	}

	return c.Status(fiber.StatusCreated).JSON(fieldDefToResponse(def))
}

// handleGetFieldDefinition returns a single field definition by ID.
//
// @Summary Get a field definition
// @Description Returns a single custom field definition.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 200 {object} FieldDefinitionResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id} [get]
func (s *Server) handleGetFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	def, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "field definition not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load field definition")
	}

	return c.JSON(fieldDefToResponse(def))
}

// handleGetFieldDefinitionStats returns usage statistics for a field definition.
//
// @Summary Get field definition stats
// @Description Returns the number of assets and projects that have a value set for this field definition. Useful for understanding the impact of deleting a field.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 200 {object} FieldDefinitionStatsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id}/stats [get]
func (s *Server) handleGetFieldDefinitionStats(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not start transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	def, err := qtx.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "field definition not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load field definition")
	}

	assetCount, err := qtx.CountFieldDefinitionAssetValues(c.RequestCtx(), def.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not count asset values")
	}
	projectCount, err := qtx.CountFieldDefinitionProjectValues(c.RequestCtx(), def.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not count project values")
	}

	return c.JSON(FieldDefinitionStatsResponse{
		AssetCount:   assetCount,
		ProjectCount: projectCount,
	})
}

// handleUpdateFieldDefinition updates a field definition.
//
// @Summary Update a field definition
// @Description Updates a custom field definition. The <code>key</code> and <code>field_type</code> are immutable after creation — supplying a different value returns 422. All other fields are optional; omitted fields retain their current values.<br><br> For <code>select</code> fields, <code>options</code> must be a non-empty JSON array of strings.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Param body body UpdateFieldDefinitionRequest true "Fields to update"
// @Success 200 {object} FieldDefinitionResponse
// @Failure 400 {object} ErrorResponse "Invalid options format or empty name"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Failure 422 {object} ErrorResponse "Attempt to change immutable key or field_type"
// @Router /api/v1/field-definitions/{id} [put]
func (s *Server) handleUpdateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &UpdateFieldDefinitionRequest{})
	if !ok {
		return nil
	}

	existing, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "field definition not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load field definition")
	}
	if existing.DeletedAt != nil {
		return errRes(c, fiber.StatusNotFound, "field definition not found")
	}

	// key and field_type are immutable
	if body.Key != nil && *body.Key != existing.Key {
		return errRes(c, fiber.StatusUnprocessableEntity, "key cannot be changed after creation")
	}
	if body.FieldType != nil && *body.FieldType != existing.FieldType {
		return errRes(c, fiber.StatusUnprocessableEntity, "field_type cannot be changed after creation")
	}

	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			return errRes(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		body.Name = &trimmed
	}

	// Validate options if provided or if field_type is select
	if body.Options != nil && existing.FieldType == "select" {
		var opts []string
		if err := json.Unmarshal([]byte(*body.Options), &opts); err != nil || len(opts) == 0 {
			return errRes(c, fiber.StatusBadRequest, "options must be a non-empty JSON array of strings")
		}
	} else if body.Options != nil && existing.FieldType != "select" {
		return errRes(c, fiber.StatusBadRequest, "options can only be set for select fields")
	}

	var required *int64
	if body.Required != nil {
		v := int64(0)
		if *body.Required {
			v = 1
		}
		required = &v
	}
	var inheritFromProject *int64
	if body.InheritFromProject != nil {
		v := int64(0)
		if *body.InheritFromProject {
			v = 1
		}
		inheritFromProject = &v
	}

	updated, err := s.db.UpdateFieldDefinition(c.RequestCtx(), dbgen.UpdateFieldDefinitionParams{
		Name:               body.Name,
		Options:            body.Options,
		Required:           required,
		Position:           body.Position,
		InheritFromProject: inheritFromProject,
		ID:                 id,
		WorkspaceID:        claims.WorkspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "field definition not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not update field definition")
	}

	return c.JSON(fieldDefToResponse(updated))
}

// handleDeleteFieldDefinition soft-deletes a field definition.
//
// @Summary Delete a field definition
// @Description Soft-deletes the field definition. The definition is hidden from the list endpoint but existing field values on assets and projects are preserved. This action is permanent from the API perspective (no undelete endpoint).
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Field definition ID"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Field definition not found"
// @Router /api/v1/field-definitions/{id} [delete]
func (s *Server) handleDeleteFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	result, err := s.sqlDB.ExecContext(c.RequestCtx(),
		`UPDATE field_definitions SET deleted_at = datetime('now'), updated_at = datetime('now') WHERE id = ? AND workspace_id = ? AND deleted_at IS NULL`,
		id, claims.WorkspaceID,
	)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete field definition")
	}
	n, err := result.RowsAffected()
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete field definition")
	}
	if n == 0 {
		return errRes(c, fiber.StatusNotFound, "field definition not found")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// handleReorderFieldDefinitions reorders field definitions.
//
// @Summary Reorder field definitions
// @Description Updates the <code>position</code> of multiple field definitions in a single request. The request body is a JSON array of <code>{"id": "...", "position": N}</code> objects. Items not belonging to this workspace are silently skipped (best-effort update).
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body ReorderFieldDefinitionsRequest true "Reorder items"
// @Success 204
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /api/v1/field-definitions/reorder [put]
func (s *Server) handleReorderFieldDefinitions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &ReorderFieldDefinitionsRequest{})
	if !ok {
		return nil
	}

	for _, item := range body.Items {
		// Best-effort — skip items not in this workspace
		_ = s.db.UpdateFieldDefinitionPosition(c.RequestCtx(), dbgen.UpdateFieldDefinitionPositionParams{
			Position:    item.Position,
			ID:          item.ID,
			WorkspaceID: claims.WorkspaceID,
		})
	}

	return c.SendStatus(fiber.StatusNoContent)
}
