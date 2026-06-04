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

func (r *AssetRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	a, ok := r.assets[id]
	if !ok || a.WorkspaceID != workspaceID {
		return repository.Asset{}, fmt.Errorf("asset %q: %w", id, apperr.ErrNotFound)
	}
	return a, nil
}

func (r *AssetRepo) List(_ context.Context, params repository.ListAssetsParams) ([]repository.Asset, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var similarAllow map[string]struct{}
	if params.SimilarToIDs != nil {
		similarAllow = make(map[string]struct{}, len(params.SimilarToIDs))
		for _, id := range params.SimilarToIDs {
			similarAllow[id] = struct{}{}
		}
	}
	var out []repository.Asset
	for _, a := range r.assets {
		if a.WorkspaceID != params.WorkspaceID {
			continue
		}
		if similarAllow != nil {
			if _, ok := similarAllow[a.ID]; !ok {
				continue
			}
		}
		if params.FolderID != nil && (a.FolderID == nil || *a.FolderID != *params.FolderID) {
			continue
		}
		if params.FolderIsRoot && a.FolderID != nil {
			continue
		}
		if params.ProjectID != nil && (a.ProjectID == nil || *a.ProjectID != *params.ProjectID) {
			continue
		}
		out = append(out, a)
	}
	return out, nil
}

func (r *AssetRepo) Create(_ context.Context, params repository.CreateAssetParams) (repository.Asset, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	a := repository.Asset{
		ID:                   params.ID,
		WorkspaceID:          params.WorkspaceID,
		ProjectID:            params.ProjectID,
		FolderID:             params.FolderID,
		DerivedFromAssetID:   params.DerivedFromAssetID,
		OriginalFilename:     params.OriginalFilename,
		StorageKey:           params.StorageKey,
		MimeType:             params.MimeType,
		Size:                 params.Size,
		Width:                params.Width,
		Height:               params.Height,
		ThumbnailKey:         params.ThumbnailKey,
		ThumbnailContentType: params.ThumbnailContentType,
		Metadata:             params.Metadata,
	}
	r.assets[a.ID] = a
	return a, nil
}

func (r *AssetRepo) Update(_ context.Context, params repository.UpdateAssetParams) (repository.Asset, error) {
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

func (r *AssetRepo) SoftDelete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[id]
	if !ok || a.WorkspaceID != workspaceID {
		return fmt.Errorf("asset %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.assets, id)
	return nil
}

func (r *AssetRepo) IsProjectCover(_ context.Context, _, _ string) (bool, error)  { return false, nil }
func (r *AssetRepo) IsWorkspaceIcon(_ context.Context, _, _ string) (bool, error) { return false, nil }

func (r *AssetRepo) RefreshFTS(_ context.Context, _ string) error { return nil }

func (r *AssetRepo) ListByFields(
	_ context.Context,
	_ repository.ListAssetsByFieldsParams,
) ([]repository.Asset, error) {
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

func (r *AssetRepo) CollectStorageKeys(_ context.Context, _, _ string) (repository.AssetStorageKeys, error) {
	return repository.AssetStorageKeys{}, nil
}
func (r *AssetRepo) HardDelete(_ context.Context, workspaceID, id string) error {
	return r.SoftDelete(context.Background(), workspaceID, id)
}
func (r *AssetRepo) CountVersionsByAsset(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (r *AssetRepo) CountVariantsByCurrentVersion(_ context.Context, _ string) (int64, error) {
	return 0, nil
}
func (r *AssetRepo) IsRebuildingVariants(_ context.Context, _ string) (bool, error) {
	return false, nil
}
func (r *AssetRepo) ListComments(_ context.Context, _ string) ([]repository.AssetComment, error) {
	return nil, nil
}
func (r *AssetRepo) BatchVersionCounts(_ context.Context, ids []string) (map[string]int64, error) {
	m := make(map[string]int64, len(ids))
	for _, id := range ids {
		m[id] = 0
	}
	return m, nil
}
func (r *AssetRepo) BatchVariantCounts(_ context.Context, ids []string) (map[string]int64, error) {
	m := make(map[string]int64, len(ids))
	for _, id := range ids {
		m[id] = 0
	}
	return m, nil
}
func (r *AssetRepo) SetProject(_ context.Context, workspaceID, assetID string, projectID *string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	a, ok := r.assets[assetID]
	if !ok || a.WorkspaceID != workspaceID {
		return fmt.Errorf("asset %q: %w", assetID, apperr.ErrNotFound)
	}
	a.ProjectID = projectID
	r.assets[assetID] = a
	return nil
}
