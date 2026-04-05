package api

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// -- Shared value validation & typed columns ---------------------------------

var dateRe = regexp.MustCompile(`^\d{4}-\d{2}-\d{2}$`)

type fieldValueInput struct {
	FieldID string      `json:"field_id"`
	Value   interface{} `json:"value"`
}

type resolvedValue struct {
	fieldID      string
	valueText    *string
	valueNumber  *float64
	valueDate    *string
	valueBoolean *int64
}

// validateAndResolve loads the field definition and validates the incoming value.
// Returns nil resolvedValue when value is null (clear intent).
func (s *Server) validateAndResolve(c fiber.Ctx, workspaceID string, input fieldValueInput) (*resolvedValue, error) {
	def, err := s.db.GetFieldDefinitionByID(c.RequestCtx(), dbgen.GetFieldDefinitionByIDParams{
		ID:          input.FieldID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("field %s not found", input.FieldID)
		}
		return nil, fmt.Errorf("could not load field %s", input.FieldID)
	}
	if def.DeletedAt != nil {
		return nil, fmt.Errorf("field %s has been deleted", input.FieldID)
	}

	// null value → explicit clear
	if input.Value == nil {
		return nil, nil //nolint:nilnil
	}

	rv := &resolvedValue{fieldID: input.FieldID}

	switch def.FieldType {
	case "text", "url":
		s, ok := input.Value.(string)
		if !ok {
			return nil, fmt.Errorf("field %s expects a string value", def.Key)
		}
		rv.valueText = &s

	case "select":
		s, ok := input.Value.(string)
		if !ok {
			return nil, fmt.Errorf("field %s expects a string value", def.Key)
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
					return nil, fmt.Errorf("value '%s' is not a valid option for field %s", s, def.Key)
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
				return nil, fmt.Errorf("field %s expects a numeric value", def.Key)
			}
			rv.valueNumber = &f
		default:
			return nil, fmt.Errorf("field %s expects a numeric value", def.Key)
		}

	case "date":
		s, ok := input.Value.(string)
		if !ok || !dateRe.MatchString(s) {
			return nil, fmt.Errorf("field %s expects a date in YYYY-MM-DD format", def.Key)
		}
		if _, err := time.Parse("2006-01-02", s); err != nil {
			return nil, fmt.Errorf("field %s: invalid date '%s'", def.Key, s)
		}
		rv.valueDate = &s

	case "boolean":
		b, ok := input.Value.(bool)
		if !ok {
			return nil, fmt.Errorf("field %s expects a boolean value", def.Key)
		}
		var v int64
		if b {
			v = 1
		}
		rv.valueBoolean = &v
	}

	return rv, nil
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

// -- Handlers -----------------------------------------------------------------

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
	return c.JSON(fiber.Map{"fields": items})
}

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

	var body struct {
		Values []fieldValueInput `json:"values"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(body.Values) == 0 {
		return errRes(c, fiber.StatusBadRequest, "values is required")
	}

	// Validate all first, then write
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
			// null value = delete
			if err := s.db.DeleteAssetFieldValue(c.RequestCtx(), dbgen.DeleteAssetFieldValueParams{
				AssetID: id,
				FieldID: body.Values[i].FieldID,
			}); err != nil && !errors.Is(err, sql.ErrNoRows) {
				return errRes(c, fiber.StatusInternalServerError, "could not clear field value")
			}
			continue
		}
		if _, err := s.db.UpsertAssetFieldValue(c.RequestCtx(), dbgen.UpsertAssetFieldValueParams{
			ID:           uuid.NewString(),
			AssetID:      id,
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
	return c.JSON(fiber.Map{"fields": items})
}

func (s *Server) handleBulkPatchAssetFields(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var body struct {
		AssetIDs []string         `json:"asset_ids"`
		Values   []fieldValueInput `json:"values"`
	}
	if err := c.Bind().Body(&body); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if len(body.AssetIDs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "asset_ids is required")
	}
	if len(body.Values) == 0 {
		return errRes(c, fiber.StatusBadRequest, "values is required")
	}

	// Validate values once (same fields for all assets)
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

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not start transaction")
	}
	defer tx.Rollback() //nolint:errcheck
	qtx := s.db.WithTx(tx)

	updatedCount := 0
	for _, assetID := range body.AssetIDs {
		// Verify asset belongs to workspace — use the transaction to avoid read/write lock contention
		if _, err := qtx.GetAssetByID(c.RequestCtx(), dbgen.GetAssetByIDParams{
			ID:          assetID,
			WorkspaceID: claims.WorkspaceID,
		}); err != nil {
			continue
		}

		for i, rv := range resolved {
			if rv == nil {
				_ = qtx.DeleteAssetFieldValue(c.RequestCtx(), dbgen.DeleteAssetFieldValueParams{
					AssetID: assetID,
					FieldID: body.Values[i].FieldID,
				})
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
		updatedCount++
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	// Refresh FTS for updated assets (best-effort, use background context)
	go func() {
		for _, assetID := range body.AssetIDs {
			s.refreshAssetFTS(context.Background(), assetID)
		}
	}()

	return c.JSON(fiber.Map{"updated": updatedCount})
}
