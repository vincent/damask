package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealVersionRepo is a map-backed VersionRepository for unit tests.
type RealVersionRepo struct {
	mu            sync.RWMutex
	versions      map[string]repository.AssetVersion
	coverVersions map[string]bool // versionID -> is referenced as cover
}

func NewRealVersionRepo() *RealVersionRepo {
	return &RealVersionRepo{
		versions:      make(map[string]repository.AssetVersion),
		coverVersions: make(map[string]bool),
	}
}

func (r *RealVersionRepo) Seed(versions ...repository.AssetVersion) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, v := range versions {
		r.versions[v.ID] = v
	}
}

// MarkAsCover marks a version as referenced by a project cover (for testing conflict checks).
func (r *RealVersionRepo) MarkAsCover(versionID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.coverVersions[versionID] = true
}

func (r *RealVersionRepo) GetByID(_ context.Context, id string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.versions[id]
	if !ok {
		return repository.AssetVersion{}, fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	return v, nil
}

func (r *RealVersionRepo) GetCurrentByAsset(_ context.Context, assetID string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	for _, v := range r.versions {
		if v.AssetID == assetID && v.IsCurrent && v.DeletedAt == nil {
			return v, nil
		}
	}
	return repository.AssetVersion{}, fmt.Errorf("no current version for asset %q: %w", assetID, apperr.ErrNotFound)
}

func (r *RealVersionRepo) GetByIDForWorkspace(_ context.Context, workspaceID, id string) (repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	v, ok := r.versions[id]
	if !ok || v.WorkspaceID != workspaceID {
		return repository.AssetVersion{}, fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	return v, nil
}

func (r *RealVersionRepo) ListByAsset(_ context.Context, assetID string) ([]repository.AssetVersion, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.AssetVersion
	for _, v := range r.versions {
		if v.AssetID == assetID {
			out = append(out, v)
		}
	}
	return out, nil
}

func (r *RealVersionRepo) Create(_ context.Context, v repository.AssetVersion) (repository.AssetVersion, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.versions[v.ID] = v
	return v, nil
}

func (r *RealVersionRepo) SoftDelete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	v, ok := r.versions[id]
	if !ok {
		return fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	now := "deleted"
	v.DeletedAt = &now
	r.versions[id] = v
	return nil
}

func (r *RealVersionRepo) Delete(_ context.Context, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, ok := r.versions[id]; !ok {
		return fmt.Errorf("version %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.versions, id)
	return nil
}

func (r *RealVersionRepo) CountByAsset(_ context.Context, assetID string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var count int64
	for _, v := range r.versions {
		if v.AssetID == assetID && v.DeletedAt == nil {
			count++
		}
	}
	return count, nil
}

func (r *RealVersionRepo) IsReferencedAsCover(_ context.Context, versionID string) (bool, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	return r.coverVersions[versionID], nil
}
