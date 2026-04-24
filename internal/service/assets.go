package service

import (
	"context"
	"fmt"
	"path/filepath"
	"strings"

	"damask/server/internal/apperr"
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
		return nil, err
	}
	return toAssetDTO(asset), nil
}

func (s *assetService) List(ctx context.Context, params ListAssetsParams) ([]*AssetDTO, error) {
	var cursorAt interface{}
	if params.CursorAt != nil {
		cursorAt = params.CursorAt
	}
	var projectID interface{}
	if params.ProjectID != nil {
		projectID = params.ProjectID
	}
	var mimePrefix interface{}
	if params.MimePrefix != nil {
		mimePrefix = params.MimePrefix
	}
	rows, err := s.assets.List(ctx, repository.ListAssetsParams{
		WorkspaceID: params.WorkspaceID,
		ProjectID:   projectID,
		MimePrefix:  mimePrefix,
		CursorAt:    cursorAt,
		CursorID:    params.CursorID,
		Limit:       params.Limit,
	})
	if err != nil {
		return nil, err
	}
	out := make([]*AssetDTO, len(rows))
	for i, r := range rows {
		out[i] = toAssetDTO(r)
	}
	return out, nil
}

func (s *assetService) Move(ctx context.Context, workspaceID, assetID string, p MoveAssetParams) (*AssetDTO, error) {
	if _, err := s.assets.GetByID(ctx, workspaceID, assetID); err != nil {
		return nil, err
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:          assetID,
		WorkspaceID: workspaceID,
		FolderID:    p.FolderID,
		ProjectID:   p.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	return toAssetDTO(updated), nil
}

func (s *assetService) Rename(ctx context.Context, workspaceID, assetID, newStem string) (*AssetDTO, error) {
	newStem = strings.TrimSpace(newStem)
	if newStem == "" {
		return nil, fmt.Errorf("name cannot be empty: %w", apperr.ErrInvalidInput)
	}
	existing, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	ext := filepath.Ext(existing.OriginalFilename)
	stem := strings.TrimSuffix(newStem, ext)
	newName := stem + ext
	if newName == existing.OriginalFilename {
		return toAssetDTO(existing), nil
	}
	updated, err := s.assets.Update(ctx, repository.UpdateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		OriginalFilename: &newName,
	})
	if err != nil {
		return nil, err
	}
	return toAssetDTO(updated), nil
}

func (s *assetService) Delete(ctx context.Context, workspaceID, assetID string) error {
	isCover, err := s.assets.IsProjectCover(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isCover {
		return fmt.Errorf("asset is used as a project cover: %w", apperr.ErrConflict)
	}
	isIcon, err := s.assets.IsWorkspaceIcon(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	if isIcon {
		return fmt.Errorf("asset is used as the workspace icon: %w", apperr.ErrConflict)
	}
	return s.assets.SoftDelete(ctx, workspaceID, assetID)
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
