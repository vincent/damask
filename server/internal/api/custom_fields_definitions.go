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

func (s *Server) handleCreateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &createFieldDefinitionRequest{})
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
		if strings.Contains(err.Error(), "UNIQUE constraint failed") {
			return errRes(c, fiber.StatusConflict, "a field with this key already exists in this scope")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not create field definition")
	}

	return c.Status(fiber.StatusCreated).JSON(fieldDefToResponse(def))
}

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

func (s *Server) handleGetFieldDefinitionStats(c fiber.Ctx) error {
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

	assetCount, err := s.db.CountFieldDefinitionAssetValues(c.RequestCtx(), def.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not count asset values")
	}
	projectCount, err := s.db.CountFieldDefinitionProjectValues(c.RequestCtx(), def.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not count project values")
	}

	return c.JSON(fiber.Map{
		"asset_count":   assetCount,
		"project_count": projectCount,
	})
}

func (s *Server) handleUpdateFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &updateFieldDefinitionRequest{})
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

func (s *Server) handleDeleteFieldDefinition(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if _, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "field definition not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load field definition")
	}

	if err := s.db.SoftDeleteFieldDefinition(c.RequestCtx(), dbgen.SoftDeleteFieldDefinitionParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not delete field definition")
	}

	return c.SendStatus(fiber.StatusNoContent)
}

func (s *Server) handleReorderFieldDefinitions(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &reorderFieldDefinitionsRequest{})
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
