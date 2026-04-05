package api

import (
	"database/sql"
	"errors"

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
	return c.JSON(fiber.Map{"fields": items})
}

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

	var body struct {
		Values []fieldValueInput `json:"values"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(body.Values) == 0 {
		return errRes(c, fiber.StatusBadRequest, "values is required")
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

	resolved := make([]*resolvedValue, len(body.Values))
	for i, input := range body.Values {
		rv, err := s.validateAndResolve(c, claims.WorkspaceID, input)
		if err != nil {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		if rv != nil {
			rv.fieldID = input.FieldID
		}
		resolved[i] = rv
	}

	for i, rv := range resolved {
		if rv == nil {
			if err := s.db.DeleteProjectFieldValue(c.RequestCtx(), dbgen.DeleteProjectFieldValueParams{
				ProjectID: id,
				FieldID:   body.Values[i].FieldID,
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusInternalServerError, "could not clear field value")
			}
			continue
		}
		if _, err := s.db.UpsertProjectFieldValue(c.RequestCtx(), dbgen.UpsertProjectFieldValueParams{
			ID:           uuid.NewString(),
			ProjectID:    id,
			FieldID:      rv.fieldID,
			ValueText:    rv.valueText,
			ValueNumber:  rv.valueNumber,
			ValueDate:    rv.valueDate,
			ValueBoolean: rv.valueBoolean,
			CreatedBy:    claims.UserID,
		}); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "could not save field value")
		}
	}

	rows, err := s.db.GetProjectFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload field values")
	}
	items := make([]projectFieldValueResponse, len(rows))
	for i, row := range rows {
		items[i] = rowToProjectFieldValueResponse(row)
	}
	return c.JSON(fiber.Map{"fields": items})
}
