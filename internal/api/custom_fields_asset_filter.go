package api

import (
	"fmt"
	"regexp"
	"strings"

	"damask/server/internal/service"

	"github.com/gofiber/fiber/v3"
)

// hasFieldFilters returns true if the request contains any field[...] query params.
func hasFieldFilters(c fiber.Ctx) bool {
	for k := range c.Queries() {
		if strings.HasPrefix(k, "field[") {
			return true
		}
	}
	return false
}

// fieldFilterDef represents a parsed field[key][op]=value query param.
type fieldFilterDef struct {
	key      string
	operator string
	value    string
}

var fieldParamRe = regexp.MustCompile(`^field\[([a-z0-9_]+)\](?:\[([a-z_]+)\])?$`)

func parseFieldFilters(c fiber.Ctx) []fieldFilterDef {
	var filters []fieldFilterDef
	seen := map[string]bool{}

	for k, v := range c.Queries() {
		m := fieldParamRe.FindStringSubmatch(k)
		if m == nil {
			continue
		}
		key := m[1]
		op := "eq"
		if len(m[2]) > 0 {
			op = m[2]
		}
		switch op {
		case "eq", "lt", "lte", "gt", "gte", "contains", "starts_with":
		default:
			continue
		}
		dedup := key + ":" + op
		if seen[dedup] {
			continue
		}
		seen[dedup] = true
		filters = append(filters, fieldFilterDef{key: key, operator: op, value: v})
	}
	return filters
}

const maxFieldFilters = 5

func (s *Server) handleListAssetsByFields(c fiber.Ctx, workspaceID string, limit int64) error {
	defs := parseFieldFilters(c)
	if len(defs) > maxFieldFilters {
		return errRes(c, fiber.StatusUnprocessableEntity, fmt.Sprintf("maximum of %d field filters allowed", maxFieldFilters))
	}
	if len(defs) == 0 {
		return errRes(c, fiber.StatusBadRequest, "no valid field filters provided")
	}

	svcFilters := make([]service.FieldFilter, len(defs))
	for i, f := range defs {
		svcFilters[i] = service.FieldFilter{Key: f.key, Operator: f.operator, Value: f.value}
	}

	var cursorAt *string
	var cursorID *string
	if cursor := c.Query("cursor"); cursor != "" {
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorAt = &cv.Value
			cursorID = &cv.ID
		}
	}

	assets, err := s.assets.ListByFields(c.RequestCtx(), service.ListAssetsByFieldsParams{
		WorkspaceID:  workspaceID,
		FieldFilters: svcFilters,
		CursorAt:     cursorAt,
		CursorID:     cursorID,
		Limit:        limit,
	})
	if err != nil {
		return ErrorStatusResponse(c, err)
	}

	// Convert service DTOs to the existing response type via dbgen shim.
	// batchVersionCounts / batchVariantCounts still use s.db; that is handled
	// by the assets handler layer which calls those helpers directly.
	// For now return the slim asset list without counts (consistent with other list paths).
	return c.JSON(buildAssetListResponseFromDTOs(assets, limit, "created_at", nil, nil))
}
