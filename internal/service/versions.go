package service

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
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
	audit    audit.Writer
}

// NewVersionService returns a VersionService.
func NewVersionService(versions repository.VersionRepository, aw audit.Writer) VersionService {
	return &versionService{versions: versions, audit: aw}
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

func (s *versionService) GetCurrentByAsset(ctx context.Context, assetID string) (*VersionDTO, error) {
	v, err := s.versions.GetCurrentByAsset(ctx, assetID)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) ListWithVariantCount(ctx context.Context, assetID string) ([]*VersionWithCountDTO, error) {
	rows, err := s.versions.ListWithVariantCount(ctx, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VersionWithCountDTO, len(rows))
	for i, r := range rows {
		out[i] = &VersionWithCountDTO{
			VersionDTO:   *toVersionDTO(r.AssetVersion),
			VariantCount: r.VariantCount,
		}
	}
	return out, nil
}

func (s *versionService) GetByHash(ctx context.Context, assetID, contentHash string) (*VersionDTO, error) {
	v, err := s.versions.GetByHash(ctx, assetID, contentHash)
	if err != nil {
		return nil, err
	}
	return toVersionDTO(v), nil
}

func (s *versionService) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	return s.versions.NextVersionNum(ctx, assetID)
}

func (s *versionService) Create(ctx context.Context, v *VersionDTO) (*VersionDTO, error) {
	created, err := s.versions.Create(ctx, repository.AssetVersion{
		ID:          v.ID,
		AssetID:     v.AssetID,
		WorkspaceID: v.WorkspaceID,
		VersionNum:  v.VersionNum,
		StorageKey:  v.StorageKey,
		ContentHash: v.ContentHash,
		MimeType:    v.MimeType,
		Size:        v.Size,
		Width:       v.Width,
		Height:      v.Height,
		DurationSec: v.DurationSec,
		Comment:     v.Comment,
		CreatedBy:   v.CreatedBy,
	})
	if err != nil {
		return nil, err
	}
	return toVersionDTO(created), nil
}

func (s *versionService) SetCurrent(ctx context.Context, assetID, versionID string) error {
	return s.versions.SetCurrent(ctx, assetID, versionID)
}

func (s *versionService) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	return s.versions.SetAssetThumbnail(ctx, assetID, key)
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
	if err := s.versions.SoftDelete(ctx, versionID); err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionDeleted,
		Payload:     audit.AssetVersionDeletedPayload{V: 1, VersionNum: v.VersionNum},
	})
	return nil
}

// WriteVersionUploaded emits an asset_version_uploaded audit event.
// Called by handlers that orchestrate the multi-step upload flow.
func (s *versionService) WriteVersionUploaded(ctx context.Context, workspaceID, assetID string, v *VersionDTO, comment string) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionUploaded,
		Payload:     audit.AssetVersionUploadedPayload{V: 1, VersionNum: v.VersionNum, Size: v.Size, Comment: comment},
	})
}

// WriteVersionRestored emits an asset_version_restored audit event.
// Called by handlers after SetCurrent succeeds.
func (s *versionService) WriteVersionRestored(ctx context.Context, workspaceID, assetID string, fromVersionNum, toVersionNum int64) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVersionRestored,
		Payload:     audit.AssetVersionRestoredPayload{V: 1, FromVersionNum: fromVersionNum, ToVersionNum: toVersionNum},
	})
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
