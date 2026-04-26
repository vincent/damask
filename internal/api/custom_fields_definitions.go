package api

import (
	"encoding/json"
	"regexp"
	"strings"
	"time"

	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

var keyRegexp = regexp.MustCompile(`^[a-z0-9_]+$`)

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

func fieldDTOToResponse(f *service.FieldDefinitionDTO) FieldDefinitionResponse {
	return FieldDefinitionResponse{
		ID:                 f.ID,
		WorkspaceID:        f.WorkspaceID,
		Scope:              f.Scope,
		Name:               f.Name,
		Key:                f.Key,
		FieldType:          f.FieldType,
		Options:            f.Options,
		Required:           f.Required,
		Position:           f.Position,
		InheritFromProject: f.InheritFromProject,
		CreatedAt:          f.CreatedAt.Format(time.RFC3339),
		UpdatedAt:          f.UpdatedAt.Format(time.RFC3339),
		DeletedAt:          f.DeletedAt,
	}
}

// @Summary List field definitions
// @Description Returns all custom field definitions for the workspace filtered by scope.
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

	defs, err := s.fields.List(c.Context(), claims.WorkspaceID, scope)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]FieldDefinitionResponse, len(defs))
	for i, d := range defs {
		items[i] = fieldDTOToResponse(d)
	}
	return c.JSON(items)
}

// @Summary Create a field definition
// @Description Creates a new custom field definition for assets or projects.
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

	def, err := s.fields.Create(c.Context(), claims.WorkspaceID, service.CreateFieldDefinitionParams{
		CreatedBy:          claims.UserID,
		Scope:              body.Scope,
		Name:               body.Name,
		Key:                body.Key,
		FieldType:          body.FieldType,
		Options:            body.Options,
		Required:           body.Required,
		Position:           body.Position,
		InheritFromProject: body.InheritFromProject,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.Status(fiber.StatusCreated).JSON(fieldDTOToResponse(def))
}

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

	def, err := s.fields.Get(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(fieldDTOToResponse(def))
}

// @Summary Get field definition stats
// @Description Returns the number of assets and projects that have a value set for this field definition.
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

	stats, err := s.fields.GetStats(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(FieldDefinitionStatsResponse{
		AssetCount:   stats.AssetCount,
		ProjectCount: stats.ProjectCount,
	})
}

// @Summary Update a field definition
// @Description Updates a custom field definition. The key and field_type are immutable after creation.
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

	// Validate options format if provided
	if body.Options != nil {
		existing, err := s.fields.Get(c.Context(), claims.WorkspaceID, id)
		if err != nil {
			return ErrorStatusResponse(c, err)
		}
		if existing.FieldType == "select" {
			var opts []string
			if err := json.Unmarshal([]byte(*body.Options), &opts); err != nil || len(opts) == 0 {
				return errRes(c, fiber.StatusBadRequest, "options must be a non-empty JSON array of strings")
			}
		} else {
			return errRes(c, fiber.StatusBadRequest, "options can only be set for select fields")
		}
	}

	if body.Name != nil {
		trimmed := strings.TrimSpace(*body.Name)
		if trimmed == "" {
			return errRes(c, fiber.StatusBadRequest, "name cannot be empty")
		}
		body.Name = &trimmed
	}

	updated, err := s.fields.Update(c.Context(), claims.WorkspaceID, id, service.UpdateFieldDefinitionParams{
		Name:               body.Name,
		Key:                body.Key,
		FieldType:          body.FieldType,
		Options:            body.Options,
		Required:           body.Required,
		Position:           body.Position,
		InheritFromProject: body.InheritFromProject,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(fieldDTOToResponse(updated))
}

// @Summary Delete a field definition
// @Description Soft-deletes the field definition.
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

	if err := s.fields.Delete(c.Context(), claims.WorkspaceID, id); err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.SendStatus(fiber.StatusNoContent)
}

// @Summary Reorder field definitions
// @Description Updates the position of multiple field definitions in a single request.
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

	items := make([]service.ReorderFieldItem, len(body.Items))
	for i, item := range body.Items {
		items[i] = service.ReorderFieldItem{ID: item.ID, Position: item.Position}
	}
	_ = s.fields.Reorder(c.Context(), claims.WorkspaceID, items)

	return c.SendStatus(fiber.StatusNoContent)
}
