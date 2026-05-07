package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/imagerouter"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// VariantJobPayload is the payload for user-triggered variant creation jobs.
// VersionID and VersionNum identify the asset version the variant is bound to.
type VariantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
	VersionID   string          `json:"version_id"`
	VersionNum  int64           `json:"version_num"`
	StorageKey  string          `json:"storage_key"`
	MimeType    string          `json:"mime_type"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
}

// enqueueVariantThumb enqueues a generate_variant_thumbnail job after a variant row is created.
func (s *JobServer) enqueueVariantThumb(ctx context.Context, p VariantJobPayload, variantID, storageKey, contentType string) {
	_ = EnqueueVariantThumbnailJob(ctx, s, VariantThumbnailJobPayload{
		VariantID:   variantID,
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		StorageKey:  storageKey,
		MimeType:    contentType,
	})
}

func (s *JobServer) jobImageTransform(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	var data []byte
	var contentType string

	switch job.Type {
	case queue.JobTypeImageResize:
		var params transform.ResizeParams
		if err = json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse resize params: %w", err)
		}
		data, contentType, err = s.trf.ImageResize(rc, params)
	case queue.JobTypeImageConvert:
		var params transform.ConvertParams
		if err = json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = s.trf.ImageConvert(rc, params)
	case queue.JobTypeImageCrop:
		var params transform.CropParams
		if err = json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = s.trf.ImageCrop(rc, params)
	case queue.JobTypeImageWatermark:
		var params transform.WatermarkParams
		if err = json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		if params.WatermarkAssetID == "" {
			return fmt.Errorf("watermark asset id is required")
		}
		var wm dbgen.Asset
		wm, err = s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
			ID:          params.WatermarkAssetID,
			WorkspaceID: p.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("get watermark asset: %w", err)
		}
		var wmRC io.ReadCloser
		wmRC, err = s.storage.Get(wm.StorageKey)
		if err != nil {
			return fmt.Errorf("get watermark file: %w", err)
		}
		data, contentType, err = s.trf.ImageWatermark(rc, wmRC, params)
		_ = wmRC.Close()
	case queue.JobTypeImageSmartCrop:
		var params transform.SmartCropParams
		if err = json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse smartcrop params: %w", err)
		}
		data, contentType, err = s.trf.ImageSmartCrop(rc, params)
	default:
		return fmt.Errorf("unknown image job type: %s", job.Type)
	}
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	ext := MimeToExt(contentType)
	variantID := uuid.NewString()
	paramsStr := string(p.Params)
	paramsHash := canonicalParamsHash(paramsStr)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, job.Type, paramsHash, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            job.Type,
		StorageKey:      storageKey,
		TransformParams: &paramsStr,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, contentType)
	}
	return err
}

func (s *JobServer) jobImageBgRemove(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal(p.Params, &params); err != nil {
		return fmt.Errorf("parse bg remove params: %w", err)
	}

	result, err := s.runImageRouterJob(ctx, p.StorageKey, func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
		return client.BgRemove(ctx, imageData, imagerouter.BgRemoveParams{Model: params.Model})
	})
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	variantID := uuid.NewString()
	paramsStr := string(p.Params)
	paramsHash := canonicalParamsHash(paramsStr)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, queue.JobTypeImageBgRemove, paramsHash, ".png")

	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(result))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            queue.JobTypeImageBgRemove,
		StorageKey:      storageKey,
		TransformParams: &paramsStr,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, "image/png")
	}
	return err
}

func (s *JobServer) jobImageWithPrompt(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal(p.Params, &params); err != nil {
		return fmt.Errorf("parse image prompt params: %w", err)
	}

	result, err := s.runImageRouterJob(ctx, p.StorageKey, func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
		return client.Transform(ctx, imageData, imagerouter.PromptParams{
			Prompt: params.Prompt,
			Model:  params.Model,
		})
	})
	if err != nil {
		return fmt.Errorf("image transform with prompt: %w", err)
	}

	variantID := uuid.NewString()
	paramsStr := string(p.Params)
	paramsHash := canonicalParamsHash(paramsStr)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, queue.JobTypeImageWithPrompt, paramsHash, ".png")

	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(result))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            queue.JobTypeImageWithPrompt,
		StorageKey:      storageKey,
		TransformParams: &paramsStr,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, "image/png")
	}
	return err
}

func (s *JobServer) rebuildImageVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	rc, err := s.storage.Get(ver.StorageKey)
	if err != nil {
		return fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	rawParams := json.RawMessage(paramsJSON)
	var (
		data        []byte
		contentType string
	)

	switch variantType {
	case queue.JobTypeImageResize:
		var params transform.ResizeParams
		if err = json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse resize params: %w", err)
		}
		data, contentType, err = s.trf.ImageResize(rc, params)
	case queue.JobTypeImageConvert:
		var params transform.ConvertParams
		if err = json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = s.trf.ImageConvert(rc, params)
	case queue.JobTypeImageCrop:
		var params transform.CropParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = s.trf.ImageCrop(rc, params)
	case queue.JobTypeImageWatermark:
		var params transform.WatermarkParams
		if err = json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		if params.WatermarkAssetID == "" {
			return fmt.Errorf("watermark asset id is required")
		}
		var wm dbgen.Asset
		wm, err = s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{
			ID:          params.WatermarkAssetID,
			WorkspaceID: ver.WorkspaceID,
		})
		if err != nil {
			return fmt.Errorf("get watermark asset: %w", err)
		}
		var wmRC io.ReadCloser
		wmRC, err = s.storage.Get(wm.StorageKey)
		if err != nil {
			return fmt.Errorf("get watermark file: %w", err)
		}
		data, contentType, err = s.trf.ImageWatermark(rc, wmRC, params)
		_ = wmRC.Close()
	case queue.JobTypeImageSmartCrop:
		var params transform.SmartCropParams
		if err = json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse smartcrop params: %w", err)
		}
		data, contentType, err = s.trf.ImageSmartCrop(rc, params)
	}
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	ext := MimeToExt(contentType)
	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, variantType, paramsHash, ext)
	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            variantType,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, contentType)
	}
	return err
}

func (s *JobServer) rebuildBgRemoveVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	var params struct {
		Model string `json:"model"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return fmt.Errorf("parse bg remove params: %w", err)
	}

	result, err := s.runImageRouterJob(ctx, ver.StorageKey, func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
		return client.BgRemove(ctx, imageData, imagerouter.BgRemoveParams{Model: params.Model})
	})
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeImageBgRemove, paramsHash, ".png")
	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(result))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeImageBgRemove,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, "image/png")
	}
	return err
}

func (s *JobServer) rebuildImageWithPromptVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	var params struct {
		Prompt string `json:"prompt"`
		Model  string `json:"model"`
	}
	if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
		return fmt.Errorf("parse image prompt params: %w", err)
	}

	result, err := s.runImageRouterJob(ctx, ver.StorageKey, func(client *imagerouter.Client, imageData []byte) ([]byte, error) {
		return client.Transform(ctx, imageData, imagerouter.PromptParams{
			Prompt: params.Prompt,
			Model:  params.Model,
		})
	})
	if err != nil {
		return fmt.Errorf("image transform with prompt: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeImageWithPrompt, paramsHash, ".png")
	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(result))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeImageWithPrompt,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, "image/png")
	}
	return err
}

func (s *JobServer) runImageRouterJob(
	ctx context.Context,
	sourceKey string,
	callFn func(*imagerouter.Client, []byte) ([]byte, error),
) ([]byte, error) {
	rc, err := s.storage.Get(sourceKey)
	if err != nil {
		return nil, fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	imageData, err := io.ReadAll(rc)
	if err != nil {
		return nil, fmt.Errorf("read source: %w", err)
	}

	client := imagerouter.NewClient(s.cfg.ImageRouter.APIKey, s.cfg.ImageRouter.RetryPaidOnFreeLimit)
	result, err := callFn(client, imageData)
	if err != nil {
		return nil, err
	}
	return result, nil
}
