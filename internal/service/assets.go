package service

import (
	"context"

	"damask/server/internal/repository"
)

type assetService struct {
	assets repository.AssetRepository
}

// NewAssetService returns an AssetService backed by the given repository.
func NewAssetService(assets repository.AssetRepository) AssetService {
	return &assetService{assets: assets}
}

func (s *assetService) Get(ctx context.Context, workspaceID, assetID string) (*AssetDTO, error) {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err // apperr.ErrNotFound already wrapped by the repository
	}
	return toAssetDTO(asset), nil
}

func toAssetDTO(a repository.Asset) *AssetDTO {
	return &AssetDTO{
		ID:               a.ID,
		WorkspaceID:      a.WorkspaceID,
		ProjectID:        a.ProjectID,
		FolderID:         a.FolderID,
		OriginalFilename: a.OriginalFilename,
		StorageKey:       a.StorageKey,
		MimeType:         a.MimeType,
		Size:             a.Size,
		Width:            a.Width,
		Height:           a.Height,
		ThumbnailKey:     a.ThumbnailKey,
		Metadata:         a.Metadata,
		CurrentVersionID: a.CurrentVersionID,
		CreatedAt:        a.CreatedAt,
		UpdatedAt:        a.UpdatedAt,
	}
}
