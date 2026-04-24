package memory

import (
	"context"
	"fmt"
	"sync"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealCollectionRepo is a map-backed CollectionRepository for unit tests.
type RealCollectionRepo struct {
	mu          sync.RWMutex
	collections map[string]repository.Collection
	assets      map[string][]string // collectionID -> []assetID
}

func NewRealCollectionRepo() *RealCollectionRepo {
	return &RealCollectionRepo{
		collections: make(map[string]repository.Collection),
		assets:      make(map[string][]string),
	}
}

func (r *RealCollectionRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Collection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	c, ok := r.collections[id]
	if !ok || c.WorkspaceID != workspaceID {
		return repository.Collection{}, fmt.Errorf("collection %q: %w", id, apperr.ErrNotFound)
	}
	return c, nil
}

func (r *RealCollectionRepo) List(_ context.Context, workspaceID string) ([]repository.Collection, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Collection
	for _, c := range r.collections {
		if c.WorkspaceID == workspaceID {
			out = append(out, c)
		}
	}
	return out, nil
}

func (r *RealCollectionRepo) Create(_ context.Context, c repository.Collection) (repository.Collection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.collections[c.ID] = c
	return c, nil
}

func (r *RealCollectionRepo) Update(_ context.Context, c repository.Collection) (repository.Collection, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.collections[c.ID]
	if !ok || existing.WorkspaceID != c.WorkspaceID {
		return repository.Collection{}, fmt.Errorf("collection %q: %w", c.ID, apperr.ErrNotFound)
	}
	r.collections[c.ID] = c
	return c, nil
}

func (r *RealCollectionRepo) Delete(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	c, ok := r.collections[id]
	if !ok || c.WorkspaceID != workspaceID {
		return fmt.Errorf("collection %q: %w", id, apperr.ErrNotFound)
	}
	delete(r.collections, id)
	delete(r.assets, id)
	return nil
}

func (r *RealCollectionRepo) AddAsset(_ context.Context, collectionID, assetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	for _, id := range r.assets[collectionID] {
		if id == assetID {
			return nil
		}
	}
	r.assets[collectionID] = append(r.assets[collectionID], assetID)
	return nil
}

func (r *RealCollectionRepo) RemoveAsset(_ context.Context, collectionID, assetID string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	ids := r.assets[collectionID]
	filtered := ids[:0]
	for _, id := range ids {
		if id != assetID {
			filtered = append(filtered, id)
		}
	}
	r.assets[collectionID] = filtered
	return nil
}
