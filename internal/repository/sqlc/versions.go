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
	q *dbgen.Queries
}

// NewVersionRepo returns a repository.VersionRepository backed by sqlc-generated queries.
func NewVersionRepo(q *dbgen.Queries) repository.VersionRepository {
	return &versionRepo{q: q}
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
