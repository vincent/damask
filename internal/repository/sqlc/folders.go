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

type folderRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewFolderRepo returns a repository.FolderRepository backed by sqlc-generated queries.
func NewFolderRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.FolderRepository {
	return &folderRepo{q: q, sqlDB: sqlDB}
}

func (r *folderRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Folder, error) {
	row, err := r.q.GetFolderByID(ctx, dbgen.GetFolderByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Folder{}, apperr.ErrNotFound
		}
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) ListByProject(ctx context.Context, workspaceID, projectID string) ([]repository.Folder, error) {
	sqlRows, err := r.sqlDB.QueryContext(ctx,
		`SELECT id, workspace_id, project_id, parent_id, name, slug, position, created_at
		 FROM folders WHERE workspace_id = ? AND project_id = ? ORDER BY position, name`,
		workspaceID, projectID,
	)
	if err != nil {
		return nil, err
	}
	defer sqlRows.Close()

	var out []repository.Folder
	for sqlRows.Next() {
		var f dbgen.Folder
		if err := sqlRows.Scan(&f.ID, &f.WorkspaceID, &f.ProjectID, &f.ParentID, &f.Name, &f.Slug, &f.Position, &f.CreatedAt); err != nil {
			return nil, err
		}
		out = append(out, toFolder(f))
	}
	return out, sqlRows.Err()
}

func (r *folderRepo) Create(ctx context.Context, f repository.Folder) (repository.Folder, error) {
	row, err := r.q.CreateFolder(ctx, dbgen.CreateFolderParams{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Slug:        f.Slug,
	})
	if err != nil {
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) Update(ctx context.Context, f repository.Folder) (repository.Folder, error) {
	row, err := r.q.UpdateFolder(ctx, dbgen.UpdateFolderParams{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		Name:        &f.Name,
		Slug:        f.Slug,
		Position:    &f.Position,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Folder{}, apperr.ErrNotFound
		}
		return repository.Folder{}, err
	}
	return toFolder(row), nil
}

func (r *folderRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.q.DeleteFolder(ctx, dbgen.DeleteFolderParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *folderRepo) GetChildren(ctx context.Context, workspaceID, parentID string) ([]repository.Folder, error) {
	rows, err := r.q.GetFolderChildren(ctx, dbgen.GetFolderChildrenParams{
		ParentID:    &parentID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.Folder, len(rows))
	for i, row := range rows {
		out[i] = toFolder(row)
	}
	return out, nil
}

func (r *folderRepo) NullifyAssets(ctx context.Context, workspaceID, folderID string) error {
	return r.q.NullifyFolderAssets(ctx, dbgen.NullifyFolderAssetsParams{
		FolderID:    &folderID,
		WorkspaceID: workspaceID,
	})
}

func toFolder(f dbgen.Folder) repository.Folder {
	return repository.Folder{
		ID:          f.ID,
		WorkspaceID: f.WorkspaceID,
		ProjectID:   f.ProjectID,
		ParentID:    f.ParentID,
		Name:        f.Name,
		Slug:        f.Slug,
		Position:    f.Position,
		CreatedAt:   parseCreatedAt(f.CreatedAt),
	}
}

// parseCreatedAt parses SQLite time strings stored as text.
func parseCreatedAt(s string) time.Time {
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		t, _ = time.Parse("2006-01-02 15:04:05", s)
	}
	return t
}
