package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type assetRepo struct {
	q *dbgen.Queries
}

// NewAssetRepo returns a repository.AssetRepository backed by sqlc-generated queries.
func NewAssetRepo(q *dbgen.Queries) repository.AssetRepository {
	return &assetRepo{q: q}
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
