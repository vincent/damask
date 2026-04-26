package api

import (
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

	dtos, err := s.projectFields.GetValues(c.Context(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
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

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	dtos, err := s.projectFields.SetValues(c.Context(), claims.WorkspaceID, id, claims.UserID, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	items := make([]projectFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = projectFieldValueDTOToResponse(d)
	}
	return c.JSON(GetProjectFieldsResponse{Fields: items})
}
