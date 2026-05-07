package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/systemtags"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

var (
	ErrInvalidVariantType = errors.New("invalid variant type")
	ErrInvalidVariantReq  = errors.New("invalid variant request")
)

type invalidVariantInput string

func (e invalidVariantInput) Error() string { return string(e) }

func (e invalidVariantInput) Unwrap() error { return apperr.ErrInvalidInput }

type invalidVariantRequest string

func (e invalidVariantRequest) Error() string { return string(e) }

func (e invalidVariantRequest) Unwrap() error { return ErrInvalidVariantReq }

// VariantDTO is the output of VariantService methods.
type VariantDTO struct {
	ID                   string
	WorkspaceID          string
	AssetVersionID       string
	Type                 string
	StorageKey           string
	TransformParams      *string
	Size                 *int64
	Status               string
	ThumbnailKey         *string
	ThumbnailContentType string
	CreatedAt            time.Time
}

type variantService struct {
	variants repository.VariantRepository
	assets   repository.AssetRepository
	tags     TagService
	audit    audit.Writer
	actions  VariantActionsStore
	queue    queue.JobQueue
	storage  storage.Storage
}

// NewVariantService returns a VariantService.
func NewVariantService(variants repository.VariantRepository, assets repository.AssetRepository, tags TagService, aw audit.Writer) VariantService {
	return &variantService{variants: variants, assets: assets, tags: tags, audit: aw}
}

type VariantServiceDeps struct {
	Actions VariantActionsStore
	Queue   queue.JobQueue
	Storage storage.Storage
}

// NewVariantServiceWithDeps returns a VariantService with the extra dependencies
// required by advanced variant actions such as promote and rerun.
func NewVariantServiceWithDeps(variants repository.VariantRepository, assets repository.AssetRepository, tags TagService, aw audit.Writer, deps VariantServiceDeps) VariantService {
	return &variantService{
		variants: variants,
		assets:   assets,
		tags:     tags,
		audit:    aw,
		actions:  deps.Actions,
		queue:    deps.Queue,
		storage:  deps.Storage,
	}
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

func (s *variantService) PrepareCreate(ctx context.Context, p PrepareCreateVariantParams) (PreparedCreateVariant, error) {
	params := json.RawMessage("{}")
	if len(p.Params) > 0 {
		params = p.Params
	}

	if !validVariantType(p.Type) {
		return PreparedCreateVariant{}, ErrInvalidVariantType
	}

	switch {
	case requiresVideoAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "video/"):
		if p.Type == queue.JobTypeExtractAudio {
			return PreparedCreateVariant{}, invalidVariantInput("asset_not_video")
		}
		return PreparedCreateVariant{}, invalidVariantRequest("video transforms require a video asset")
	case requiresImageAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "image/"):
		return PreparedCreateVariant{}, invalidVariantRequest("image transforms require an image asset")
	case requiresAudioAsset(p.Type) && !strings.HasPrefix(p.AssetMimeType, "audio/"):
		return PreparedCreateVariant{}, invalidVariantInput("asset_not_audio")
	}

	if (p.Type == queue.JobTypeImageBgRemove || p.Type == queue.JobTypeImageWithPrompt) && !p.ImageRouterConfigured {
		return PreparedCreateVariant{}, invalidVariantInput("imagerouter_not_configured")
	}

	if p.Type == queue.JobTypeImageWatermark {
		normalized, err := s.prepareImageWatermarkParams(ctx, p, params)
		if err != nil {
			return PreparedCreateVariant{}, err
		}
		return PreparedCreateVariant{Type: p.Type, Params: normalized}, nil
	}

	if p.Type == queue.JobTypeVideoWatermark {
		normalized, err := s.prepareVideoWatermarkParams(ctx, p, params)
		if err != nil {
			return PreparedCreateVariant{}, err
		}
		return PreparedCreateVariant{Type: p.Type, Params: normalized}, nil
	}

	if p.Type == queue.JobTypeImageBgRemove {
		normalized, err := prepareImageRouterBgRemoveParams(params, p.DefaultBgRemoveModel)
		if err != nil {
			return PreparedCreateVariant{}, err
		}
		return PreparedCreateVariant{Type: p.Type, Params: normalized}, nil
	}

	if p.Type == queue.JobTypeImageWithPrompt {
		normalized, err := prepareImageRouterPromptParams(params, p.DefaultImageModel)
		if err != nil {
			return PreparedCreateVariant{}, err
		}
		return PreparedCreateVariant{Type: p.Type, Params: normalized}, nil
	}

	normalized, err := prepareAudioVariantParams(p.Type, p.AssetMimeType, params)
	if err != nil {
		return PreparedCreateVariant{}, err
	}

	return PreparedCreateVariant{Type: p.Type, Params: normalized}, nil
}

func (s *variantService) prepareImageWatermarkParams(ctx context.Context, p PrepareCreateVariantParams, raw json.RawMessage) (json.RawMessage, error) {
	var params transform.WatermarkParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInput("invalid watermark params")
	}
	params.WatermarkAssetID = ""
	params.Normalize()

	wm, err := s.resolveSystemTagAsset(ctx, p.WorkspaceID, p.AssetID, systemtags.Watermark)
	if err != nil {
		return nil, err
	}
	params.WatermarkAssetID = wm.ID
	return marshalRaw(params), nil
}

func (s *variantService) prepareVideoWatermarkParams(ctx context.Context, p PrepareCreateVariantParams, raw json.RawMessage) (json.RawMessage, error) {
	var params transform.VideoWatermarkParams
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInput("invalid watermark params")
	}
	params.WatermarkAssetID = ""
	params.Normalize()

	wm, err := s.resolveSystemTagAsset(ctx, p.WorkspaceID, p.AssetID, systemtags.Watermark)
	if err != nil {
		return nil, err
	}
	params.WatermarkAssetID = wm.ID
	return marshalRaw(params), nil
}

func (s *variantService) resolveSystemTagAsset(ctx context.Context, workspaceID, assetID, tagName string) (*AssetDTO, error) {
	if s.tags == nil {
		return nil, fmt.Errorf("tag service unavailable")
	}

	asset, err := s.assets.GetByID(ctx, workspaceID, assetID)
	if err != nil {
		return nil, err
	}

	resolved, err := s.tags.ResolveSystemTag(ctx, workspaceID, tagName, SystemTagScope{
		FolderID:  asset.FolderID,
		ProjectID: asset.ProjectID,
	})
	if err != nil {
		return nil, err
	}
	if resolved == nil {
		return nil, invalidVariantInput("no_watermark_asset")
	}
	return resolved, nil
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

func validVariantType(variantType string) bool {
	switch variantType {
	case queue.JobTypeImageResize,
		queue.JobTypeImageWatermark,
		queue.JobTypeImageConvert,
		queue.JobTypeImageCrop,
		queue.JobTypeVideoCaptureImage,
		queue.JobTypeVideoTranscode,
		queue.JobTypeVideoWatermark,
		queue.JobTypeImageBgRemove,
		queue.JobTypeImageWithPrompt,
		queue.JobTypeImageSmartCrop,
		queue.JobTypeExtractAudio,
		queue.JobTypeTranscodeAudio,
		queue.JobTypeNormalizeAudio:
		return true
	default:
		return false
	}
}

func requiresVideoAsset(variantType string) bool {
	return variantType == queue.JobTypeVideoCaptureImage ||
		variantType == queue.JobTypeVideoTranscode ||
		variantType == queue.JobTypeVideoWatermark ||
		variantType == queue.JobTypeExtractAudio
}

func requiresImageAsset(variantType string) bool {
	return variantType == queue.JobTypeImageResize ||
		variantType == queue.JobTypeImageConvert ||
		variantType == queue.JobTypeImageCrop ||
		variantType == queue.JobTypeImageWatermark ||
		variantType == queue.JobTypeImageSmartCrop ||
		variantType == queue.JobTypeImageBgRemove ||
		variantType == queue.JobTypeImageWithPrompt
}

func requiresAudioAsset(variantType string) bool {
	return variantType == queue.JobTypeTranscodeAudio ||
		variantType == queue.JobTypeNormalizeAudio
}

func prepareAudioVariantParams(variantType, mimeType string, raw json.RawMessage) (json.RawMessage, error) {
	switch variantType {
	case queue.JobTypeExtractAudio:
		var p transform.AudioParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInput("invalid audio params")
		}
		if p.OutputFormat == "" {
			p.OutputFormat = "aac"
		}
		if p.Bitrate == "" {
			p.Bitrate = "192k"
		}
		if !isAllowedAudioBitrate(p.Bitrate) {
			return nil, invalidVariantInput("unsupported audio bitrate")
		}
		if !isAllowedAudioFormat(p.OutputFormat, "aac", "mp3", "opus", "flac") {
			return nil, invalidVariantInput("unsupported audio format")
		}
		return marshalRaw(p), nil
	case queue.JobTypeTranscodeAudio:
		var p transform.AudioParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInput("invalid audio params")
		}
		if p.OutputFormat == "" {
			return nil, invalidVariantInput("format is required")
		}
		if p.Bitrate == "" {
			p.Bitrate = "192k"
		}
		if !isAllowedAudioBitrate(p.Bitrate) {
			return nil, invalidVariantInput("unsupported audio bitrate")
		}
		if !isAllowedAudioFormat(p.OutputFormat, "mp3", "aac", "opus", "ogg", "flac", "wav") {
			return nil, invalidVariantInput("unsupported audio format")
		}
		return marshalRaw(p), nil
	case queue.JobTypeNormalizeAudio:
		var p transform.AudioParams
		if err := json.Unmarshal(raw, &p); err != nil {
			return nil, invalidVariantInput("invalid audio params")
		}
		if p.OutputFormat == "" {
			p.OutputFormat = "source"
		}
		if p.OutputFormat == "source" {
			p.OutputFormat = transform.AudioFormatFromMimeType(mimeType)
		}
		if p.TargetLUFS == 0 {
			p.TargetLUFS = -16
		}
		if p.TargetLUFS < -70 || p.TargetLUFS > 0 {
			return nil, invalidVariantInput("target_lufs must be between -70 and 0")
		}
		if !isAllowedAudioFormat(p.OutputFormat, "mp3", "aac", "wav", "ogg", "flac") {
			return nil, invalidVariantInput("unsupported audio format")
		}
		return marshalRaw(p), nil
	default:
		return raw, nil
	}
}

func marshalRaw(v any) json.RawMessage {
	b, _ := json.Marshal(v)
	return b
}

func prepareImageRouterBgRemoveParams(raw json.RawMessage, defaultModel string) (json.RawMessage, error) {
	var params struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInput("invalid image background removal params")
	}
	params.Model = strings.TrimSpace(params.Model)
	params.Prompt = strings.TrimSpace(params.Prompt)
	if params.Model == "" {
		params.Model = defaultModel
	}
	if params.Model == "" {
		return nil, invalidVariantInput("model is required")
	}
	return marshalRaw(params), nil
}

func prepareImageRouterPromptParams(raw json.RawMessage, defaultModel string) (json.RawMessage, error) {
	var params struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(raw, &params); err != nil {
		return nil, invalidVariantInput("invalid image prompt params")
	}
	params.Prompt = strings.TrimSpace(params.Prompt)
	params.Model = strings.TrimSpace(params.Model)
	if params.Prompt == "" {
		return nil, invalidVariantRequest("prompt_required")
	}
	if params.Model == "" {
		params.Model = defaultModel
	}
	if params.Model == "" {
		return nil, invalidVariantInput("model is required")
	}
	return marshalRaw(params), nil
}

func isAllowedAudioFormat(format string, allowed ...string) bool {
	for _, a := range allowed {
		if strings.EqualFold(format, a) {
			return true
		}
	}
	return false
}

func isAllowedAudioBitrate(bitrate string) bool {
	switch bitrate {
	case "64k", "96k", "128k", "192k", "256k", "320k":
		return true
	default:
		return false
	}
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
	status := v.Status
	if status == "" {
		status = "ready"
	}
	return &VariantDTO{
		ID:                   v.ID,
		WorkspaceID:          v.WorkspaceID,
		AssetVersionID:       v.AssetVersionID,
		Type:                 v.Type,
		StorageKey:           v.StorageKey,
		TransformParams:      v.TransformParams,
		Size:                 v.Size,
		Status:               status,
		ThumbnailKey:         v.ThumbnailKey,
		ThumbnailContentType: v.ThumbnailContentType,
		CreatedAt:            v.CreatedAt,
	}
}
