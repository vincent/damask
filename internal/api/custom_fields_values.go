package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// -- Shared value validation & typed columns ---------------------------------

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type FieldValueInput struct {
	FieldID string      `json:"field_id"`
	Value   interface{} `json:"value"`
}

// fieldValueInput is the unexported alias for backward compatibility
type fieldValueInput = FieldValueInput

type resolvedValue struct {
	fieldID      string
	valueText    *string
	valueNumber  *float64
	valueDate    *string
	valueBoolean *int64
}

// validateAndResolve loads the field definition and validates the incoming value.
// Returns (nil, nil) when input.Value is nil — the caller must distinguish this
// "explicit clear" sentinel from an error (err != nil) before writing.
func (s *Server) validateAndResolve(c fiber.Ctx, workspaceID string, input fieldValueInput) (*resolvedValue, dbgen.FieldDefinition, error) {
	def, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          input.FieldID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, def, fmt.Errorf("field %s not found", input.FieldID)
		}
		return nil, def, fmt.Errorf("could not load field %s", input.FieldID)
	}
	if def.DeletedAt != nil {
		return nil, def, fmt.Errorf("field %s has been deleted", input.FieldID)
	}

	// null value → explicit clear; caller uses (nil, def, nil) to know key/name
	if input.Value == nil {
		return nil, def, nil //nolint:nilnil
	}

	rv := &resolvedValue{fieldID: input.FieldID}

	switch def.FieldType {
	case "text", "url":
		s, ok := input.Value.(string)
		if !ok {
			return nil, def, fmt.Errorf("field %s expects a string value", def.Key)
		}
		rv.valueText = &s

	case "select":
		s, ok := input.Value.(string)
		if !ok {
			return nil, def, fmt.Errorf("field %s expects a string value", def.Key)
		}
		// validate against options
		if def.Options != nil {
			var opts []string
			if err := json.Unmarshal([]byte(*def.Options), &opts); err == nil {
				valid := false
				for _, o := range opts {
					if o == s {
						valid = true
						break
					}
				}
				if !valid {
					return nil, def, fmt.Errorf("value '%s' is not a valid option for field %s", s, def.Key)
				}
			}
		}
		rv.valueText = &s

	case "number":
		switch v := input.Value.(type) {
		case float64:
			rv.valueNumber = &v
		case json.Number:
			f, err := v.Float64()
			if err != nil {
				return nil, def, fmt.Errorf("field %s expects a numeric value", def.Key)
			}
			rv.valueNumber = &f
		default:
			return nil, def, fmt.Errorf("field %s expects a numeric value", def.Key)
		}

	case "date":
		s, ok := input.Value.(string)
		if !ok || !dateRe.MatchString(s) {
			return nil, def, fmt.Errorf("field %s expects a date in YYYY-MM-DD format", def.Key)
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, def, fmt.Errorf("field %s: invalid date '%s'", def.Key, s)
		}
		rv.valueDate = &s

	case "boolean":
		b, ok := input.Value.(bool)
		if !ok {
			return nil, def, fmt.Errorf("field %s expects a boolean value", def.Key)
		}
		var v int64
		if b {
			v = 1
		}
		rv.valueBoolean = &v
	}

	return rv, def, nil
}

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

func rowToAssetFieldValueResponse(row dbgen.GetAssetFieldValuesRow) assetFieldValueResponse {
	r := assetFieldValueResponse{
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

// fieldRowToValue extracts the typed value from a field row for event payloads.
func fieldRowToValue(row dbgen.GetAssetFieldValuesRow) any {
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

// resolvedToValue extracts the typed value from a resolvedValue for event payloads.
func resolvedToValue(rv *resolvedValue) any {
	if rv == nil {
		return nil
	}
	if rv.valueText != nil {
		return *rv.valueText
	}
	if rv.valueNumber != nil {
		return *rv.valueNumber
	}
	if rv.valueDate != nil {
		return *rv.valueDate
	}
	if rv.valueBoolean != nil {
		return *rv.valueBoolean != 0
	}
	return nil
}

// -- Handlers -----------------------------------------------------------------

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

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	rows, err := s.db.GetAssetFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load field values")
	}

	items := make([]assetFieldValueResponse, len(rows))
	for i, row := range rows {
		items[i] = rowToAssetFieldValueResponse(row)
	}
	return c.JSON(GetAssetFieldsResponse{Fields: items})
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

	if _, err := s.db.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: claims.WorkspaceID,
	}); err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "asset not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load asset")
	}

	body, ok := decodeAndValidate(c, &PatchAssetFieldsRequest{})
	if !ok {
		return nil
	}

	// Snapshot existing values before writing — used for event before/after.
	existingRows, err := s.db.GetAssetFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load field values")
	}
	existingByFieldID := make(map[string]dbgen.GetAssetFieldValuesRow, len(existingRows))
	for _, row := range existingRows {
		existingByFieldID[row.FieldID] = row
	}

	// Validate all first, then write.
	type resolvedEntry struct {
		rv  *resolvedValue
		def dbgen.FieldDefinition
	}
	entries := make([]resolvedEntry, len(body.Values))
	for i, input := range body.Values {
		rv, def, err := s.validateAndResolve(c, claims.WorkspaceID, input)
		if err != nil {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		entries[i] = resolvedEntry{rv: rv, def: def}
	}

	userID := claims.UserID
	for i, e := range entries {
		input := body.Values[i]
		existing := existingByFieldID[input.FieldID]
		beforeVal := fieldRowToValue(existing)
		// Use def for key/name so brand-new fields (no prior value) are correct in audit.
		fieldKey := e.def.Key
		fieldName := e.def.Name
		if e.rv == nil {
			// null value = delete
			if err := s.db.DeleteAssetFieldValue(c.RequestCtx(), dbgen.DeleteAssetFieldValueParams{
				AssetID: id,
				FieldID: input.FieldID,
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusInternalServerError, "could not clear field value")
			}
			s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
				WorkspaceID: claims.WorkspaceID,
				AssetID:     id,
				UserID:      &userID,
				ActorType:   audit.ActorTypeUser,
				EventType:   audit.EventAssetFieldCleared,
				Payload:     audit.AssetFieldClearedPayload{V: 1, FieldKey: fieldKey, FieldName: fieldName, Before: beforeVal},
			})
			continue
		}
		if _, err := s.db.UpsertAssetFieldValue(c.RequestCtx(), dbgen.UpsertAssetFieldValueParams{
			ID:           uuid.NewString(),
			AssetID:      id,
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
		s.audit.WriteAsset(c.RequestCtx(), audit.AssetEvent{
			WorkspaceID: claims.WorkspaceID,
			AssetID:     id,
			UserID:      &userID,
			ActorType:   audit.ActorTypeUser,
			EventType:   audit.EventAssetFieldSet,
			Payload:     audit.AssetFieldSetPayload{V: 1, FieldKey: fieldKey, FieldName: fieldName, Before: beforeVal, After: afterVal},
		})
	}

	// Refresh FTS (best-effort, background)
	go s.refreshAssetFTS(context.Background(), id)

	rows, err := s.db.GetAssetFieldValues(c.RequestCtx(), id)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload field values")
	}
	items := make([]assetFieldValueResponse, len(rows))
	for i, row := range rows {
		items[i] = rowToAssetFieldValueResponse(row)
	}
	return c.JSON(GetAssetFieldsResponse{Fields: items})
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

	// Validate values once (same fields for all assets).
	resolved := make([]*resolvedValue, len(body.Values))
	for i, input := range body.Values {
		rv, _, err := s.validateAndResolve(c, claims.WorkspaceID, input)
		if err != nil {
			return errRes(c, fiber.StatusUnprocessableEntity, err.Error())
		}
		resolved[i] = rv
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not start transaction")
	}
	defer tx.Rollback()
	qtx := s.db.WithTx(tx)

	updatedCount := 0
	for _, assetID := range body.AssetIDs {
		// Verify asset belongs to workspace — use the transaction to avoid read/write lock contention.
		if _, err := qtx.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID:          assetID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			continue
		}

		assetOK := true
		for i, rv := range resolved {
			if rv == nil {
				if err := qtx.DeleteAssetFieldValue(c.RequestCtx(), dbgen.DeleteAssetFieldValueParams{
					AssetID: assetID,
					FieldID: body.Values[i].FieldID,
				}); err != nil && !errors.Is(err, sql.ErrNoRows) {
					assetOK = false
					break
				}
				continue
			}
			if _, err := qtx.UpsertAssetFieldValue(c.RequestCtx(), dbgen.UpsertAssetFieldValueParams{
				ID:           uuid.NewString(),
				AssetID:      assetID,
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
		if assetOK {
			updatedCount++
		}
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	// Refresh FTS for updated assets (best-effort, background). Copy slice to avoid capture of body.
	assetIDsCopy := make([]string, len(body.AssetIDs))
	copy(assetIDsCopy, body.AssetIDs)
	go func() {
		for _, assetID := range assetIDsCopy {
			s.refreshAssetFTS(context.Background(), assetID)
		}
	}()

	return c.JSON(BulkPatchAssetFieldsResponse{Updated: int64(updatedCount)})
}
