package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// ---- Payload types ----

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

// ---- Variant jobs — user-triggered ----

func (s *JobServer) jobVideoCaptureImage(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	if len(p.StorageKey) == 0 {
		return fmt.Errorf("storage key is empty: %s", job.ID)
	}

	var params transform.VideoThumbnailParams
	if len(p.Params) > 0 {
		_ = json.Unmarshal(p.Params, &params)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(p.StorageKey)
	tmpPath, cleanup, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	defer cleanup()

	data, err := transform.VideoExtractThumbnail(ctx, tmpPath, params)
	if err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}

	variantID := uuid.NewString()
	paramsJSON, _ := json.Marshal(params)
	paramsHash := canonicalParamsHash(string(paramsJSON))
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, queue.JobTypeVideoCaptureImage, paramsHash, ".jpg")

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	// Set as asset thumbnail if none yet.
	asset, _ := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: p.AssetID, WorkspaceID: p.WorkspaceID})
	if asset.ThumbnailKey == nil {
		if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey: &storageKey,
			ID:           p.AssetID,
		}); err == nil {
			s.hub.Publish(p.WorkspaceID, events.Event{
				Type:         "thumbnail_ready",
				AssetID:      p.AssetID,
				ThumbnailKey: storageKey,
			})
		}
	}

	pj := string(paramsJSON)
	sz := int64(len(data))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            queue.JobTypeVideoCaptureImage,
		StorageKey:      storageKey,
		TransformParams: &pj,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, "image/jpeg")
	}
	return err
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
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse resize params: %w", err)
		}
		data, contentType, err = transform.ImageResize(rc, params)
	case queue.JobTypeImageConvert:
		var params transform.ConvertParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = transform.ImageConvert(rc, params)
	case queue.JobTypeImageCrop:
		var params transform.CropParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = transform.ImageCrop(rc, params)
	case queue.JobTypeImageWatermark:
		var params transform.WatermarkParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		data, contentType, err = transform.ImageWatermark(rc, params)
	case queue.JobTypeImageSmartCrop:
		var params transform.SmartCropParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse smartcrop params: %w", err)
		}
		data, contentType, err = transform.ImageSmartCrop(rc, params)
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

func (s *JobServer) jobVideoTranscode(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	var params transform.TranscodeParams
	if len(p.Params) > 0 {
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse transcode params: %w", err)
		}
	}
	if params.Format == "" {
		params.Format = "mp4"
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(p.StorageKey)
	srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.FormatExtension(params.Format)
	dstPath := srcPath + "_out" + ext
	defer os.Remove(dstPath)

	if err := transform.VideoTranscode(ctx, srcPath, dstPath, params); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}

	variantID := uuid.NewString()
	paramsJSON, _ := json.Marshal(params)
	pj := string(paramsJSON)
	paramsHash := canonicalParamsHash(pj)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, queue.JobTypeVideoTranscode, paramsHash, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(dstData))
	outputMime := "video/mp4"
	if params.Format == "webm" {
		outputMime = "video/webm"
	}
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            queue.JobTypeVideoTranscode,
		StorageKey:      storageKey,
		TransformParams: &pj,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, outputMime)
	}
	return err
}

func (s *JobServer) jobImageBgRemove(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	imgData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read file: %w", err)
	}

	result, err := transform.RemoveBackground(ctx, imgData, s.cfg.RemoveBgAPIKey)
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	variantID := uuid.NewString()
	emptyParams := "{}"
	paramsHash := canonicalParamsHash(emptyParams)
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
		TransformParams: &emptyParams,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, "image/png")
	}
	return err
}
