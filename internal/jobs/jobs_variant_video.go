package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

func (s *JobServer) jobVideoCaptureImage(ctx context.Context, job dbgen.Job) error {
	if !s.trf.FFmpegAvailable() {
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

	data, err := s.trf.VideoExtractThumbnail(ctx, tmpPath, params)
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
			s.hub.Publish(ctx, p.WorkspaceID, events.Event{
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

func (s *JobServer) jobVideoTranscode(ctx context.Context, job dbgen.Job) error {
	if !s.trf.FFmpegAvailable() {
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

	if err := s.trf.VideoTranscode(ctx, srcPath, dstPath, params); err != nil {
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

func (s *JobServer) rebuildVideoCaptureVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	if !s.trf.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var params transform.VideoThumbnailParams
	if paramsJSON != "" && paramsJSON != "{}" {
		_ = json.Unmarshal([]byte(paramsJSON), &params)
	}

	rc, err := s.storage.Get(ver.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(ver.StorageKey)
	tmpPath, cleanup, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write temp: %w", err)
	}
	defer cleanup()

	data, err := s.trf.VideoExtractThumbnail(ctx, tmpPath, params)
	if err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeVideoCaptureImage, paramsHash, ".jpg")
	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeVideoCaptureImage,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, "image/jpeg")
	}
	return err
}

func (s *JobServer) rebuildVideoTranscodeVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	if !s.trf.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var params transform.TranscodeParams
	if paramsJSON != "" && paramsJSON != "{}" {
		if err := json.Unmarshal([]byte(paramsJSON), &params); err != nil {
			return fmt.Errorf("parse transcode params: %w", err)
		}
	}
	if params.Format == "" {
		params.Format = "mp4"
	}

	rc, err := s.storage.Get(ver.StorageKey)
	if err != nil {
		return err
	}
	defer rc.Close()

	srcExt := filepath.Ext(ver.StorageKey)
	srcPath, cleanSrc, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.FormatExtension(params.Format)
	dstPath := srcPath + "_out" + ext
	defer os.Remove(dstPath)

	if err := s.trf.VideoTranscode(ctx, srcPath, dstPath, params); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}

	dstData, err := os.ReadFile(dstPath)
	if err != nil {
		return fmt.Errorf("read output: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeVideoTranscode, paramsHash, ext)
	if err := s.storage.Put(storageKey, bytes.NewReader(dstData)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	outputMime := "video/mp4"
	if params.Format == "webm" {
		outputMime = "video/webm"
	}
	sz := int64(len(dstData))
	vid := uuid.NewString()
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeVideoTranscode,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, outputMime)
	}
	return err
}
