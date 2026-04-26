package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"

	"github.com/google/uuid"
)

// RealFolderRepo is a map-backed FolderRepository for unit tests.
type RealFolderRepo struct {
	mu      sync.RWMutex
	folders map[string]repository.Folder
}

func NewRealFolderRepo() *RealFolderRepo {
	return &RealFolderRepo{folders: make(map[string]repository.Folder)}
}

func (r *RealFolderRepo) Seed(folders ...repository.Folder) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, f := range folders {
		r.folders[f.ID] = f
	}
}

func (r *RealFolderRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Folder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.folders[id]
	if !ok || f.WorkspaceID != workspaceID {
		return repository.Folder{}, fmt.Errorf("folder %q: %w", id, apperr.ErrNotFound)
	}
	return f, nil
}

func (r *RealFolderRepo) ListByProject(_ context.Context, workspaceID, projectID string) ([]repository.Folder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Folder
	for _, f := range r.folders {
		if f.WorkspaceID == workspaceID && f.ProjectID == projectID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *RealFolderRepo) Create(_ context.Context, f repository.Folder) (repository.Folder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if f.ID == "" {
		f.ID = uuid.NewString()
	}
	r.folders[f.ID] = f
	return f, nil
}

func (r *RealFolderRepo) Update(_ context.Context, f repository.Folder) (repository.Folder, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.folders[f.ID]
	if !ok || existing.WorkspaceID != f.WorkspaceID {
		return repository.Folder{}, fmt.Errorf("folder %q: %w", f.ID, apperr.ErrNotFound)
	}
	r.folders[f.ID] = f
	return f, nil
}

func (r *RealFolderRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.folders[id]
	if !ok || f.WorkspaceID != workspaceID {
		return fmt.Errorf("folder %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.folders, id)
	return nil
}

func (r *RealFolderRepo) GetChildren(_ context.Context, workspaceID, parentID string) ([]repository.Folder, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Folder
	for _, f := range r.folders {
		if f.WorkspaceID == workspaceID && f.ParentID != nil && *f.ParentID == parentID {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *RealFolderRepo) NullifyAssets(_ context.Context, _, _ string) error { return nil }

func (r *RealFolderRepo) ListTree(_ context.Context, workspaceID, projectID string) ([]repository.FolderTree, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var roots []repository.FolderTree
	for _, f := range r.folders {
		if f.WorkspaceID == workspaceID && f.ProjectID == projectID && f.ParentID == nil {
			roots = append(roots, repository.FolderTree{Folder: f})
		}
	}
	return roots, nil
}
