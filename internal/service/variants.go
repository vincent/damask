package service

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
)

// VariantDTO is the output of VariantService methods.
type VariantDTO struct {
	ID              string
	WorkspaceID     string
	AssetVersionID  string
	Type            string
	StorageKey      string
	TransformParams *string
	Size            *int64
	CreatedAt       time.Time
}

type variantService struct {
	variants repository.VariantRepository
	assets   repository.AssetRepository
}

// NewVariantService returns a VariantService.
func NewVariantService(variants repository.VariantRepository, assets repository.AssetRepository) VariantService {
	return &variantService{variants: variants, assets: assets}
}

func (s *variantService) List(ctx context.Context, workspaceID, assetID string) ([]*VariantDTO, error) {
	rows, err := s.variants.ListByAsset(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}
	out := make([]*VariantDTO, len(rows))
	for i, r := range rows {
		out[i] = toVariantDTO(r)
	}
	return out, nil
}

func (s *variantService) Get(ctx context.Context, workspaceID, id string) (*VariantDTO, error) {
	v, err := s.variants.GetByID(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return toVariantDTO(v), nil
}

func (s *variantService) Create(ctx context.Context, p CreateVariantParams) (*VariantDTO, error) {
	v, err := s.variants.Create(ctx, repository.Variant{
		ID:              p.ID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.AssetVersionID,
		Type:            p.Type,
		StorageKey:      p.StorageKey,
		TransformParams: p.TransformParams,
		Size:            p.Size,
	})
	if err != nil {
		return nil, err
	}
	return toVariantDTO(v), nil
}

// Delete deletes a variant. Only variants attached to the asset's current version may be deleted.
func (s *variantService) Delete(ctx context.Context, workspaceID, assetID, variantID string) error {
	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return err
	}
	v, err := s.variants.GetByID(ctx, workspaceID, variantID)
	if err != nil {
		return err
	}
	if asset.CurrentVersionID == nil || v.AssetVersionID != *asset.CurrentVersionID {
		return fmt.Errorf("variant belongs to a previous version: %w", apperr.ErrInvalidInput)
	}
	return s.variants.Delete(ctx, workspaceID, variantID)
}

func toVariantDTO(v repository.Variant) *VariantDTO {
	return &VariantDTO{
		ID:              v.ID,
		WorkspaceID:     v.WorkspaceID,
		AssetVersionID:  v.AssetVersionID,
		Type:            v.Type,
		StorageKey:      v.StorageKey,
		TransformParams: v.TransformParams,
		Size:            v.Size,
		CreatedAt:       v.CreatedAt,
	}
}
