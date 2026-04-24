package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// AssetRepo is an in-memory implementation of repository.AssetRepository.
// It is safe for concurrent use and intended for use in unit tests only.
type AssetRepo struct {
	mu     sync.RWMutex
	assets map[string]repository.Asset // keyed by id
}

// NewAssetRepo returns an empty AssetRepo.
func NewAssetRepo() *AssetRepo {
	return &AssetRepo{assets: make(map[string]repository.Asset)}
}

// Seed pre-populates the repo with the given assets. Call before the test runs.
func (r *AssetRepo) Seed(assets ...repository.Asset) {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, a := range assets {
		r.assets[a.ID] = a
	}
}

func (r *AssetRepo) GetByID(ctx context.Context, workspaceID, id string) (repository.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.assets[id]
	if !ok || a.WorkspaceID != workspaceID {
		return repository.Asset{}, fmt.Errorf("asset %q: %w", id, apperr.ErrNotFound)
	}
	return a, nil
}

func (r *AssetRepo) List(ctx context.Context, params repository.ListAssetsParams) ([]repository.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Asset
	for _, a := range r.assets {
		if a.WorkspaceID != params.WorkspaceID {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *AssetRepo) Create(ctx context.Context, params repository.CreateAssetParams) (repository.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := repository.Asset{
		ID:               params.ID,
		WorkspaceID:      params.WorkspaceID,
		ProjectID:        params.ProjectID,
		OriginalFilename: params.OriginalFilename,
		StorageKey:       params.StorageKey,
		MimeType:         params.MimeType,
		Size:             params.Size,
		Width:            params.Width,
		Height:           params.Height,
		Metadata:         params.Metadata,
	}
	r.assets[a.ID] = a
	return a, nil
}

func (r *AssetRepo) Update(ctx context.Context, params repository.UpdateAssetParams) (repository.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[params.ID]
	if !ok || a.WorkspaceID != params.WorkspaceID {
		return repository.Asset{}, fmt.Errorf("asset %q: %w", params.ID, apperr.ErrNotFound)
	}
	if params.OriginalFilename != nil {
		a.OriginalFilename = *params.OriginalFilename
	}
	if params.FolderID != nil {
		a.FolderID = params.FolderID
	}
	if params.ProjectID != nil {
		a.ProjectID = params.ProjectID
	}
	if params.ThumbnailKey != nil {
		a.ThumbnailKey = params.ThumbnailKey
	}
	if params.CurrentVersionID != nil {
		a.CurrentVersionID = params.CurrentVersionID
	}
	if params.Width != nil {
		a.Width = params.Width
	}
	if params.Height != nil {
		a.Height = params.Height
	}
	r.assets[a.ID] = a
	return a, nil
}

func (r *AssetRepo) SoftDelete(ctx context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[id]
	if !ok || a.WorkspaceID != workspaceID {
		return fmt.Errorf("asset %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.assets, id)
	return nil
}

func (r *AssetRepo) IsProjectCover(_ context.Context, _, _ string) (bool, error) { return false, nil }
func (r *AssetRepo) IsWorkspaceIcon(_ context.Context, _, _ string) (bool, error) { return false, nil }

func (r *AssetRepo) RefreshFTS(_ context.Context, _ string) error { return nil }

func (r *AssetRepo) ListByFields(_ context.Context, params repository.ListAssetsByFieldsParams) ([]repository.Asset, error) {
	return nil, nil
}

func (r *AssetRepo) CountByIDs(_ context.Context, workspaceID string, ids []string) (int64, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	set := make(map[string]struct{}, len(ids))
	for _, id := range ids {
		set[id] = struct{}{}
	}
	var count int64
	for _, a := range r.assets {
		if a.WorkspaceID == workspaceID {
			if _, ok := set[a.ID]; ok {
				count++
			}
		}
	}
	return count, nil
}
