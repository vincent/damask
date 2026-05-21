package memory

import (
	"context"
	"fmt"
	"slices"
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

func (r *RealVariantRepo) ListByAsset(_ context.Context, workspaceID, _ string) ([]repository.Variant, error) {
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
	slices.SortFunc(out, func(a, b repository.Variant) int {
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		if a.ID < b.ID {
			return -1
		}
		if a.ID > b.ID {
			return 1
		}
		return 0
	})
	return out, nil
}

func (r *RealVariantRepo) Create(_ context.Context, v repository.Variant) (repository.Variant, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if v.Status == "" {
		v.Status = "ready"
	}
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

func (r *RealVariantRepo) UpdateTitle(_ context.Context, workspaceID, variantID string, title *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.variants[variantID]
	if !ok || v.WorkspaceID != workspaceID {
		return fmt.Errorf("variant %q: %w", variantID, apperr.ErrNotFound)
	}
	v.Title = title
	r.variants[variantID] = v
	return nil
}

func (r *RealVariantRepo) UpdateSharedBatch(_ context.Context, workspaceID string, ids []string, isShared bool) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range ids {
		v, ok := r.variants[id]
		if !ok || v.WorkspaceID != workspaceID {
			continue
		}
		v.IsShared = isShared
		r.variants[id] = v
	}
	return nil
}

func (r *RealVariantRepo) ListSharedByAssetIDs(
	_ context.Context,
	assetIDs []string,
) ([]repository.VariantWithAssetID, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	if len(assetIDs) == 0 {
		return nil, nil
	}
	assetSet := make(map[string]struct{}, len(assetIDs))
	for _, id := range assetIDs {
		assetSet[id] = struct{}{}
	}
	out := make([]repository.VariantWithAssetID, 0)
	for _, v := range r.variants {
		if !v.IsShared {
			continue
		}
		if _, ok := assetSet[v.AssetVersionID]; !ok {
			continue
		}
		out = append(out, repository.VariantWithAssetID{Variant: v, AssetID: v.AssetVersionID})
	}
	slices.SortFunc(out, func(a, b repository.VariantWithAssetID) int {
		if a.AssetID < b.AssetID {
			return -1
		}
		if a.AssetID > b.AssetID {
			return 1
		}
		if a.CreatedAt.Before(b.CreatedAt) {
			return -1
		}
		if a.CreatedAt.After(b.CreatedAt) {
			return 1
		}
		return 0
	})
	return out, nil
}

func (r *RealVariantRepo) GetSharedByVariantAndAsset(
	_ context.Context,
	variantID, assetID string,
) (repository.Variant, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.variants[variantID]
	if !ok || !v.IsShared || v.AssetVersionID != assetID {
		return repository.Variant{}, fmt.Errorf("variant %q: %w", variantID, apperr.ErrNotFound)
	}
	return v, nil
}
