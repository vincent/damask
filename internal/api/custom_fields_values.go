package api

import (
	"context"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// -- Asset field values response ----------------------------------------------

type assetFieldValueResponse struct {
	FieldID           string      `json:"field_id"`
	Key               string      `json:"key"`
	Name              string      `json:"name"`
	FieldType         string      `json:"field_type"`
	Value             interface{} `json:"value"`
	DefinitionDeleted bool        `json:"definition_deleted"`
}

type GetAssetFieldsResponse struct {
	Fields []assetFieldValueResponse `json:"fields"`
}

type BulkPatchAssetFieldsResponse struct {
	Updated int64 `json:"updated"`
}

func fieldValueDTOToResponse(dto *service.FieldValueDTO) assetFieldValueResponse {
	return assetFieldValueResponse{
		FieldID:           dto.FieldID,
		Key:               dto.FieldKey,
		Name:              dto.FieldName,
		FieldType:         dto.FieldType,
		Value:             dto.Value,
		DefinitionDeleted: dto.DefinitionDeleted,
	}
}

func fieldValueDTOsToResponse(dtos []*service.FieldValueDTO) []assetFieldValueResponse {
	items := make([]assetFieldValueResponse, len(dtos))
	for i, d := range dtos {
		items[i] = fieldValueDTOToResponse(d)
	}
	return items
}

// handleGetAssetFields returns all custom field values for an asset.
//
// @Summary Get asset field values
// @Description Returns all custom field values set on the asset, including fields for which no value has been set yet (those have <code>value: null</code>). Each entry includes the field definition metadata (<code>key</code>, <code>name</code>, <code>field_type</code>) alongside the typed value. The <code>definition_deleted</code> flag indicates the field definition was soft-deleted — the value is still stored but the field is no longer active.
// @Tags Custom Fields
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Success 200 {object} GetAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Router /api/v1/assets/{id}/fields [get]
func (s *Server) handleGetAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	dtos, err := s.assetFields.GetValues(c.RequestCtx(), claims.WorkspaceID, id)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	return c.JSON(GetAssetFieldsResponse{Fields: fieldValueDTOsToResponse(dtos)})
}

// handlePatchAssetFields sets or clears custom field values on an asset.
//
// @Summary Update asset field values
// @Description Sets or clears one or more custom field values on the asset. Each entry in the <code>values</code> array targets a field by <code>field_id</code>. To clear a value, pass <code>null</code> as the value. All values are validated before any writes.<br><br> Value types must match the field definition's <code>field_type</code>: <ul> <li><strong>text / url</strong> — string</li> <li><strong>number</strong> — number</li> <li><strong>date</strong> — string in <code>YYYY-MM-DD</code> format</li> <li><strong>boolean</strong> — boolean</li> <li><strong>select</strong> — string matching one of the field's defined options</li> </ul> Returns the full updated field values for the asset.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param id path string true "Asset ID"
// @Param body body PatchAssetFieldsRequest true "Field values to set"
// @Success 200 {object} GetAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 404 {object} ErrorResponse "Asset not found"
// @Failure 422 {object} ErrorResponse "Value validation failed"
// @Router /api/v1/assets/{id}/fields [patch]
func (s *Server) handlePatchAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	id := c.Params("id")

	body, ok := decodeAndValidate(c, &PatchAssetFieldsRequest{})
	if !ok {
		return nil
	}

	// Snapshot existing values for audit before/after.
	existing, _ := s.assetFields.GetValues(c.RequestCtx(), claims.WorkspaceID, id)
	existingByFieldID := make(map[string]*service.FieldValueDTO, len(existing))
	for _, v := range existing {
		v := v
		existingByFieldID[v.FieldID] = v
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	dtos, err := s.assetFields.SetValues(c.RequestCtx(), claims.WorkspaceID, id, claims.UserID, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
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
			s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
				WorkspaceID: claims.WorkspaceID,
				AssetID:     id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventAssetFieldCleared,
				Payload:     audit.AssetFieldClearedPayload{V: 1, FieldKey: fieldKeyOf(before, after), FieldName: fieldNameOf(before, after), Before: beforeVal},
			})
		} else {
			if after != nil {
				afterVal = after.Value
			}
			s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
				WorkspaceID: claims.WorkspaceID,
				AssetID:     id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventAssetFieldSet,
				Payload:     audit.AssetFieldSetPayload{V: 1, FieldKey: fieldKeyOf(before, after), FieldName: fieldNameOf(before, after), Before: beforeVal, After: afterVal},
			})
		}
	}

	go func() { _ = s.assets.RefreshFTS(context.Background(), id) }()

	return c.JSON(GetAssetFieldsResponse{Fields: fieldValueDTOsToResponse(dtos)})
}

// handleBulkPatchAssetFields applies the same field values to multiple assets.
//
// @Summary Bulk update asset field values
// @Description Applies the same set of field value changes to multiple assets in a single transaction. Assets not belonging to the workspace are silently skipped. Returns the count of assets successfully updated.<br><br> Supports the same value format and null-for-clear semantics as the single-asset PATCH endpoint.
// @Tags Custom Fields
// @Accept json
// @Produce json
// @Security BearerAuth
// @Param body body BulkPatchAssetFieldsRequest true "Asset IDs and field values"
// @Success 200 {object} BulkPatchAssetFieldsResponse
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Failure 422 {object} ErrorResponse "Value validation failed"
// @Router /api/v1/assets/bulk/fields [patch]
func (s *Server) handleBulkPatchAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &BulkPatchAssetFieldsRequest{})
	if !ok {
		return nil
	}

	inputs := make([]service.SetFieldValueInput, len(body.Values))
	for i, v := range body.Values {
		inputs[i] = service.SetFieldValueInput{FieldID: v.FieldID, Value: v.Value}
	}

	updated, err := s.assetFields.BulkSetValues(c.RequestCtx(), claims.WorkspaceID, claims.UserID, body.AssetIDs, inputs)
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	assetIDsCopy := make([]string, len(body.AssetIDs))
	copy(assetIDsCopy, body.AssetIDs)
	go func() {
		for _, assetID := range assetIDsCopy {
			_ = s.assets.RefreshFTS(context.Background(), assetID)
		}
	}()

	return c.JSON(BulkPatchAssetFieldsResponse{Updated: updated})
}

// FieldValueInput is the unexported alias for backward compatibility.
type FieldValueInput struct {
	FieldID string      `json:"field_id"`
	Value   interface{} `json:"value"`
}

func fieldKeyOf(before, after *service.FieldValueDTO) string {
	if after != nil {
		return after.FieldKey
	}
	if before != nil {
		return before.FieldKey
	}
	return ""
}

func fieldNameOf(before, after *service.FieldValueDTO) string {
	if after != nil {
		return after.FieldName
	}
	if before != nil {
		return before.FieldName
	}
	return ""
}
