package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/imagerouter"
	"damask/server/internal/queue"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

const variantStatusReady = "ready"

// VariantJobPayload is the payload for user-triggered variant creation jobs.
// VersionID and VersionNum identify the asset version the variant is bound to.
type VariantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
	VersionID   string          `json:"version_id"`
	VersionNum  int64           `json:"version_num"`
	VariantID   string          `json:"variant_id,omitempty"`
	StorageKey  string          `json:"storage_key"`
	MimeType    string          `json:"mime_type"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
	Title       *string         `json:"title,omitempty"`
	IsShared    bool            `json:"is_shared,omitempty"`
}

// enqueueVariantThumb enqueues a generate_variant_thumbnail job after a variant row is created.
func (s *JobServer) enqueueVariantThumb(
	ctx context.Context,
	p VariantJobPayload,
	variantID, storageKey, contentType string,
) {
	_ = EnqueueVariantThumbnailJob(ctx, s, VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		StorageKey:  storageKey,
		MimeType:    contentType,
	})
}

// imageLocalBuild is the variantBuildFn for local image transforms (resize, convert, crop, watermark, smart-crop).
func (s *JobServer) imageLocalBuild(jobType, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.imageLocalTransformer(jobType, params, workspaceID)
}

// imageLocalTransformer returns a variantTransformer for local
// image transforms: resize, convert, crop, watermark, smart-crop.
func (s *JobServer) imageLocalTransformer(
	jobType string,
	params json.RawMessage,
	workspaceID string,
) (variantTransformer, error) {
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", jobType),
		)

		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("get file: %w", err)
		}
		defer rc.Close()

		var (
			data        []byte
			contentType string
		)
		switch jobType {
		case queue.JobTypeImageResize:
			var p transform.ResizeParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse resize params: %w", err)
			}
			data, contentType, err = s.trf.ImageResize(rc, p)
		case queue.JobTypeImageConvert:
			var p transform.ConvertParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse convert params: %w", err)
			}
			data, contentType, err = s.trf.ImageConvert(rc, p)
		case queue.JobTypeImageCrop:
			var p transform.CropParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse crop params: %w", err)
			}
			data, contentType, err = s.trf.ImageCrop(rc, p)
		case queue.JobTypeImageWatermark:
			var p transform.WatermarkParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse watermark params: %w", err)
			}
			if p.WatermarkAssetID == "" {
				err = errors.New("watermark asset id is required")
				telemetry.EndSpan(span, err)
				return nil, "", err
			}
			wm, werr := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
				ID:          p.WatermarkAssetID,
				WorkspaceID: workspaceID,
			})
			if werr != nil {
				telemetry.EndSpan(span, werr)
				return nil, "", fmt.Errorf("get watermark asset: %w", werr)
			}
			wmRC, werr := s.storage.Get(wm.StorageKey)
			if werr != nil {
				telemetry.EndSpan(span, werr)
				return nil, "", fmt.Errorf("get watermark file: %w", werr)
			}
			data, contentType, err = s.trf.ImageWatermark(rc, wmRC, p)
			_ = wmRC.Close()
		case queue.JobTypeImageSmartCrop:
			var p transform.SmartCropParams
			if err = json.Unmarshal(params, &p); err != nil {
				telemetry.EndSpan(span, err)
				return nil, "", fmt.Errorf("parse smartcrop params: %w", err)
			}
			data, contentType, err = s.trf.ImageSmartCrop(rc, p)
		default:
			err = fmt.Errorf("unknown image job type: %s", jobType)
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("transform: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, contentType, nil
	}, nil
}

// imageBgRemoveBuild is the variantBuildFn for background removal.
func (s *JobServer) imageBgRemoveBuild(_, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.imageBgRemoveTransformer(workspaceID, params)
}

// imageBgRemoveTransformer returns a variantTransformer for background removal.
func (s *JobServer) imageBgRemoveTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p struct {
		Model  string `json:"model"`
		Prompt string `json:"prompt"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse bg remove params: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_bg_remove"),
		)
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.BgRemove(ctx, imageData, imagerouter.BgRemoveParams{
					Model:  p.Model,
					Prompt: p.Prompt,
				})
			},
		)
		if err != nil && strings.Contains(err.Error(), "Prompt must be a string") {
			err = errors.New(
				"the selected model is not available for background removal. please choose another model and try again",
			)
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("remove background: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

// imageWithPromptBuild is the variantBuildFn for AI image-with-prompt.
func (s *JobServer) imageWithPromptBuild(_, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.imageWithPromptTransformer(workspaceID, params)
}

// imageWithPromptTransformer returns a variantTransformer for AI image-with-prompt.
func (s *JobServer) imageWithPromptTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	var p struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(params, &p); err != nil {
		return nil, fmt.Errorf("parse image prompt params: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "image_with_prompt"),
		)
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.Transform(ctx, imageData, imagerouter.PromptParams{
					Prompt: p.Prompt,
					Model:  p.Model,
				})
			},
		)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("image transform with prompt: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return result, "image/png", nil
	}, nil
}

func (s *JobServer) runImageRouterJob(
	ctx context.Context,
	workspaceID string,
	sourceKey string,
	callFn func(*imagerouter.Client, []byte) ([]byte, error),
) ([]byte, error) {
	ctx, span := telemetry.StartBackgroundSpan(ctx, "imagerouter.job",
		attribute.String("damask.workspace_id", workspaceID),
	)

	rc, err := s.storage.Get(sourceKey)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	imageData, err := io.ReadAll(rc)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, fmt.Errorf("read source: %w", err)
	}

	key, source, err := s.imgKeyResolver(ctx, workspaceID)
	if err != nil {
		telemetry.EndSpan(span, err)
		return nil, err
	}
	if source == imagerouter.SourceNone {
		telemetry.EndSpan(span, imagerouter.ErrNotConfigured)
		return nil, imagerouter.ErrNotConfigured
	}

	client := imagerouter.NewClient(key, s.cfg.ImageRouter.RetryPaidOnFreeLimit)
	result, err := callFn(client, imageData)
	telemetry.EndSpan(span, err)
	if err != nil {
		return nil, err
	}
	return result, nil
}

func boolToInt64(b bool) int64 {
	if b {
		return 1
	}
	return 0
}
