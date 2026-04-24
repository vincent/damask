package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type workspaceRepo struct {
	q *dbgen.Queries
}

// NewWorkspaceRepo returns a repository.WorkspaceRepository backed by sqlc-generated queries.
func NewWorkspaceRepo(q *dbgen.Queries) repository.WorkspaceRepository {
	return &workspaceRepo{q: q}
}

func (r *workspaceRepo) GetByID(ctx context.Context, id string) (repository.Workspace, error) {
	row, err := r.q.GetWorkspaceByID(ctx, id)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Workspace{}, apperr.ErrNotFound
		}
		return repository.Workspace{}, err
	}
	return toWorkspace(row), nil
}

// Update applies exif + version-retention settings. Other workspace fields (name, icon)
// are updated via separate targeted sqlc queries in the handlers until those handlers
// are fully migrated to a service.
func (r *workspaceRepo) Update(ctx context.Context, w repository.Workspace) (repository.Workspace, error) {
	if err := r.q.UpdateWorkspaceExifSettings(ctx, dbgen.UpdateWorkspaceExifSettingsParams{
		ID:          w.ID,
		ExifKeep:    boolToInt64(w.ExifKeep),
		ExifKeepGps: boolToInt64(w.ExifKeepGps),
	}); err != nil {
		return repository.Workspace{}, err
	}
	if err := r.q.UpdateWorkspaceVersionRetention(ctx, dbgen.UpdateWorkspaceVersionRetentionParams{
		ID:                    w.ID,
		VersionRetentionCount: w.VersionRetentionCount,
	}); err != nil {
		return repository.Workspace{}, err
	}
	return r.GetByID(ctx, w.ID)
}

func toWorkspace(w dbgen.Workspace) repository.Workspace {
	return repository.Workspace{
		ID:                       w.ID,
		Name:                     w.Name,
		IngestToken:              w.IngestToken,
		VersionRetentionCount:    w.VersionRetentionCount,
		EventLogRetentionDays:    w.EventLogRetentionDays,
		DownloadLogRetentionDays: w.DownloadLogRetentionDays,
		IconAssetID:              w.IconAssetID,
		IconVersionID:            w.IconVersionID,
		ExifKeep:                 w.ExifKeep != 0,
		ExifKeepGps:              w.ExifKeepGps != 0,
		CreatedAt:                w.CreatedAt,
		UpdatedAt:                w.UpdatedAt,
	}
}
