package reposqlc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type assetRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewAssetRepo returns a repository.AssetRepository backed by sqlc-generated queries.
func NewAssetRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.AssetRepository {
	return &assetRepo{q: q, sqlDB: sqlDB}
}

func (r *assetRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Asset, error) {
	row, err := r.q.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Asset{}, apperr.ErrNotFound
		}
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

func (r *assetRepo) List(ctx context.Context, params repository.ListAssetsParams) ([]repository.Asset, error) {
	rows, err := r.q.ListAssets(ctx, dbgen.ListAssetsParams{
		WorkspaceID: params.WorkspaceID,
		ProjectID:   params.ProjectID,
		MimePrefix:  params.MimePrefix,
		CursorAt:    params.CursorAt,
		CursorID:    params.CursorID,
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Asset, len(rows))
	for i, row := range rows {
		out[i] = toAsset(row)
	}
	return out, nil
}

func (r *assetRepo) Create(ctx context.Context, params repository.CreateAssetParams) (repository.Asset, error) {
	row, err := r.q.CreateAsset(ctx, dbgen.CreateAssetParams{
		ID:               params.ID,
		WorkspaceID:      params.WorkspaceID,
		ProjectID:        params.ProjectID,
		OriginalFilename: params.OriginalFilename,
		StorageKey:       params.StorageKey,
		MimeType:         params.MimeType,
		Size:             params.Size,
		Width:            params.Width,
		Height:           params.Height,
		Metadata:         params.Metadata,
	})
	if err != nil {
		return repository.Asset{}, err
	}
	return toAsset(row), nil
}

// Update applies whichever optional fields are set in params.
// The repository makes individual sqlc calls per updated field because sqlc
// generates separate update queries rather than a single partial-update query.
func (r *assetRepo) Update(ctx context.Context, params repository.UpdateAssetParams) (repository.Asset, error) {
	if params.OriginalFilename != nil {
		if err := r.q.UpdateAssetName(ctx, dbgen.UpdateAssetNameParams{
			ID:               params.ID,
			WorkspaceID:      params.WorkspaceID,
			OriginalFilename: *params.OriginalFilename,
		}); err != nil {
			return repository.Asset{}, err
		}
	}
	if params.FolderID != nil || params.ProjectID != nil {
		if params.FolderID != nil {
			if err := r.q.UpdateAssetFolder(ctx, dbgen.UpdateAssetFolderParams{
				ID:          params.ID,
				WorkspaceID: params.WorkspaceID,
				FolderID:    params.FolderID,
			}); err != nil {
				return repository.Asset{}, err
			}
		}
		if params.ProjectID != nil {
			if err := r.q.UpdateAssetProject(ctx, dbgen.UpdateAssetProjectParams{
				ID:          params.ID,
				WorkspaceID: params.WorkspaceID,
				ProjectID:   params.ProjectID,
			}); err != nil {
				return repository.Asset{}, err
			}
		}
	}
	if params.ThumbnailKey != nil {
		if err := r.q.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ID:           params.ID,
			ThumbnailKey: params.ThumbnailKey,
		}); err != nil {
			return repository.Asset{}, err
		}
	}
	if params.CurrentVersionID != nil {
		if err := r.q.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
			ID:               params.ID,
			CurrentVersionID: params.CurrentVersionID,
		}); err != nil {
			return repository.Asset{}, err
		}
	}
	if params.Width != nil || params.Height != nil {
		if err := r.q.UpdateAssetDimensions(ctx, dbgen.UpdateAssetDimensionsParams{
			ID:     params.ID,
			Width:  params.Width,
			Height: params.Height,
		}); err != nil {
			return repository.Asset{}, err
		}
	}
	return r.GetByID(ctx, params.WorkspaceID, params.ID)
}

func (r *assetRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	return r.q.DeleteAsset(ctx, dbgen.DeleteAssetParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *assetRepo) IsProjectCover(ctx context.Context, workspaceID, assetID string) (bool, error) {
	_, err := r.q.GetProjectByCoverAsset(ctx, dbgen.GetProjectByCoverAssetParams{
		CoverAssetID: &assetID,
		WorkspaceID:  workspaceID,
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (r *assetRepo) IsWorkspaceIcon(ctx context.Context, workspaceID, assetID string) (bool, error) {
	_, err := r.q.GetWorkspaceByIconAsset(ctx, dbgen.GetWorkspaceByIconAssetParams{
		IconAssetID: &assetID,
		ID:          workspaceID,
	})
	if err == nil {
		return true, nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return false, nil
	}
	return false, err
}

func (r *assetRepo) CountByIDs(ctx context.Context, workspaceID string, ids []string) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}
	idsJSON, err := json.Marshal(ids)
	if err != nil {
		return 0, err
	}
	row := r.sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM assets WHERE workspace_id = ? AND id IN (SELECT value FROM json_each(?))`,
		workspaceID, string(idsJSON),
	)
	var count int64
	return count, row.Scan(&count)
}

func (r *assetRepo) RefreshFTS(ctx context.Context, assetID string) error {
	var originalFilename string
	row := r.sqlDB.QueryRowContext(ctx, `SELECT original_filename FROM assets WHERE id = ?`, assetID)
	if err := row.Scan(&originalFilename); err != nil {
		return nil // asset not found — no-op
	}

	rows, err := r.sqlDB.QueryContext(ctx, `
		SELECT v.value_text
		FROM asset_field_values v
		JOIN field_definitions f ON f.id = v.field_id
		WHERE v.asset_id = ? AND f.field_type IN ('text', 'url', 'select') AND f.deleted_at IS NULL AND v.value_text IS NOT NULL
	`, assetID)
	if err != nil {
		return err
	}
	defer rows.Close()

	parts := []string{originalFilename}
	for rows.Next() {
		var t string
		if err := rows.Scan(&t); err == nil {
			parts = append(parts, t)
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}

	combined := strings.Join(parts, " ")
	if _, err := r.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(assets_fts, rowid, original_filename)
		SELECT 'delete', rowid, original_filename FROM assets WHERE id = ?
	`, assetID); err != nil {
		slog.Error("fts refresh delete", "asset_id", assetID, "error", err)
		return err
	}
	if _, err := r.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(rowid, original_filename)
		SELECT rowid, ? FROM assets WHERE id = ?
	`, combined, assetID); err != nil {
		slog.Error("fts refresh insert", "asset_id", assetID, "error", err)
		return err
	}
	return nil
}

func (r *assetRepo) ListByFields(ctx context.Context, params repository.ListAssetsByFieldsParams) ([]repository.Asset, error) {
	filters := params.FieldFilters
	if len(filters) == 0 {
		return nil, nil
	}

	joins := make([]string, len(filters))
	whereFilters := make([]string, len(filters))
	var joinArgs []interface{}
	var valueArgs []interface{}

	for i, f := range filters {
		alias := fmt.Sprintf("v%d", i+1)
		joins[i] = fmt.Sprintf(
			`JOIN asset_field_values %s ON %s.asset_id = a.id AND %s.field_id = (SELECT id FROM field_definitions WHERE workspace_id = ? AND key = ? AND deleted_at IS NULL LIMIT 1)`,
			alias, alias, alias,
		)
		joinArgs = append(joinArgs, params.WorkspaceID, f.Key)
		whereFilters[i] = fieldFilterSQL(f, alias)
		valueArgs = append(valueArgs, fieldFilterArgs(f)...)
	}

	var args []interface{}
	args = append(args, joinArgs...)
	args = append(args, params.WorkspaceID)
	args = append(args, valueArgs...)

	var cursorClause string
	if params.CursorAt != nil && params.CursorID != nil {
		cursorClause = "AND (a.created_at < ? OR (a.created_at = ? AND a.id < ?))"
		args = append(args, *params.CursorAt, *params.CursorAt, *params.CursorID)
	}
	args = append(args, params.Limit)

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

	rows, err := r.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, fmt.Errorf("ListByFields: %w", err)
	}
	defer rows.Close()

	var out []repository.Asset
	for rows.Next() {
		var a dbgen.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, fmt.Errorf("ListByFields scan: %w", err)
		}
		out = append(out, toAsset(a))
	}
	return out, rows.Err()
}

// fieldFilterSQL returns the SQL WHERE clause for one filter using the given table alias.
// Only the alias (internally controlled) and operator (validated allowlist) appear in the format string.
func fieldFilterSQL(f repository.FieldFilter, alias string) string {
	textCol := fmt.Sprintf("COALESCE(%s.value_text, %s.value_date, CAST(%s.value_boolean AS TEXT))", alias, alias, alias)
	numCol := fmt.Sprintf("CAST(%s.value_number AS REAL)", alias)
	switch f.Operator {
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

// fieldFilterArgs returns the SQL argument(s) for the filter operator.
func fieldFilterArgs(f repository.FieldFilter) []interface{} {
	switch f.Operator {
	case "contains":
		return []interface{}{"%" + f.Value + "%"}
	case "starts_with":
		return []interface{}{f.Value + "%"}
	case "lt", "lte", "gt", "gte":
		return []interface{}{f.Value, f.Value}
	default: // eq
		v := f.Value
		switch strings.ToLower(v) {
		case "true":
			v = "1"
		case "false":
			v = "0"
		}
		return []interface{}{v}
	}
}

func toAsset(a dbgen.Asset) repository.Asset {
	return repository.Asset{
		ID:               a.ID,
		WorkspaceID:      a.WorkspaceID,
		ProjectID:        a.ProjectID,
		FolderID:         a.FolderID,
		OriginalFilename: a.OriginalFilename,
		StorageKey:       a.StorageKey,
		MimeType:         a.MimeType,
		Size:             a.Size,
		Width:            a.Width,
		Height:           a.Height,
		ThumbnailKey:     a.ThumbnailKey,
		Metadata:         a.Metadata,
		CurrentVersionID: a.CurrentVersionID,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}
