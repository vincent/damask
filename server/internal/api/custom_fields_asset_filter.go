package api

import (
	"context"
	"fmt"
	"log"
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

		// Build per-alias WHERE expression
		valExpr := fieldFilterSQL(f)
		// Prefix bare column names with the alias
		for _, col := range []string{"value_text", "value_number", "value_date", "value_boolean"} {
			valExpr = strings.ReplaceAll(valExpr, col, alias+"."+col)
		}
		// Fix double-alias from CAST(alias.value_number …)
		whereFilters[i] = valExpr
		valueArgs = append(valueArgs, fieldFilterValue(f))
	}

	// Assemble args: joins first, then value comparisons, then cursor, then limit
	var args []interface{}
	args = append(args, joinArgs...)
	args = append(args, workspaceID) // for WHERE a.workspace_id = ?
	args = append(args, valueArgs...)

	var cursorClause string
	if cursor := c.Query("cursor"); cursor != "" {
		at, id, err := decodeCursor(cursor)
		if err == nil {
			cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
			args = append(args, at.UTC().Format("2006-01-02 15:04:05"), at.UTC().Format("2006-01-02 15:04:05"), id)
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
		log.Printf("field filter query: %v", err)
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

	return c.JSON(buildAssetListResponse(assets, limit))
}

// fieldFilterSQL returns the SQL comparison snippet (without table alias) for a filter.
// We look up field_id via a subquery so we never interpolate user-supplied values into SQL.
// The value comparison uses a single generated column expression that SQLite evaluates correctly.
func fieldFilterSQL(f fieldFilter) (expr string) {
	// For all operators, coerce the stored value to a comparable string using SQLite's
	// loose typing. The expression must not include a table alias — it's used directly
	// in the JOIN ON clause where the alias is already part of the surrounding context.
	valueCol := "COALESCE(value_text, CAST(value_number AS TEXT), value_date, CAST(value_boolean AS TEXT))"

	switch f.operator {
	case "eq":
		return fmt.Sprintf("%s = ?", valueCol)
	case "lt":
		return fmt.Sprintf("%s < ?", valueCol)
	case "lte":
		return fmt.Sprintf("%s <= ?", valueCol)
	case "gt":
		return fmt.Sprintf("%s > ?", valueCol)
	case "gte":
		return fmt.Sprintf("%s >= ?", valueCol)
	case "contains":
		return "value_text LIKE ?"
	case "starts_with":
		return "value_text LIKE ?"
	}
	return fmt.Sprintf("%s = ?", valueCol)
}

// fieldFilterValue transforms the user-supplied value for the SQL operator.
// Boolean fields are stored as INTEGER (1/0) in SQLite, so "true"/"false" must
// be normalised to "1"/"0" so that the COALESCE comparison works correctly.
func fieldFilterValue(f fieldFilter) interface{} {
	switch f.operator {
	case "contains":
		return "%" + f.value + "%"
	case "starts_with":
		return f.value + "%"
	default:
		v := f.value
		switch strings.ToLower(v) {
		case "true":
			v = "1"
		case "false":
			v = "0"
		}
		return v
	}
}

// refreshAssetFTS updates the FTS5 index for a single asset to include its text field values.
func (s *Server) refreshAssetFTS(ctx context.Context, assetID string) {
	asset, err := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID: assetID,
		// workspace_id check not needed here — internal call
		WorkspaceID: "",
	})
	if err != nil {
		// Try without workspace filter via raw query
		row := s.sqlDB.QueryRowContext(ctx, `SELECT original_filename FROM assets WHERE id = ?`, assetID)
		var name string
		if err2 := row.Scan(&name); err2 != nil {
			return
		}
		asset = dbgen.Asset{ID: assetID, OriginalFilename: name}
	}

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
		log.Printf("fts refresh delete: %v", err)
		return
	}
	_, err = s.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(rowid, original_filename)
		SELECT rowid, ? FROM assets WHERE id = ?
	`, combined, assetID)
	if err != nil {
		log.Printf("fts refresh insert: %v", err)
	}
}
