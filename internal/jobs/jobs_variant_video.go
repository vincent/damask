package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

)

const (
	videoFormatMP4  = "mp4"
	videoFormatWebM = "webm"
	mimeVideoMP4    = "video/mp4"
	mimeVideoWebM   = "video/webm"
)

func videoMimeType(format string) string {
	if format == videoFormatWebM {
		return mimeVideoWebM
	}
	return mimeVideoMP4
}

// videoCaptureTransformer returns a variantTransformer for video frame capture.
func (s *JobServer) videoCaptureTransformer(params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.VideoThumbnailParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		tmpPath, cleanup, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			return nil, "", fmt.Errorf("write temp: %w", err)
		}
		defer cleanup()
		data, err := s.trf.VideoExtractThumbnail(ctx, tmpPath, p)
		if err != nil {
			return nil, "", fmt.Errorf("extract thumbnail: %w", err)
		}
		return data, "image/jpeg", nil
	}, nil
}

// videoTranscodeTransformer returns a variantTransformer for video transcoding.
func (s *JobServer) videoTranscodeTransformer(params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.TranscodeParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse transcode params: %w", err)
		}
	}
	if p.Format == "" {
		p.Format = videoFormatMP4
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_out" + ext
		defer os.Remove(dstPath)
		if err := s.trf.VideoTranscode(ctx, srcPath, dstPath, p); err != nil {
			return nil, "", fmt.Errorf("transcode: %w", err)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		return data, videoMimeType(p.Format), nil
	}, nil
}

// videoWatermarkTransformer returns a variantTransformer for video watermarking.
func (s *JobServer) videoWatermarkTransformer(workspaceID string, params json.RawMessage) (variantTransformer, error) {
	if !s.trf.FFmpegAvailable() {
		return nil, errors.New("ffmpeg not found in PATH")
	}
	var p transform.VideoWatermarkParams
	if len(params) > 0 {
		if err := json.Unmarshal(params, &p); err != nil {
			return nil, fmt.Errorf("parse watermark params: %w", err)
		}
	}
	p.Normalize()
	if p.WatermarkAssetID == "" {
		return nil, errors.New("watermark asset id is required")
	}
	wm, err := s.db.GetAssetByID(context.Background(), dbgen.GetAssetByIDParams{
		ID:          p.WatermarkAssetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("get watermark asset: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()
		wmRC, err := s.storage.Get(wm.StorageKey)
		if err != nil {
			return nil, "", fmt.Errorf("get watermark file: %w", err)
		}
		defer wmRC.Close()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_wm" + ext
		defer os.Remove(dstPath)
		if err := s.trf.VideoWatermark(ctx, srcPath, dstPath, wmRC, p); err != nil {
			return nil, "", fmt.Errorf("watermark: %w", err)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		return data, videoMimeType(p.Format), nil
	}, nil
}

func (s *JobServer) jobVideoCaptureImage(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	if len(p.StorageKey) == 0 {
		return fmt.Errorf("storage key is empty: %s", job.ID)
	}
	trf, err := s.videoCaptureTransformer(p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}

	// Normalise params so the storage key is stable (same as the transformer parsed).
	var captureParams transform.VideoThumbnailParams
	if len(p.Params) > 0 {
		_ = json.Unmarshal(p.Params, &captureParams)
	}
	paramsJSON, _ := json.Marshal(captureParams)
	pj := string(paramsJSON)
	paramsHash := CanonicalParamsHash(pj)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, queue.JobTypeVideoCaptureImage, paramsHash, ".jpg")

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	variantID := resolveVariantID(p)
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
	if err != nil {
		return err
	}
	s.publishVariantReady(ctx, p.WorkspaceID, p.AssetID, variantID)
	s.enqueueVariantThumb(ctx, p, variantID, storageKey, contentType)

	// Set as asset thumbnail if none yet.
	asset, _ := s.db.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: p.AssetID, WorkspaceID: p.WorkspaceID})
	if asset.ThumbnailKey == nil {
		if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey: &storageKey,
			ID:           p.AssetID,
		}); err == nil {
			s.hub.Publish(ctx, p.WorkspaceID, events.Event{
				Type:         "thumbnail_ready",
				AssetID:      p.AssetID,
				ThumbnailKey: storageKey,
			})
		}
	}
	return nil
}

func (s *JobServer) jobVideoTranscode(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.videoTranscodeTransformer(p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}
	// Use normalised params for the storage key (matches original behaviour).
	var params transform.TranscodeParams
	_ = json.Unmarshal(p.Params, &params)
	if params.Format == "" {
		params.Format = videoFormatMP4
	}
	pj, _ := json.Marshal(params)
	cj := string(pj)
	return s.finalizeRebuildVariant(ctx, assetVersionFromPayload(p), job.Type, cj, CanonicalParamsHash(cj), data, contentType)
}

func (s *JobServer) jobVideoWatermark(ctx context.Context, job dbgen.Job) error {
	var p VariantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}
	trf, err := s.videoWatermarkTransformer(p.WorkspaceID, p.Params)
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, p.StorageKey)
	if err != nil {
		return err
	}
	// Use normalised params for the storage key (matches original behaviour).
	var params transform.VideoWatermarkParams
	_ = json.Unmarshal(p.Params, &params)
	params.Normalize()
	pj, _ := json.Marshal(params)
	cj := string(pj)
	return s.finalizeRebuildVariant(ctx, assetVersionFromPayload(p), job.Type, cj, CanonicalParamsHash(cj), data, contentType)
}

func (s *JobServer) rebuildVideoCaptureVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	trf, err := s.videoCaptureTransformer(json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, queue.JobTypeVideoCaptureImage, paramsJSON, paramsHash, data, contentType)
}

func (s *JobServer) rebuildVideoTranscodeVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	trf, err := s.videoTranscodeTransformer(json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, queue.JobTypeVideoTranscode, paramsJSON, paramsHash, data, contentType)
}

func (s *JobServer) rebuildVideoWatermarkVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	trf, err := s.videoWatermarkTransformer(ver.WorkspaceID, json.RawMessage(paramsJSON))
	if err != nil {
		return err
	}
	data, contentType, err := trf(ctx, ver.StorageKey)
	if err != nil {
		return err
	}
	return s.finalizeRebuildVariant(ctx, ver, queue.JobTypeVideoWatermark, paramsJSON, paramsHash, data, contentType)
}

// assetVersionFromPayload constructs a minimal AssetVersion from a VariantJobPayload,
// so user-triggered video/audio jobs can reuse finalizeRebuildVariant (which uses
// CreateVariant, matching the original behaviour for these job types).
func assetVersionFromPayload(p VariantJobPayload) dbgen.AssetVersion {
	return dbgen.AssetVersion{
		ID:          p.VersionID,
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		VersionNum:  p.VersionNum,
		StorageKey:  p.StorageKey,
		MimeType:    p.MimeType,
	}
}


