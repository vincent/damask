package reposqlc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

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

func (r *assetRepo) List(ctx context.Context, p repository.ListAssetsParams) ([]repository.Asset, error) {
	var joins []string
	var where []string
	var args []any

	// Base table alias differs when joining for taken_at sort.
	from := "assets a"

	where = append(where, "a.workspace_id = ?")
	args = append(args, p.WorkspaceID)

	switch {
	case p.FolderIsRoot:
		where = append(where, "a.folder_id IS NULL")
		if p.ProjectID != nil {
			where = append(where, "a.project_id = ?")
			args = append(args, *p.ProjectID)
		}
	case p.FolderID != nil:
		where = append(where, "a.folder_id = ?")
		args = append(args, *p.FolderID)
	case p.ProjectID != nil:
		where = append(where, "a.project_id = ?")
		args = append(args, *p.ProjectID)
	}

	if p.CollectionID != nil {
		where = append(where, "a.id IN (SELECT asset_id FROM collection_assets WHERE collection_id = ?)")
		args = append(args, *p.CollectionID)
	}

	if len(p.TagNames) > 0 {
		placeholders := strings.Repeat("?,", len(p.TagNames))
		placeholders = placeholders[:len(placeholders)-1]
		where = append(where, fmt.Sprintf(
			"a.id IN (SELECT at.asset_id FROM asset_tags at JOIN tags t ON t.id = at.tag_id WHERE t.workspace_id = ? AND t.name IN (%s) GROUP BY at.asset_id HAVING COUNT(DISTINCT t.id) = ?)",
			placeholders,
		))
		args = append(args, p.WorkspaceID)
		for _, name := range p.TagNames {
			args = append(args, name)
		}
		args = append(args, int64(len(p.TagNames)))
	}

	if p.SearchQuery != "" {
		where = append(
			where,
			"(a.rowid IN (SELECT rowid FROM assets_fts WHERE assets_fts MATCH ?) OR a.id IN (SELECT asset_id FROM assets_text_fts WHERE workspace_id = ? AND assets_text_fts MATCH ?))",
		)
		args = append(args, p.SearchQuery+"*", p.WorkspaceID, p.SearchQuery+"*")
	}

	if p.MimePrefix != nil {
		where = append(where, "a.mime_type LIKE ?")
		args = append(args, *p.MimePrefix+"%")
	}

	// Cursor
	if p.CursorID != "" && p.CursorValue != "" {
		switch p.CursorField {
		case "size":
			if p.SortDesc {
				where = append(where, "(a.size < ? OR (a.size = ? AND a.id < ?))")
			} else {
				where = append(where, "(a.size > ? OR (a.size = ? AND a.id < ?))")
			}
			args = append(args, p.CursorValue, p.CursorValue, p.CursorID)
		case "id":
			if p.SortDesc {
				where = append(where, "a.id < ?")
			} else {
				where = append(where, "a.id > ?")
			}
			args = append(args, p.CursorID)
		default: // created_at
			if p.SortDesc || p.SortField == "" {
				where = append(where, "(a.created_at < ? OR (a.created_at = ? AND a.id < ?))")
			} else {
				where = append(where, "(a.created_at > ? OR (a.created_at = ? AND a.id > ?))")
			}
			args = append(args, p.CursorValue, p.CursorValue, p.CursorID)
		}
	}

	// ORDER BY
	var orderBy string
	switch p.SortField {
	case "size":
		if p.SortDesc {
			orderBy = "a.size DESC, a.id DESC"
		} else {
			orderBy = "a.size ASC, a.id DESC"
		}
	case "id":
		if p.SortDesc {
			orderBy = "a.id DESC"
		} else {
			orderBy = "a.id ASC"
		}
	case "taken_at":
		joins = append(joins, "LEFT JOIN asset_field_values afv ON afv.asset_id = a.id AND afv.field_id = ?")
		// ExifFieldID goes before WHERE args — prepend it.
		newArgs := []any{p.ExifFieldID}
		newArgs = append(newArgs, args...)
		args = newArgs
		dir := "ASC"
		if p.SortDesc {
			dir = "DESC"
		}
		orderBy = fmt.Sprintf("afv.value_date %s NULLS LAST, a.created_at DESC, a.id DESC", dir)
	case "created_at_asc":
		orderBy = "a.created_at ASC, a.id ASC"
	default: // created_at DESC
		orderBy = "a.created_at DESC, a.id DESC"
	}

	args = append(args, p.Limit)

	joinSQL := ""
	if len(joins) > 0 {
		joinSQL = " " + strings.Join(joins, " ")
	}
	query := fmt.Sprintf(
		`SELECT a.id, a.workspace_id, a.project_id, a.folder_id, a.derived_from_asset_id, a.original_filename, a.storage_key,
		        a.mime_type, a.size, a.width, a.height, a.thumbnail_key, a.metadata,
		        a.current_version_id, a.created_at, a.updated_at
		 FROM %s%s
		 WHERE %s
		 ORDER BY %s
		 LIMIT ?`,
		from,
		joinSQL,
		strings.Join(where, " AND "),
		orderBy,
	)

	rows, err := r.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.Asset
	for rows.Next() {
		var a repository.Asset
		if err := rows.Scan(
			&a.ID, &a.WorkspaceID, &a.ProjectID, &a.FolderID, &a.DerivedFromAssetID, &a.OriginalFilename, &a.StorageKey,
			&a.MimeType, &a.Size, &a.Width, &a.Height, &a.ThumbnailKey, &a.Metadata,
			&a.CurrentVersionID, &a.CreatedAt, &a.UpdatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, a)
	}
	return out, rows.Err()
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
		slog.ErrorContext(ctx, "fts refresh delete", "asset_id", assetID, "error", err)
		return err
	}
	if _, err := r.sqlDB.ExecContext(ctx, `
		INSERT INTO assets_fts(rowid, original_filename)
		SELECT rowid, ? FROM assets WHERE id = ?
	`, combined, assetID); err != nil {
		slog.ErrorContext(ctx, "fts refresh insert", "asset_id", assetID, "error", err)
		return err
	}
	return nil
}

func (r *assetRepo) ListByFields(
	ctx context.Context,
	params repository.ListAssetsByFieldsParams,
) ([]repository.Asset, error) {
	filters := params.FieldFilters
	if len(filters) == 0 {
		return nil, nil
	}

	joins := make([]string, len(filters))
	whereFilters := make([]string, len(filters))
	var joinArgs []any
	var valueArgs []any

	for i, f := range filters {
		alias := fmt.Sprintf("v%d", i+1)
		joins[i] = fmt.Sprintf(
			`JOIN asset_field_values %s ON %s.asset_id = a.id AND %s.field_id = (SELECT id FROM field_definitions WHERE workspace_id = ? AND key = ? AND deleted_at IS NULL LIMIT 1)`,
			alias,
			alias,
			alias,
		)
		joinArgs = append(joinArgs, params.WorkspaceID, f.Key)
		whereFilters[i] = fieldFilterSQL(f, alias)
		valueArgs = append(valueArgs, fieldFilterArgs(f)...)
	}

	var args []any
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
	textCol := fmt.Sprintf(
		"COALESCE(%s.value_text, %s.value_date, CAST(%s.value_boolean AS TEXT))",
		alias,
		alias,
		alias,
	)
	numCol := fmt.Sprintf("CAST(%s.value_number AS REAL)", alias)
	switch f.Operator {
	case "eq":
		return fmt.Sprintf(
			"COALESCE(%s.value_text, CAST(%s.value_number AS TEXT), %s.value_date, CAST(%s.value_boolean AS TEXT)) = ?",
			alias,
			alias,
			alias,
			alias,
		)
	case "lt":
		return fmt.Sprintf(
			"(%s.value_number IS NOT NULL AND %s < ? OR %s.value_number IS NULL AND %s < ?)",
			alias,
			numCol,
			alias,
			textCol,
		)
	case "lte":
		return fmt.Sprintf(
			"(%s.value_number IS NOT NULL AND %s <= ? OR %s.value_number IS NULL AND %s <= ?)",
			alias,
			numCol,
			alias,
			textCol,
		)
	case "gt":
		return fmt.Sprintf(
			"(%s.value_number IS NOT NULL AND %s > ? OR %s.value_number IS NULL AND %s > ?)",
			alias,
			numCol,
			alias,
			textCol,
		)
	case "gte":
		return fmt.Sprintf(
			"(%s.value_number IS NOT NULL AND %s >= ? OR %s.value_number IS NULL AND %s >= ?)",
			alias,
			numCol,
			alias,
			textCol,
		)
	case "contains":
		return fmt.Sprintf("%s.value_text LIKE ?", alias)
	case "starts_with":
		return fmt.Sprintf("%s.value_text LIKE ?", alias)
	}
	return fmt.Sprintf(
		"COALESCE(%s.value_text, CAST(%s.value_number AS TEXT), %s.value_date, CAST(%s.value_boolean AS TEXT)) = ?",
		alias,
		alias,
		alias,
		alias,
	)
}

// fieldFilterArgs returns the SQL argument(s) for the filter operator.
func fieldFilterArgs(f repository.FieldFilter) []any {
	switch f.Operator {
	case "contains":
		return []any{"%" + f.Value + "%"}
	case "starts_with":
		return []any{f.Value + "%"}
	case "lt", "lte", "gt", "gte":
		return []any{f.Value, f.Value}
	default: // eq
		v := f.Value
		switch strings.ToLower(v) {
		case "true":
			v = "1"
		case "false":
			v = "0"
		}
		return []any{v}
	}
}

func toAsset(a dbgen.Asset) repository.Asset {
	return repository.Asset{
		ID:                   a.ID,
		WorkspaceID:          a.WorkspaceID,
		ProjectID:            a.ProjectID,
		FolderID:             a.FolderID,
		DerivedFromAssetID:   a.DerivedFromAssetID,
		OriginalFilename:     a.OriginalFilename,
		StorageKey:           a.StorageKey,
		MimeType:             a.MimeType,
		Size:                 a.Size,
		Width:                a.Width,
		Height:               a.Height,
		ThumbnailKey:         a.ThumbnailKey,
		ThumbnailContentType: a.ThumbnailContentType,
		Metadata:             a.Metadata,
		CurrentVersionID:     a.CurrentVersionID,
		CreatedAt:            a.CreatedAt,
		UpdatedAt:            a.UpdatedAt,
	}
}

func (r *assetRepo) CollectStorageKeys(
	ctx context.Context,
	workspaceID, assetID string,
) (repository.AssetStorageKeys, error) {
	asset, err := r.q.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: assetID, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetStorageKeys{}, apperr.ErrNotFound
		}
		return repository.AssetStorageKeys{}, err
	}
	out := repository.AssetStorageKeys{
		AssetKey: asset.StorageKey,
		ThumbKey: asset.ThumbnailKey,
	}
	versions, err := r.q.ListAllVersions(ctx, assetID)
	if err != nil {
		return repository.AssetStorageKeys{}, err
	}
	for _, v := range versions {
		vk := repository.VersionStorageKeys{
			StorageKey:   v.StorageKey,
			ThumbnailKey: v.ThumbnailKey,
		}
		variants, err := r.q.ListVariantsByVersion(ctx, v.ID)
		if err != nil {
			return repository.AssetStorageKeys{}, err
		}
		for _, variant := range variants {
			vk.VariantKeys = append(vk.VariantKeys, variant.StorageKey)
		}
		out.VersionKeys = append(out.VersionKeys, vk)
	}
	textTrackRows, err := r.sqlDB.QueryContext(
		ctx,
		`SELECT storage_key FROM asset_text_tracks WHERE asset_id = ? AND workspace_id = ? AND storage_key IS NOT NULL`,
		assetID,
		workspaceID,
	)
	if err != nil {
		return repository.AssetStorageKeys{}, err
	}
	defer textTrackRows.Close()
	for textTrackRows.Next() {
		var key string
		if err := textTrackRows.Scan(&key); err != nil {
			return repository.AssetStorageKeys{}, err
		}
		out.TextTrackKeys = append(out.TextTrackKeys, key)
	}
	if err := textTrackRows.Err(); err != nil {
		return repository.AssetStorageKeys{}, err
	}
	return out, nil
}

func (r *assetRepo) HardDelete(ctx context.Context, workspaceID, assetID string) error {
	_ = r.q.DeleteTextFTSByAsset(ctx, assetID)
	return r.q.DeleteAsset(ctx, dbgen.DeleteAssetParams{ID: assetID, WorkspaceID: workspaceID})
}

func (r *assetRepo) CountVersionsByAsset(ctx context.Context, assetID string) (int64, error) {
	return r.q.CountVersionsForAsset(ctx, assetID)
}

func (r *assetRepo) CountVariantsByCurrentVersion(ctx context.Context, assetID string) (int64, error) {
	var currentVersionID string
	err := r.sqlDB.QueryRowContext(ctx,
		`SELECT COALESCE(current_version_id, '') FROM assets WHERE id = ?`, assetID,
	).Scan(&currentVersionID)
	if err != nil || currentVersionID == "" {
		return 0, nil //nolint:nilerr
	}
	return r.q.CountVariantsByVersion(ctx, currentVersionID)
}

func (r *assetRepo) IsRebuildingVariants(ctx context.Context, versionID string) (bool, error) {
	var count int64
	err := r.sqlDB.QueryRowContext(ctx,
		`SELECT COUNT(*) FROM jobs
		 WHERE type = 'rebuild_variants'
		   AND JSON_EXTRACT(payload, '$.new_version_id') = ?
		   AND status IN ('pending', 'processing')`,
		versionID,
	).Scan(&count)
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *assetRepo) ListComments(ctx context.Context, assetID string) ([]repository.AssetComment, error) {
	rows, err := r.q.ListCommentsOnAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.AssetComment, len(rows))
	for i, c := range rows {
		createdAt, _ := time.Parse("2006-01-02 15:04:05", c.CreatedAt)
		out[i] = repository.AssetComment{
			ID:          c.ID,
			AssetID:     assetID,
			ShareID:     c.ShareID,
			AuthorName:  c.AuthorName,
			AuthorEmail: c.AuthorEmail,
			Body:        c.Body,
			CreatedAt:   createdAt,
		}
	}
	return out, nil
}

func (r *assetRepo) SetProject(ctx context.Context, workspaceID, assetID string, projectID *string) error {
	return r.q.UpdateAssetProject(ctx, dbgen.UpdateAssetProjectParams{
		ID:          assetID,
		WorkspaceID: workspaceID,
		ProjectID:   projectID,
	})
}

func (r *assetRepo) BatchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return r.batchVersionCounts(ctx, assetIDs)
}

func (r *assetRepo) BatchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	return r.batchVariantCounts(ctx, assetIDs)
}

// batchVersionCounts returns version counts for a slice of asset IDs.
func (r *assetRepo) batchVersionCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	query := fmt.Sprintf(
		`SELECT asset_id, COUNT(*) FROM asset_versions WHERE deleted_at IS NULL AND asset_id IN (%s) GROUP BY asset_id`,
		batchCountPlaceholders(assetIDs),
	)
	return r.batchCounts(ctx, assetIDs, query)
}

// batchVariantCounts returns variant counts (on current version) for a slice of asset IDs.
func (r *assetRepo) batchVariantCounts(ctx context.Context, assetIDs []string) (map[string]int64, error) {
	query := fmt.Sprintf(
		`SELECT av.asset_id, COUNT(v.id)
		   FROM asset_versions av
		   JOIN variants v ON v.asset_version_id = av.id
		  WHERE av.is_current = 1 AND av.asset_id IN (%s)
		  GROUP BY av.asset_id`,
		batchCountPlaceholders(assetIDs),
	)
	return r.batchCounts(ctx, assetIDs, query)
}

func batchCountPlaceholders(assetIDs []string) string {
	placeholders := make([]string, len(assetIDs))
	for i := range assetIDs {
		placeholders[i] = "?"
	}
	return strings.Join(placeholders, ",")
}

func (r *assetRepo) batchCounts(ctx context.Context, assetIDs []string, query string) (map[string]int64, error) {
	counts := make(map[string]int64, len(assetIDs))
	if len(assetIDs) == 0 {
		return counts, nil
	}
	args := make([]any, len(assetIDs))
	for i, id := range assetIDs {
		args[i] = id
	}
	rows, err := r.sqlDB.QueryContext(ctx, query, args...)
	if err != nil {
		return counts, err
	}
	defer rows.Close()
	for rows.Next() {
		var id string
		var n int64
		if err := rows.Scan(&id, &n); err == nil {
			counts[id] = n
		}
	}
	return counts, rows.Err()
}
