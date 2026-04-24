package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealWorkspaceRepo is a map-backed WorkspaceRepository for unit tests.
type RealWorkspaceRepo struct {
	mu         sync.RWMutex
	workspaces map[string]repository.Workspace
}

func NewRealWorkspaceRepo() *RealWorkspaceRepo {
	return &RealWorkspaceRepo{workspaces: make(map[string]repository.Workspace)}
}

func (r *RealWorkspaceRepo) Seed(workspaces ...repository.Workspace) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, ws := range workspaces {
		r.workspaces[ws.ID] = ws
	}
}

func (r *RealWorkspaceRepo) GetByID(_ context.Context, id string) (repository.Workspace, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	ws, ok := r.workspaces[id]
	if !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", id, apperr.ErrNotFound)
	}
	return ws, nil
}

func (r *RealWorkspaceRepo) Update(_ context.Context, ws repository.Workspace) (repository.Workspace, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.workspaces[ws.ID]; !ok {
		return repository.Workspace{}, fmt.Errorf("workspace %q: %w", ws.ID, apperr.ErrNotFound)
	}
	r.workspaces[ws.ID] = ws
	return ws, nil
}
