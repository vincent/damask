package reposqlc

import (
	"context"
	"database/sql"
	"errors"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

type projectRepo struct {
	q *dbgen.Queries
}

// NewProjectRepo returns a repository.ProjectRepository backed by sqlc-generated queries.
func NewProjectRepo(q *dbgen.Queries) repository.ProjectRepository {
	return &projectRepo{q: q}
}

func (r *projectRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Project, error) {
	row, err := r.q.GetProjectByID(ctx, dbgen.GetProjectByIDParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Project{}, apperr.ErrNotFound
		}
		return repository.Project{}, err
	}
	return toProject(row), nil
}

func (r *projectRepo) List(ctx context.Context, workspaceID string) ([]repository.ProjectWithCount, error) {
	rows, err := r.q.ListProjectsWithCount(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.ProjectWithCount, len(rows))
	for i, row := range rows {
		out[i] = repository.ProjectWithCount{
			Project: repository.Project{
				ID:             row.ID,
				WorkspaceID:    row.WorkspaceID,
				Name:           row.Name,
				Description:    row.Description,
				Color:          row.Color,
				CoverAssetID:   row.CoverAssetID,
				CoverVersionID: row.CoverVersionID,
				CreatedAt:      row.CreatedAt,
				UpdatedAt:      row.UpdatedAt,
			},
			AssetCount: row.AssetCount,
		}
	}
	return out, nil
}

func (r *projectRepo) Create(ctx context.Context, p repository.Project) (repository.Project, error) {
	row, err := r.q.CreateProject(ctx, dbgen.CreateProjectParams{
		ID:           p.ID,
		WorkspaceID:  p.WorkspaceID,
		Name:         p.Name,
		Description:  p.Description,
		Color:        p.Color,
		CoverAssetID: p.CoverAssetID,
	})
	if err != nil {
		return repository.Project{}, err
	}
	return toProject(row), nil
}

func (r *projectRepo) Update(ctx context.Context, p repository.Project) (repository.Project, error) {
	row, err := r.q.UpdateProject(ctx, dbgen.UpdateProjectParams{
		ID:           p.ID,
		WorkspaceID:  p.WorkspaceID,
		Name:         &p.Name,
		Description:  p.Description,
		Color:        p.Color,
		CoverAssetID: p.CoverAssetID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.Project{}, apperr.ErrNotFound
		}
		return repository.Project{}, err
	}
	return toProject(row), nil
}

func (r *projectRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.q.DeleteProject(ctx, dbgen.DeleteProjectParams{
		ID:          id,
		WorkspaceID: workspaceID,
	})
}

func (r *projectRepo) NullifyAssets(ctx context.Context, workspaceID, projectID string) error {
	return r.q.NullifyProjectAssets(ctx, dbgen.NullifyProjectAssetsParams{
		ProjectID:   &projectID,
		WorkspaceID: workspaceID,
	})
}

func toProject(p dbgen.Project) repository.Project {
	return repository.Project{
		ID:             p.ID,
		WorkspaceID:    p.WorkspaceID,
		Name:           p.Name,
		Description:    p.Description,
		Color:          p.Color,
		CoverAssetID:   p.CoverAssetID,
		CoverVersionID: p.CoverVersionID,
		CreatedAt:      p.CreatedAt,
		UpdatedAt:      p.UpdatedAt,
	}
}
