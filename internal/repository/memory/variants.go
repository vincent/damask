package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealVariantRepo is a map-backed VariantRepository for unit tests.
type RealVariantRepo struct {
	mu       sync.RWMutex
	variants map[string]repository.Variant // key: id
}

func NewRealVariantRepo() *RealVariantRepo {
	return &RealVariantRepo{variants: make(map[string]repository.Variant)}
}

func (r *RealVariantRepo) Seed(variants ...repository.Variant) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range variants {
		r.variants[v.ID] = v
	}
}

func (r *RealVariantRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Variant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.variants[id]
	if !ok || v.WorkspaceID != workspaceID {
		return repository.Variant{}, fmt.Errorf("variant %q: %w", id, apperr.ErrNotFound)
	}
	return v, nil
}

func (r *RealVariantRepo) ListByAsset(_ context.Context, workspaceID, assetID string) ([]repository.Variant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	// In tests we just filter by workspaceID -- without a join to the asset table
	// we can't replicate "current version only", so return all for the workspace.
	var out []repository.Variant
	for _, v := range r.variants {
		if v.WorkspaceID == workspaceID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *RealVariantRepo) Create(_ context.Context, v repository.Variant) (repository.Variant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.variants[v.ID] = v
	return v, nil
}

func (r *RealVariantRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.variants[id]
	if !ok || v.WorkspaceID != workspaceID {
		return fmt.Errorf("variant %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.variants, id)
	return nil
}
