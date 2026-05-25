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

// imageLocalTransformer returns a variantTransformer for local (non-imagerouter)
// image transforms: resize, convert, crop, watermark, smart-crop.
func (s *JobServer) imageLocalTransformer(
	jobType string,
	params json.RawMessage,
	workspaceID string,
) (variantTransformer, error) {
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
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
				return nil, "", fmt.Errorf("parse resize params: %w", err)
			}
			data, contentType, err = s.trf.ImageResize(rc, p)
		case queue.JobTypeImageConvert:
			var p transform.ConvertParams
			if err = json.Unmarshal(params, &p); err != nil {
				return nil, "", fmt.Errorf("parse convert params: %w", err)
			}
			data, contentType, err = s.trf.ImageConvert(rc, p)
		case queue.JobTypeImageCrop:
			var p transform.CropParams
			if err = json.Unmarshal(params, &p); err != nil {
				return nil, "", fmt.Errorf("parse crop params: %w", err)
			}
			data, contentType, err = s.trf.ImageCrop(rc, p)
		case queue.JobTypeImageWatermark:
			var p transform.WatermarkParams
			if err = json.Unmarshal(params, &p); err != nil {
				return nil, "", fmt.Errorf("parse watermark params: %w", err)
			}
			if p.WatermarkAssetID == "" {
				return nil, "", errors.New("watermark asset id is required")
			}
			wm, werr := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
				ID:          p.WatermarkAssetID,
				WorkspaceID: workspaceID,
			})
			if werr != nil {
				return nil, "", fmt.Errorf("get watermark asset: %w", werr)
			}
			wmRC, werr := s.storage.Get(wm.StorageKey)
			if werr != nil {
				return nil, "", fmt.Errorf("get watermark file: %w", werr)
			}
			data, contentType, err = s.trf.ImageWatermark(rc, wmRC, p)
			_ = wmRC.Close()
		case queue.JobTypeImageSmartCrop:
			var p transform.SmartCropParams
			if err = json.Unmarshal(params, &p); err != nil {
				return nil, "", fmt.Errorf("parse smartcrop params: %w", err)
			}
			data, contentType, err = s.trf.ImageSmartCrop(rc, p)
		default:
			return nil, "", fmt.Errorf("unknown image job type: %s", jobType)
		}
		if err != nil {
			return nil, "", fmt.Errorf("transform: %w", err)
		}
		return data, contentType, nil
	}, nil
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
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.BgRemove(ctx, imageData, imagerouter.BgRemoveParams{
					Model:  p.Model,
					Prompt: p.Prompt,
				})
			},
		)
		if err != nil && strings.Contains(err.Error(), "Prompt must be a string") {
			return nil, "", errors.New(
				"the selected model is not available for background removal. please choose another model and try again",
			)
		}
		if err != nil {
			return nil, "", fmt.Errorf("remove background: %w", err)
		}
		return result, "image/png", nil
	}, nil
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
		result, err := s.runImageRouterJob(ctx, workspaceID, sourceKey,
			func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
				return client.Transform(ctx, imageData, imagerouter.PromptParams{
					Prompt: p.Prompt,
					Model:  p.Model,
				})
			},
		)
		if err != nil {
			return nil, "", fmt.Errorf("image transform with prompt: %w", err)
		}
		return result, "image/png", nil
	}, nil
}

func (s *JobServer) jobImageTransform(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.imageLocalTransformer(job.Type, p.Params, p.WorkspaceID)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeVariant(ctx, p, resolveVariantID(p), job.Type, data, contentType)
}

func (s *JobServer) jobImageBgRemove(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.imageBgRemoveTransformer(p.WorkspaceID, p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeVariant(ctx, p, resolveVariantID(p), job.Type, data, contentType)
}

func (s *JobServer) jobImageWithPrompt(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.imageWithPromptTransformer(p.WorkspaceID, p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeVariant(ctx, p, resolveVariantID(p), job.Type, data, contentType)
}

func (s *JobServer) rebuildImageVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	trf, err := s.imageLocalTransformer(variantType, json.RawMessage(paramsJSON), ver.WorkspaceID)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, variantType, paramsJSON, paramsHash, data, contentType)
}

func (s *JobServer) rebuildBgRemoveVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	trf, err := s.imageBgRemoveTransformer(ver.WorkspaceID, json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, queue.JobTypeImageBgRemove, paramsJSON, paramsHash, data, contentType)
}

func (s *JobServer) rebuildImageWithPromptVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	trf, err := s.imageWithPromptTransformer(ver.WorkspaceID, json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, queue.JobTypeImageWithPrompt, paramsJSON, paramsHash, data, contentType)
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
