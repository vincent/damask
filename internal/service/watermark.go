package service

import (
	"context"
	"database/sql"
	"errors"
	"fmt"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

var ErrNoWatermarkAsset = errors.New(
	"no watermark asset found: upload an image named '*watermark*' to this folder, project, or workspace",
)

type watermarkService struct {
	db     *dbgen.Queries
	assets repository.AssetRepository
}

func NewWatermarkService(db *dbgen.Queries, assets repository.AssetRepository, folders repository.FolderRepository) WatermarkService {
	_ = folders
	return &watermarkService{db: db, assets: assets}
}

func (s *watermarkService) ResolveWatermarkAsset(ctx context.Context, workspaceID, assetID string) (*WatermarkAssetDTO, error) {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}

	if asset.FolderID != nil {
		row, err := s.db.FindWatermarkAssetInFolder(ctx, dbgen.FindWatermarkAssetInFolderParams{
			WorkspaceID: workspaceID,
			FolderID:    asset.FolderID,
		})
		if err == nil {
			return toWatermarkAssetDTO(row, "folder"), nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find folder watermark: %w", err)
		}
	}

	if asset.ProjectID != nil {
		row, err := s.db.FindWatermarkAssetInProject(ctx, dbgen.FindWatermarkAssetInProjectParams{
			WorkspaceID: workspaceID,
			ProjectID:   *asset.ProjectID,
		})
		if err == nil {
			return toWatermarkAssetDTO(row, "project"), nil
		}
		if !errors.Is(err, sql.ErrNoRows) {
			return nil, fmt.Errorf("find project watermark: %w", err)
		}
	}

	row, err := s.db.FindWatermarkAssetInWorkspace(ctx, workspaceID)
	if err == nil {
		return toWatermarkAssetDTO(row, "workspace"), nil
	}
	if errors.Is(err, sql.ErrNoRows) {
		return nil, fmt.Errorf("%w: %w", ErrNoWatermarkAsset, apperr.ErrInvalidInput)
	}
	return nil, fmt.Errorf("find workspace watermark: %w", err)
}

func toWatermarkAssetDTO(asset dbgen.Asset, scope string) *WatermarkAssetDTO {
	return &WatermarkAssetDTO{
		ID:         asset.ID,
		Name:       asset.OriginalFilename,
		StorageKey: asset.StorageKey,
		MimeType:   asset.MimeType,
		Scope:      scope,
	}
}
