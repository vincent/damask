package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

type ProjectRepo struct {
	mu       sync.RWMutex
	projects map[string]repository.Project
}

func NewProjectRepo() *ProjectRepo {
	return &ProjectRepo{projects: make(map[string]repository.Project)}
}

func (r *ProjectRepo) Seed(projects ...repository.Project) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, p := range projects {
		r.projects[p.ID] = p
	}
}

func (r *ProjectRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Project, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	p, ok := r.projects[id]
	if !ok || p.WorkspaceID != workspaceID {
		return repository.Project{}, fmt.Errorf("project %q: %w", id, apperr.ErrNotFound)
	}
	return p, nil
}

func (r *ProjectRepo) List(ctx context.Context, workspaceID string) ([]repository.ProjectWithCount, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.ProjectWithCount
	for _, p := range r.projects {
		if p.WorkspaceID == workspaceID {
			out = append(out, repository.ProjectWithCount{Project: p})
		}
	}
	return out, nil
}

func (r *ProjectRepo) NullifyAssets(_ context.Context, _, _ string) error { return nil }

func (r *ProjectRepo) Create(ctx context.Context, p repository.Project) (repository.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.projects[p.ID] = p
	return p, nil
}

func (r *ProjectRepo) Update(ctx context.Context, p repository.Project) (repository.Project, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.projects[p.ID]
	if !ok || existing.WorkspaceID != p.WorkspaceID {
		return repository.Project{}, fmt.Errorf("project %q: %w", p.ID, apperr.ErrNotFound)
	}
	r.projects[p.ID] = p
	return p, nil
}

func (r *ProjectRepo) Delete(ctx context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	p, ok := r.projects[id]
	if !ok || p.WorkspaceID != workspaceID {
		return fmt.Errorf("project %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.projects, id)
	return nil
}
