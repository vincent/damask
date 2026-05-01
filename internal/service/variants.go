package service

import (
	"context"
	"fmt"
	"log/slog"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/repository"
	apptelemetry "damask/server/internal/telemetry"

	"go.opentelemetry.io/otel/attribute"
)

// VariantDTO is the output of VariantService methods.
type VariantDTO struct {
	ID                   string
	WorkspaceID          string
	AssetVersionID       string
	Type                 string
	StorageKey           string
	TransformParams      *string
	Size                 *int64
	ThumbnailKey         *string
	ThumbnailContentType string
	CreatedAt            time.Time
}

type variantService struct {
	variants repository.VariantRepository
	assets   repository.AssetRepository
	audit    audit.Writer
}

// NewVariantService returns a VariantService.
func NewVariantService(variants repository.VariantRepository, assets repository.AssetRepository, aw audit.Writer) VariantService {
	return &variantService{variants: variants, assets: assets, audit: aw}
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

func (s *variantService) Create(ctx context.Context, p CreateVariantParams) (dto *VariantDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.create",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.version_id", p.AssetVersionID),
		attribute.String("damask.variant.type", p.Type),
	)
	defer func() {
		if dto != nil {
			span.SetAttributes(attribute.String("damask.variant_id", dto.ID))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "variant create failed", "workspace_id", p.WorkspaceID, "asset_id", p.AssetID, "type", p.Type, "error", err)
		}
	}()

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
	dto = toVariantDTO(v)
	// Only emit audit for manual uploads (job-queued variants are audited via WriteVariantQueued).
	if p.AssetID != "" {
		actor := auth.ActorFromCtx(ctx)
		s.audit.WriteAsset(ctx, audit.AssetEvent{
			WorkspaceID: p.WorkspaceID,
			AssetID:     p.AssetID,
			UserID:      actor.UserID,
			ActorType:   actor.Type,
			EventType:   audit.EventAssetVariantCreated,
			Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: dto.Type},
		})
	}
	return dto, nil
}

// WriteVariantQueued emits asset_variant_created for job-queued variants (before the job runs).
func (s *variantService) WriteVariantQueued(ctx context.Context, workspaceID, assetID, variantType string) {
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantCreated,
		Payload:     audit.AssetVariantCreatedPayload{V: 1, Type: variantType},
	})
}

// WriteVariantDownloadedAsync emits asset_variant_downloaded in a background goroutine.
func (s *variantService) WriteVariantDownloadedAsync(workspaceID, assetID, variantID, variantType string) {
	s.audit.WriteAssetAsync(audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		ActorType:   audit.ActorTypeUser,
		EventType:   audit.EventAssetVariantDownloaded,
		Payload:     audit.AssetVariantDownloadedPayload{V: 1, VariantID: variantID, Type: variantType},
	})
}

// Delete deletes a variant. Only variants attached to the asset's current version may be deleted.
func (s *variantService) Delete(ctx context.Context, workspaceID, assetID, variantID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.variants.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.variant_id", variantID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "variant delete failed", "workspace_id", workspaceID, "asset_id", assetID, "variant_id", variantID, "error", err)
		}
	}()

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
	if err := s.variants.Delete(ctx, workspaceID, variantID); err != nil {
		return err
	}
	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     assetID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetVariantDeleted,
		Payload:     audit.AssetVariantDeletedPayload{V: 1, VariantID: variantID, Type: v.Type},
	})
	return nil
}

func toVariantDTO(v repository.Variant) *VariantDTO {
	return &VariantDTO{
		ID:                   v.ID,
		WorkspaceID:          v.WorkspaceID,
		AssetVersionID:       v.AssetVersionID,
		Type:                 v.Type,
		StorageKey:           v.StorageKey,
		TransformParams:      v.TransformParams,
		Size:                 v.Size,
		ThumbnailKey:         v.ThumbnailKey,
		ThumbnailContentType: v.ThumbnailContentType,
		CreatedAt:            v.CreatedAt,
	}
}
