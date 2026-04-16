package api

import (
	"database/sql"
	"errors"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type projectFieldValueResponse struct {
	FieldID           string      `json:"field_id"`
	Key               string      `json:"key"`
	Name              string      `json:"name"`
	FieldType         string      `json:"field_type"`
	Value             interface{} `json:"value"`
	DefinitionDeleted bool        `json:"definition_deleted"`
}

type GetProjectFieldsResponse struct {
	Fields []projectFieldValueResponse `json:"fields"`
}

func projectFieldRowToValue(row dbgen.GetProjectFieldValuesRow) any {
	switch row.FieldType {
	case "text", "url", "select":
		if row.ValueText != nil {
			return *row.ValueText
		}
	case "number":
		if row.ValueNumber != nil {
			return *row.ValueNumber
		}
	case "date":
		if row.ValueDate != nil {
			return *row.ValueDate
		}
	case "boolean":
		if row.ValueBoolean != nil {
			return *row.ValueBoolean != 0
		}
	}
	return nil
}

func rowToProjectFieldValueResponse(row dbgen.GetProjectFieldValuesRow) projectFieldValueResponse {
	r := projectFieldValueResponse{
		FieldID:           row.FieldID,
		Key:               row.FieldKey,
		Name:              row.FieldName,
		FieldType:         row.FieldType,
		DefinitionDeleted: row.DefinitionDeleted != 0,
	}
	switch row.FieldType {
	case "text", "url", "select":
		if row.ValueText != nil {
			r.Value = *row.ValueText
		}
	case "number":
		if row.ValueNumber != nil {
			r.Value = *row.ValueNumber
		}
	case "date":
		if row.ValueDate != nil {
			r.Value = *row.ValueDate
		}
	case "boolean":
		if row.ValueBoolean != nil {
			r.Value = *row.ValueBoolean != 0
		}
	}
	return r
}

// handleGetProjectFields returns all custom field values for a project.
//
// @Summary Get project field values
// @Description Returns all custom field values set on the project. Only fields with <code>scope=project</code> are included. The response structure mirrors the asset fields endpoint.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Success 200 {object} GetProjectFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project not found"
// @Router /api/v1/projects/{id}/fields [get]
func (s *Server) handleGetProjectFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	rows, err := s.db.GetProjectFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load field values")
	}

	items := make([]projectFieldValueResponse, len(rows))
	for i, row := range rows {
		items[i] = rowToProjectFieldValueResponse(row)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}

// handlePatchProjectFields sets or clears custom field values on a project.
//
// @Summary Update project field values
// @Description Sets or clears one or more custom field values on the project. Only fields with <code>scope=project</code> are accepted — submitting an asset-scoped field returns 422.<br><br> Pass <code>null</code> as the value to clear a field. Returns the full updated field values for the project.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Project ID"
// @Param body body PatchProjectFieldsRequest true "Field values to set"
// @Success 200 {object} GetProjectFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Project or field not found"
// @Failure 422 {object} ErrorResponse "Value validation failed or wrong scope"
// @Router /api/v1/projects/{id}/fields [patch]
func (s *Server) handlePatchProjectFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	if _, err := s.db.GetProjectByID(c.RequestCtx(), dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "project not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load project")
	}

	body, ok := decodeAndValidate(c, &PatchProjectFieldsRequest{})
	if !ok {
		return nil
	}

	// Validate all fields must be scope=project
	for _, input := range body.Values {
		def, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
			ID:          input.FieldID,
			WorkspaceID: claims.WorkspaceID,
		})
		if err != nil {
			if errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusNotFound, "field "+input.FieldID+" not found")
			}
			return errRes(c, fiber.StatusInternalServerError, "could not load field")
		}
		if def.Scope != "project" {
			return errRes(c, fiber.StatusUnprocessableEntity, "field "+def.Key+" is not a project field")
		}
	}

	// Snapshot existing values for event before/after.
	existingProjRows, _ := s.db.GetProjectFieldValues(c.RequestCtx(), id)
	existingProjByFieldID := make(map[string]dbgen.GetProjectFieldValuesRow, len(existingProjRows))
	for _, row := range existingProjRows {
		existingProjByFieldID[row.FieldID] = row
	}

	type projResolvedEntry struct {
		rv  *resolvedValue
		def dbgen.FieldDefinition
	}
	entries := make([]projResolvedEntry, len(body.Values))
	for i, input := range body.Values {
		rv, def, err := s.validateAndResolve(c, claims.WorkspaceID, input)
		if err != nil {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		entries[i] = projResolvedEntry{rv: rv, def: def}
	}

	userID := claims.UserID
	for i, e := range entries {
		input := body.Values[i]
		existing := existingProjByFieldID[input.FieldID]
		beforeVal := projectFieldRowToValue(existing)
		// Use def for key/name so brand-new fields are correct in audit.
		fieldKey := e.def.Key
		fieldName := e.def.Name
		if e.rv == nil {
			if err := s.db.DeleteProjectFieldValue(c.RequestCtx(), dbgen.DeleteProjectFieldValueParams{
				ProjectID: id,
				FieldID:   input.FieldID,
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusInternalServerError, "could not clear field value")
			}
			s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
				WorkspaceID: claims.WorkspaceID,
				ProjectID:   id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventProjectFieldCleared,
				Payload:     audit.ProjectFieldClearedPayload{V: 1, FieldKey: fieldKey, FieldName: fieldName, Before: beforeVal},
			})
			continue
		}
		if _, err := s.db.UpsertProjectFieldValue(c.RequestCtx(), dbgen.UpsertProjectFieldValueParams{
			ID:           uuid.NewString(),
			ProjectID:    id,
			FieldID:      e.rv.fieldID,
			ValueText:    e.rv.valueText,
			ValueNumber:  e.rv.valueNumber,
			ValueDate:    e.rv.valueDate,
			ValueBoolean: e.rv.valueBoolean,
			CreatedBy:    claims.UserID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not save field value")
		}
		afterVal := resolvedToValue(e.rv)
		s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
			WorkspaceID: claims.WorkspaceID,
			ProjectID:   id,
			UserID:      &userID,
			ActorType:   audit.ActorTypeUser,
			EventType:   audit.EventProjectFieldSet,
			Payload:     audit.ProjectFieldSetPayload{V: 1, FieldKey: fieldKey, FieldName: fieldName, Before: beforeVal, After: afterVal},
		})
	}

	rows, err := s.db.GetProjectFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload field values")
	}
	items := make([]projectFieldValueResponse, len(rows))
	for i, row := range rows {
		items[i] = rowToProjectFieldValueResponse(row)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}
