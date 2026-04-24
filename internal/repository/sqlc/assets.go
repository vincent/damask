package reposqlc

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"

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
