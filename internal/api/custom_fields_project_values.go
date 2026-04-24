package api

import (
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
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

func projectFieldValueDTOToResponse(dto *service.FieldValueDTO) projectFieldValueResponse {
	return projectFieldValueResponse{
		FieldID:           dto.FieldID,
		Key:               dto.FieldKey,
		Name:              dto.FieldName,
		FieldType:         dto.FieldType,
		Value:             dto.Value,
		DefinitionDeleted: dto.DefinitionDeleted,
	}
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

	dtos, err := s.projectFields.GetValues(c.RequestCtx(), claims.WorkspaceID, id)
	if err != nil {
		return Respond(c, err)
	}

	items := make([]projectFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectFieldValueDTOToResponse(d)
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

	body, ok := decodeAndValidate(c, &PatchProjectFieldsRequest{})
	if !ok {
		return nil
	}

	// Snapshot existing values for audit before/after.
	existing, _ := s.projectFields.GetValues(c.RequestCtx(), claims.WorkspaceID, id)
	existingByFieldID := make(map[string]*service.FieldValueDTO, len(existing))
	for _, v := range existing {
		v := v
		existingByFieldID[v.FieldID] = v
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	dtos, err := s.projectFields.SetValues(c.RequestCtx(), claims.WorkspaceID, id, claims.UserID, inputs)
	if err != nil {
		return Respond(c, err)
	}

	// Emit audit events (best-effort).
	userID := claims.UserID
	afterByFieldID := make(map[string]*service.FieldValueDTO, len(dtos))
	for _, v := range dtos {
		afterByFieldID[v.FieldID] = v
	}
	for _, input := range body.Values {
		before := existingByFieldID[input.FieldID]
		after := afterByFieldID[input.FieldID]
		var beforeVal, afterVal interface{}
		if before != nil {
			beforeVal = before.Value
		}
		if input.Value == nil {
			s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
				WorkspaceID: claims.WorkspaceID,
				ProjectID:   id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventProjectFieldCleared,
				Payload:     audit.ProjectFieldClearedPayload{V: 1, FieldKey: fieldKeyOf(before, after), FieldName: fieldNameOf(before, after), Before: beforeVal},
			})
		} else {
			if after != nil {
				afterVal = after.Value
			}
			s.audit.WriteProject(c.RequestCtx(), audit.ProjectEvent{
				WorkspaceID: claims.WorkspaceID,
				ProjectID:   id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventProjectFieldSet,
				Payload:     audit.ProjectFieldSetPayload{V: 1, FieldKey: fieldKeyOf(before, after), FieldName: fieldNameOf(before, after), Before: beforeVal, After: afterVal},
			})
		}
	}

	items := make([]projectFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectFieldValueDTOToResponse(d)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}
