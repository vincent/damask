package service

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// VersionDTO is the output of VersionService methods.
type VersionDTO struct {
	ID           string
	AssetID      string
	WorkspaceID  string
	VersionNum   int64
	StorageKey   string
	ContentHash  string
	MimeType     string
	Size         int64
	Width        *int64
	Height       *int64
	DurationSec  *float64
	ThumbnailKey *string
	Comment      *string
	CreatedBy    *string
	CreatedAt    time.Time
	IsCurrent    bool
	DeletedAt    *string
}

type versionService struct {
	versions repository.VersionRepository
}

// NewVersionService returns a VersionService.
func NewVersionService(versions repository.VersionRepository) VersionService {
	return &versionService{versions: versions}
}

func (s *versionService) List(ctx context.Context, assetID string) ([]*VersionDTO, error) {
	rows, err := s.versions.ListByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VersionDTO, len(rows))
	for i, r := range rows {
		out[i] = toVersionDTO(r)
	}
	return out, nil
}

func (s *versionService) Get(ctx context.Context, workspaceID, id string) (*VersionDTO, error) {
	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

// Delete soft-deletes a non-current version that is not in use as a cover.
func (s *versionService) Delete(ctx context.Context, workspaceID, assetID, versionID string) error {
	v, err := s.versions.GetByIDForWorkspace(ctx, workspaceID, versionID)
	if err != nil {
		return err
	}
	if v.AssetID != assetID {
		return fmt.Errorf("version %q: %w", versionID, apperr.ErrNotFound)
	}
	if v.IsCurrent {
		return fmt.Errorf("cannot delete the current version: %w", apperr.ErrInvalidInput)
	}
	isCover, err := s.versions.IsReferencedAsCover(ctx, versionID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("version is in use as a project cover or workspace icon: %w", apperr.ErrConflict)
	}
	return s.versions.SoftDelete(ctx, versionID)
}

func toVersionDTO(v repository.AssetVersion) *VersionDTO {
	return &VersionDTO{
		ID:           v.ID,
		AssetID:      v.AssetID,
		WorkspaceID:  v.WorkspaceID,
		VersionNum:   v.VersionNum,
		StorageKey:   v.StorageKey,
		ContentHash:  v.ContentHash,
		MimeType:     v.MimeType,
		Size:         v.Size,
		Width:        v.Width,
		Height:       v.Height,
		DurationSec:  v.DurationSec,
		ThumbnailKey: v.ThumbnailKey,
		Comment:      v.Comment,
		CreatedBy:    v.CreatedBy,
		CreatedAt:    v.CreatedAt,
		IsCurrent:    v.IsCurrent,
		DeletedAt:    v.DeletedAt,
	}
}
