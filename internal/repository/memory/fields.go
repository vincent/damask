package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealFieldRepo is a map-backed FieldRepository for unit tests.
type RealFieldRepo struct {
	mu     sync.RWMutex
	fields map[string]repository.FieldDefinition
}

func NewRealFieldRepo() *RealFieldRepo {
	return &RealFieldRepo{fields: make(map[string]repository.FieldDefinition)}
}

func (r *RealFieldRepo) GetByID(_ context.Context, workspaceID, id string) (repository.FieldDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	f, ok := r.fields[id]
	if !ok || f.WorkspaceID != workspaceID {
		return repository.FieldDefinition{}, fmt.Errorf("field %q: %w", id, apperr.ErrNotFound)
	}
	return f, nil
}

func (r *RealFieldRepo) List(_ context.Context, workspaceID, scope string) ([]repository.FieldDefinition, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.FieldDefinition
	for _, f := range r.fields {
		if f.WorkspaceID == workspaceID && f.Scope == scope && f.DeletedAt == nil {
			out = append(out, f)
		}
	}
	return out, nil
}

func (r *RealFieldRepo) Create(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	// Check unique key within (workspace, scope)
	for _, existing := range r.fields {
		if existing.WorkspaceID == f.WorkspaceID && existing.Scope == f.Scope && existing.Key == f.Key && existing.DeletedAt == nil {
			return repository.FieldDefinition{}, fmt.Errorf("UNIQUE constraint failed: field_definitions.key")
		}
	}
	r.fields[f.ID] = f
	return f, nil
}

func (r *RealFieldRepo) Update(_ context.Context, f repository.FieldDefinition) (repository.FieldDefinition, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.fields[f.ID]
	if !ok || existing.WorkspaceID != f.WorkspaceID {
		return repository.FieldDefinition{}, fmt.Errorf("field %q: %w", f.ID, apperr.ErrNotFound)
	}
	r.fields[f.ID] = f
	return f, nil
}

func (r *RealFieldRepo) SoftDelete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.fields[id]
	if !ok || f.WorkspaceID != workspaceID {
		return fmt.Errorf("field %q: %w", id, apperr.ErrNotFound)
	}
	now := "deleted"
	f.DeletedAt = &now
	r.fields[id] = f
	return nil
}

func (r *RealFieldRepo) CountByWorkspaceAndScope(_ context.Context, workspaceID, scope string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, f := range r.fields {
		if f.WorkspaceID == workspaceID && f.Scope == scope && f.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *RealFieldRepo) CountAssetValues(_ context.Context, fieldID string) (int64, error) {
	return 0, nil
}

func (r *RealFieldRepo) CountProjectValues(_ context.Context, fieldID string) (int64, error) {
	return 0, nil
}

func (r *RealFieldRepo) UpdatePosition(_ context.Context, workspaceID, id string, position int64) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	f, ok := r.fields[id]
	if !ok || f.WorkspaceID != workspaceID {
		return nil
	}
	f.Position = position
	r.fields[id] = f
	return nil
}
