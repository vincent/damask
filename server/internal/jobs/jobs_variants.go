package jobs

import (
	"bytes"
	"context"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"github.com/google/uuid"
)

// ---- Payload types ----

type VariantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
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

	data, err := transform.ExtractVideoThumbnail(ctx, tmpPath, params)
	if err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}

	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s.jpg", p.WorkspaceID, p.AssetID, variantID)
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

	paramsJSON, _ := json.Marshal(params)
	pj := string(paramsJSON)
	sz := int64(len(data))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            queue.JobTypeVideoCaptureImage,
		StorageKey:      storageKey,
		TransformParams: &pj,
		Size:            &sz,
	})
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
		data, contentType, err = transform.Resize(rc, params)
	case queue.JobTypeImageConvert:
		var params transform.ConvertParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = transform.Convert(rc, params)
	case queue.JobTypeImageCrop:
		var params transform.CropParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = transform.Crop(rc, params)
	case queue.JobTypeImageWatermark:
		var params transform.WatermarkParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		data, contentType, err = transform.Watermark(rc, params)
	case queue.JobTypeImageSmartCrop:
		var params transform.SmartCropParams
		if err := json.Unmarshal(p.Params, &params); err != nil {
			return fmt.Errorf("parse smartcrop params: %w", err)
		}
		data, contentType, err = transform.SmartCrop(rc, params)
	default:
		return fmt.Errorf("unknown image job type: %s", job.Type)
	}
	if err != nil {
		return fmt.Errorf("transform: %w", err)
	}

	ext := MimeToExt(contentType)
	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s%s", p.WorkspaceID, p.AssetID, variantID, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	paramsJSON := string(p.Params)
	sz := int64(len(data))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            job.Type,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
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

	ext := transform.TranscodeExtension(params.Format)
	dstPath := srcPath + "_out" + ext
	defer os.Remove(dstPath)

	if err := transform.TranscodeVideo(ctx, srcPath, dstPath, params); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}

	variantID := uuid.NewString()
	storageKey := fmt.Sprintf("%s/%s/variants/%s%s", p.WorkspaceID, p.AssetID, variantID, ext)
	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	paramsJSON, _ := json.Marshal(params)
	pj := string(paramsJSON)
	sz := int64(len(dstData))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              variantID,
		AssetID:         p.AssetID,
		WorkspaceID:     p.WorkspaceID,
		Type:            queue.JobTypeVideoTranscode,
		StorageKey:      storageKey,
		TransformParams: &pj,
		Size:            &sz,
	})
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
	storageKey := fmt.Sprintf("%s/%s/variants/%s.png", p.WorkspaceID, p.AssetID, variantID)
	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(result))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:          variantID,
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
		Type:        queue.JobTypeImageBgRemove,
		StorageKey:  storageKey,
		Size:        &sz,
	})
	return err
}
