package memory

import (
	"context"
	"fmt"
	"sync"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// RealShareRepo is a map-backed ShareRepository for unit tests.
type RealShareRepo struct {
	mu     sync.RWMutex
	shares map[string]repository.Share
}

func NewRealShareRepo() *RealShareRepo {
	return &RealShareRepo{shares: make(map[string]repository.Share)}
}

func (r *RealShareRepo) GetByID(_ context.Context, workspaceID, id string) (repository.Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sh, ok := r.shares[id]
	if !ok || sh.WorkspaceID != workspaceID {
		return repository.Share{}, fmt.Errorf("share %q: %w", id, apperr.ErrNotFound)
	}
	return sh, nil
}

func (r *RealShareRepo) List(_ context.Context, workspaceID string) ([]repository.Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var out []repository.Share
	for _, sh := range r.shares {
		if sh.WorkspaceID == workspaceID {
			out = append(out, sh)
		}
	}
	return out, nil
}

func (r *RealShareRepo) Create(_ context.Context, sh repository.Share) (repository.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.shares[sh.ID] = sh
	return sh, nil
}

func (r *RealShareRepo) Update(_ context.Context, sh repository.Share) (repository.Share, error) {
	r.mu.Lock()
	defer r.mu.Unlock()
	existing, ok := r.shares[sh.ID]
	if !ok || existing.WorkspaceID != sh.WorkspaceID {
		return repository.Share{}, fmt.Errorf("share %q: %w", sh.ID, apperr.ErrNotFound)
	}
	r.shares[sh.ID] = sh
	return sh, nil
}

func (r *RealShareRepo) Revoke(_ context.Context, workspaceID, id string) error {
	r.mu.Lock()
	defer r.mu.Unlock()
	sh, ok := r.shares[id]
	if !ok || sh.WorkspaceID != workspaceID {
		return fmt.Errorf("share %q: %w", id, apperr.ErrNotFound)
	}
	now := time.Now().UTC().Format("2006-01-02 15:04:05")
	sh.RevokedAt = &now
	r.shares[id] = sh
	return nil
}

func (r *RealShareRepo) GetPublic(_ context.Context, id string) (repository.Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sh, ok := r.shares[id]
	if !ok {
		return repository.Share{}, fmt.Errorf("share %q: %w", id, apperr.ErrNotFound)
	}
	return sh, nil
}

func (r *RealShareRepo) GetByIDAndWorkspace(_ context.Context, workspaceID, id string) (repository.Share, error) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	sh, ok := r.shares[id]
	if !ok || sh.WorkspaceID != workspaceID {
		return repository.Share{}, fmt.Errorf("share %q: %w", id, apperr.ErrNotFound)
	}
	return sh, nil
}

func (r *RealShareRepo) IncrementViewCount(_ context.Context, _ string) error { return nil }

func (r *RealShareRepo) ListAssetsByTarget(_ context.Context, _, _ string) ([]repository.PublicAsset, error) {
	return nil, nil
}

func (r *RealShareRepo) GetPublicAsset(_ context.Context, _ string) (repository.PublicAsset, error) {
	return repository.PublicAsset{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *RealShareRepo) GetPublicAssetFile(_ context.Context, _ string) (repository.PublicAssetFile, error) {
	return repository.PublicAssetFile{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *RealShareRepo) GetPublicAssetThumb(_ context.Context, _ string) (*string, time.Time, error) {
	return nil, time.Time{}, fmt.Errorf("%w", apperr.ErrNotFound)
}

func (r *RealShareRepo) IsAssetInTarget(_ context.Context, _, _, _ string) (bool, error) {
	return false, nil
}

func (r *RealShareRepo) CreateComment(_ context.Context, c repository.ShareComment) (repository.ShareComment, error) {
	return c, nil
}

func (r *RealShareRepo) ListCommentsByShare(_ context.Context, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}

func (r *RealShareRepo) ListCommentsByShareAndAsset(_ context.Context, _, _ string) ([]repository.ShareComment, error) {
	return nil, nil
}

func (r *RealShareRepo) DeleteComment(_ context.Context, _, _ string) error { return nil }
