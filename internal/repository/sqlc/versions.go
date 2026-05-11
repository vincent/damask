package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type versionRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewVersionRepo returns a repository.VersionRepository backed by sqlc-generated queries.
func NewVersionRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.VersionRepository {
	return &versionRepo{q: q, sqlDB: sqlDB}
}

func (r *versionRepo) GetByID(ctx context.Context, id string) (repository.AssetVersion, error) {
	row, err := r.q.GetVersionByIDUnchecked(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) ListByAsset(ctx context.Context, assetID string) ([]repository.AssetVersion, error) {
	rows, err := r.q.ListVersions(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.AssetVersion, len(rows))
	for i, row := range rows {
		out[i] = toVersion(row)
	}
	return out, nil
}

func (r *versionRepo) Create(ctx context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	row, err := r.q.CreateAssetVersion(ctx, dbgen.CreateAssetVersionParams{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
	})
	if err != nil {
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetCurrentByAsset(ctx context.Context, assetID string) (repository.AssetVersion, error) {
	row, err := r.q.GetCurrentVersion(ctx, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetFirstByAsset(ctx context.Context, assetID string) (repository.AssetVersion, error) {
	row, err := r.q.GetFirstVersion(ctx, assetID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) GetByIDForWorkspace(ctx context.Context, workspaceID, id string) (repository.AssetVersion, error) {
	row, err := r.q.GetVersionByID(ctx, dbgen.GetVersionByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) SoftDelete(ctx context.Context, id string) error {
	return r.q.SoftDeleteVersion(ctx, id)
}

func (r *versionRepo) Delete(ctx context.Context, id string) error {
	return r.q.HardDeleteVersion(ctx, id)
}

func (r *versionRepo) IsReferencedAsCover(ctx context.Context, versionID string) (bool, error) {
	count, err := r.q.IsVersionReferencedAsCover(ctx, dbgen.IsVersionReferencedAsCoverParams{
		CoverVersionID: &versionID,
		IconVersionID:  &versionID,
	})
	if err != nil {
		return false, err
	}
	return count > 0, nil
}

func (r *versionRepo) CountByAsset(ctx context.Context, assetID string) (int64, error) {
	return r.q.CountActiveVersions(ctx, assetID)
}

func (r *versionRepo) GetByHash(ctx context.Context, assetID, contentHash string) (repository.AssetVersion, error) {
	row, err := r.q.GetVersionByHash(ctx, dbgen.GetVersionByHashParams{
		AssetID:     assetID,
		ContentHash: contentHash,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.AssetVersion{}, apperr.ErrNotFound
		}
		return repository.AssetVersion{}, err
	}
	return toVersion(row), nil
}

func (r *versionRepo) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	var maxNum sql.NullInt64
	err := r.sqlDB.QueryRowContext(ctx,
		`SELECT MAX(version_num) FROM asset_versions WHERE asset_id = ?`, assetID,
	).Scan(&maxNum)
	if err != nil {
		return 0, err
	}
	return maxNum.Int64 + 1, nil
}

func (r *versionRepo) SetCurrent(ctx context.Context, assetID, versionID string) error {
	tx, err := r.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck
	qtx := r.q.WithTx(tx)
	if err := qtx.ClearCurrentVersionFlags(ctx, assetID); err != nil {
		return err
	}
	if err := qtx.SetCurrentVersionFlag(ctx, versionID); err != nil {
		return err
	}
	if err := qtx.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               assetID,
	}); err != nil {
		return err
	}
	return tx.Commit()
}

func (r *versionRepo) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return r.q.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: key,
		ID:           assetID,
	})
}

func (r *versionRepo) ListWithVariantCount(ctx context.Context, assetID string) ([]repository.AssetVersionWithCount, error) {
	rows, err := r.q.ListVersionsWithVariantCount(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.AssetVersionWithCount, len(rows))
	for i, row := range rows {
		out[i] = repository.AssetVersionWithCount{
			AssetVersion: repository.AssetVersion{
				ID:           row.ID,
				AssetID:      row.AssetID,
				WorkspaceID:  row.WorkspaceID,
				VersionNum:   row.VersionNum,
				StorageKey:   row.StorageKey,
				ContentHash:  row.ContentHash,
				MimeType:     row.MimeType,
				Size:         row.Size,
				Width:        row.Width,
				Height:       row.Height,
				DurationSec:  row.DurationSec,
				ThumbnailKey: row.ThumbnailKey,
				Comment:      row.Comment,
				CreatedBy:    row.CreatedBy,
				CreatedAt:    parseVersionTime(row.CreatedAt),
				IsCurrent:    row.IsCurrent != 0,
				DeletedAt:    row.DeletedAt,
			},
			VariantCount: row.VariantCount,
		}
	}
	return out, nil
}

func toVersion(v dbgen.AssetVersion) repository.AssetVersion {
	return repository.AssetVersion{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
		CreatedAt:    parseVersionTime(v.CreatedAt),
		IsCurrent:    v.IsCurrent != 0,
		DeletedAt:    v.DeletedAt,
	}
}

func parseVersionTime(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
