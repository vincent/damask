package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type variantRepo struct {
	sqlDB *sql.DB
}

// NewVariantRepo returns a repository.VariantRepository backed by sqlc-generated queries.
func NewVariantRepo(_ *dbgen.Queries, sqlDB *sql.DB) repository.VariantRepository {
	return &variantRepo{sqlDB: sqlDB}
}

func (r *variantRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Variant, error) {
	row := r.sqlDB.QueryRowContext(ctx, `
		SELECT id, workspace_id, asset_version_id, type, storage_key, transform_params, size,
		       status, thumbnail_key, thumbnail_content_type, created_at
		FROM variants
		WHERE id = ? AND workspace_id = ?`, id, workspaceID)
	return scanVariant(row)
}

func (r *variantRepo) ListByAsset(ctx context.Context, _ string, assetID string) ([]repository.Variant, error) {
	rows, err := r.sqlDB.QueryContext(ctx, `
		SELECT v.id, v.workspace_id, v.asset_version_id, v.type, v.storage_key, v.transform_params, v.size,
		       v.status, v.thumbnail_key, v.thumbnail_content_type, v.created_at
		FROM variants v
		JOIN asset_versions av ON av.id = v.asset_version_id
		WHERE av.asset_id = ? AND av.is_current = 1
		ORDER BY v.created_at DESC`, assetID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []repository.Variant
	for rows.Next() {
		var v repository.Variant
		if err := rows.Scan(
			&v.ID, &v.WorkspaceID, &v.AssetVersionID, &v.Type, &v.StorageKey, &v.TransformParams, &v.Size,
			&v.Status, &v.ThumbnailKey, &v.ThumbnailContentType, &v.CreatedAt,
		); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (r *variantRepo) Create(ctx context.Context, v repository.Variant) (repository.Variant, error) {
	status := v.Status
	if status == "" {
		status = "ready"
	}
	row := r.sqlDB.QueryRowContext(ctx, `
		INSERT INTO variants (id, workspace_id, asset_version_id, type, storage_key, transform_params, size, status)
		VALUES (?, ?, ?, ?, ?, ?, ?, ?)
		RETURNING id, workspace_id, asset_version_id, type, storage_key, transform_params, size,
		          status, thumbnail_key, thumbnail_content_type, created_at`,
		v.ID, v.WorkspaceID, v.AssetVersionID, v.Type, v.StorageKey, v.TransformParams, v.Size, status,
	)
	return scanVariant(row)
}

func (r *variantRepo) Delete(ctx context.Context, workspaceID, id string) error {
	_, err := r.sqlDB.ExecContext(ctx, `DELETE FROM variants WHERE id = ? AND workspace_id = ?`, id, workspaceID)
	return err
}

func scanVariant(row interface {
	Scan(dest ...any) error
}) (repository.Variant, error) {
	var v repository.Variant
	err := row.Scan(
		&v.ID, &v.WorkspaceID, &v.AssetVersionID, &v.Type, &v.StorageKey, &v.TransformParams, &v.Size,
		&v.Status, &v.ThumbnailKey, &v.ThumbnailContentType, &v.CreatedAt,
	)
	if errors.Is(err, sql.ErrNoRows) {
		return repository.Variant{}, apperr.ErrNotFound
	}
	return v, err
}
