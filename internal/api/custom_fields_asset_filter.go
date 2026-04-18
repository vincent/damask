package api

import (
	"context"
	"fmt"
	"log/slog"
	"regexp"
	"strings"

	dbgen "damask/server/internal/db/gen"

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

// fieldFilter represents a parsed field[key][op]=value query param.
type fieldFilter struct {
	key      string // field key slug
	operator string // eq, lt, lte, gt, gte, contains, starts_with
	value    string
}

var fieldParamRe = regexp.MustCompile(`^field\[([a-z0-9_]+)\](?:\[([a-z_]+)\])?$`)

func parseFieldFilters(c fiber.Ctx) []fieldFilter {
	var filters []fieldFilter
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
			continue // ignore unknown operators
		}
		dedup := key + ":" + op
		if seen[dedup] {
			continue
		}
		seen[dedup] = true
		filters = append(filters, fieldFilter{key: key, operator: op, value: v})
	}
	return filters
}

const maxFieldFilters = 5

func (s *Server) handleListAssetsByFields(c fiber.Ctx, workspaceID string, limit int64) error {
	filters := parseFieldFilters(c)
	if len(filters) > maxFieldFilters {
		return errRes(c, fiber.StatusUnprocessableEntity, fmt.Sprintf("maximum of %d field filters allowed", maxFieldFilters))
	}
	if len(filters) == 0 {
		return errRes(c, fiber.StatusBadRequest, "no valid field filters provided")
	}

	// Build one JOIN per filter plus a WHERE condition for the value comparison.
	// Args order: [workspaceID for WHERE] then per-join [workspaceID, key] then per-filter [value] then cursor... then limit.
	joins := make([]string, len(filters))
	whereFilters := make([]string, len(filters))
	var joinArgs []interface{}  // workspace_id + key per join
	var valueArgs []interface{} // value per filter

	for i, f := range filters {
		alias := fmt.Sprintf("v%d", i+1)
		joins[i] = fmt.Sprintf(
			`JOIN asset_field_values %s ON %s.asset_id = a.id AND %s.field_id = (SELECT id FROM field_definitions WHERE workspace_id = ? AND key = ? AND deleted_at IS NULL LIMIT 1)`,
			alias, alias, alias,
		)
		joinArgs = append(joinArgs, workspaceID, f.key)

		whereFilters[i] = fieldFilterSQL(f, alias)
		valueArgs = append(valueArgs, fieldFilterValue(f)...)
	}

	// Assemble args: joins first, then value comparisons, then cursor, then limit
	var args []interface{}
	args = append(args, joinArgs...)
	args = append(args, workspaceID) // for WHERE a.workspace_id = ?
	args = append(args, valueArgs...)

	var cursorClause string
	if cursor := c.Query("cursor"); cursor != "" {
		cv, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, cv.Value, cv.Value, cv.ID)
		}
	}
	args = append(args, limit)

	filterClauses := ""
	if len(whereFilters) > 0 {
		filterClauses = "AND " + strings.Join(whereFilters, " AND ")
	}

	query := fmt.Sprintf(`
		SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.original_filename, a.storage_key,
		       a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		       a.created_at, a.updated_at
		FROM assets a
		%s
		WHERE a.workspace_id = ?
		  %s
		  %s
		ORDER BY a.created_at DESC, a.id DESC
		LIMIT ?
	`, strings.Join(joins, "\n"), filterClauses, cursorClause)

	rows, err := s.sqlDB.QueryContext(c.RequestCtx(), query, args...)
	if err != nil {
		slog.Error("field filter query", "error", err)
		return errRes(c, fiber.StatusInternalServerError, "could not list assets")
	}
	defer rows.Close()

	var assets []dbgen.Asset
	for rows.Next() {
		var a dbgen.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return errRes(c, fiber.StatusInternalServerError, "scan failed")
		}
		assets = append(assets, a)
	}
	if err := rows.Err(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "query failed")
	}

	versionCounts := s.batchVersionCounts(c.RequestCtx(), assets)
	variantCounts := s.batchVariantCounts(c.RequestCtx(), assets)
	return c.JSON(buildAssetListResponseWithCounts(assets, limit, "created_at", versionCounts, variantCounts))
}

// fieldFilterSQL returns the SQL WHERE expression for a filter using the given table alias.
// We never interpolate user-supplied values into SQL — only the alias (controlled internally)
// and operator (validated against an allowlist) appear in the format string.
func fieldFilterSQL(f fieldFilter, alias string) string {
	// Text/date/boolean equality: COALESCE all columns to a comparable text value.
	textCol := fmt.Sprintf("COALESCE(%s.value_text, %s.value_date, CAST(%s.value_boolean AS TEXT))", alias, alias, alias)
	// Numeric ordering: use the numeric column directly so comparisons are arithmetic, not lexicographic.
	numCol := fmt.Sprintf("CAST(%s.value_number AS REAL)", alias)

	switch f.operator {
	case "eq":
		return fmt.Sprintf("COALESCE(%s.value_text, CAST(%s.value_number AS TEXT), %s.value_date, CAST(%s.value_boolean AS TEXT)) = ?",
			alias, alias, alias, alias)
	case "lt":
		return fmt.Sprintf("(%s.value_number IS NOT NULL AND %s < ? OR %s.value_number IS NULL AND %s < ?)", alias, numCol, alias, textCol)
	case "lte":
		return fmt.Sprintf("(%s.value_number IS NOT NULL AND %s <= ? OR %s.value_number IS NULL AND %s <= ?)", alias, numCol, alias, textCol)
	case "gt":
		return fmt.Sprintf("(%s.value_number IS NOT NULL AND %s > ? OR %s.value_number IS NULL AND %s > ?)", alias, numCol, alias, textCol)
	case "gte":
		return fmt.Sprintf("(%s.value_number IS NOT NULL AND %s >= ? OR %s.value_number IS NULL AND %s >= ?)", alias, numCol, alias, textCol)
	case "contains":
		return fmt.Sprintf("%s.value_text LIKE ?", alias)
	case "starts_with":
		return fmt.Sprintf("%s.value_text LIKE ?", alias)
	}
	return fmt.Sprintf("COALESCE(%s.value_text, CAST(%s.value_number AS TEXT), %s.value_date, CAST(%s.value_boolean AS TEXT)) = ?",
		alias, alias, alias, alias)
}

// fieldFilterValue returns the SQL argument(s) for the operator.
// lt/lte/gt/gte emit two placeholders (numeric branch + text branch), so two args are returned.
// Boolean normalisation ("true"→"1") is only applied for eq, where the COALESCE text comparison
// needs it — ordering operators on boolean fields are meaningless and left as-is.
func fieldFilterValue(f fieldFilter) []interface{} {
	switch f.operator {
	case "contains":
		return []interface{}{"%" + f.value + "%"}
	case "starts_with":
		return []interface{}{f.value + "%"}
	case "lt", "lte", "gt", "gte":
		// Two placeholders: numeric value (REAL), then text fallback.
		return []interface{}{f.value, f.value}
	default: // eq
		v := f.value
		switch strings.ToLower(v) {
		case "true":
			v = "1"
		case "false":
			v = "0"
		}
		return []interface{}{v}
	}
}

// refreshAssetFTS updates the FTS5 index for a single asset to include its text field values.
func (s *Server) refreshAssetFTS(ctx context.Context, assetID string) {
	// Use a raw query — GetAssetByID requires workspace_id which is not available here.
	var originalFilename string
	row := s.sqlDB.QueryRowContext(ctx, `SELECT original_filename FROM assets WHERE id = ?`, assetID)
	if err := row.Scan(&originalFilename); err != nil {
		return
	}
	asset := dbgen.Asset{ID: assetID, OriginalFilename: originalFilename}

	// Gather text field values
	rows, err := s.sqlDB.QueryContext(ctx, `
		SELECT v.value_text
		FROM asset_field_values v
		JOIN field_definitions f ON f.id = v.field_id
		WHERE v.asset_id = ? AND f.field_type IN ('text', 'url', 'select') AND f.deleted_at IS NULL AND v.value_text IS NOT NULL
	`, assetID)
	if err != nil {
		return
	}
	defer rows.Close()

	var parts []string
	parts = append(parts, asset.OriginalFilename)
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			parts = append(parts, t)
		}
	}
	_ = rows.Err()

	// FTS5 external content update: delete old entry, insert new one with all text
	combined := strings.Join(parts, " ")
	_, err = s.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(assets_fts, rowid, original_filename)
		SELECT 'delete', rowid, original_filename FROM assets WHERE id = ?
	`, assetID)
	if err != nil {
		slog.Error("fts refresh delete", "error", err)
		return
	}
	_, err = s.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(rowid, original_filename)
		SELECT rowid, ? FROM assets WHERE id = ?
	`, combined, assetID)
	if err != nil {
		slog.Error("fts refresh insert", "error", err)
	}
}
