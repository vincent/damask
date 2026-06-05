package jobs

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

// videoCaptureBuild is the variantBuildFn for video frame capture.
func (s *JobServer) videoCaptureBuild(_, _, _ string, params json.RawMessage) (variantTransformer, error) {
	return s.videoCaptureTransformer(params)
}

// videoCaptureCanonical returns canonical JSON for video capture params.
func videoCaptureCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.VideoThumbnailParams
	if len(params) > 0 {
		_ = json.Unmarshal(params, &p)
	}
	b, err := json.Marshal(p)
	return string(b), err
}

// videoCapturePostHook updates the asset thumbnail if none exists yet.
func (s *JobServer) videoCapturePostHook(ctx context.Context, p VariantJobPayload, _, storageKey, _ string) {
	asset, _ := s.queries.GetAssetByID(ctx, dbgen.GetAssetByIDParams{ID: p.AssetID, WorkspaceID: p.WorkspaceID})
	if asset.ThumbnailKey != nil {
		return
	}
	if err := s.queries.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
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
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_capture_image"),
		)
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		tmpPath, cleanup, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write temp: %w", err)
		}
		defer cleanup()
		data, err := s.trf.VideoExtractThumbnail(ctx, tmpPath, p)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("extract thumbnail: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, "image/jpeg", nil
	}, nil
}

// videoTranscodeBuild is the variantBuildFn for video transcoding.
func (s *JobServer) videoTranscodeBuild(_, _, _ string, params json.RawMessage) (variantTransformer, error) {
	return s.videoTranscodeTransformer(params)
}

// videoTranscodeCanonical returns canonical JSON for transcode params.
func videoTranscodeCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.TranscodeParams
	_ = json.Unmarshal(params, &p)
	if p.Format == "" {
		p.Format = transform.FormatMP4
	}
	b, err := json.Marshal(p)
	return string(b), err
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
		p.Format = transform.FormatMP4
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_transcode"),
		)
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_out" + ext
		defer os.Remove(dstPath)
		if e := s.trf.VideoTranscode(ctx, srcPath, dstPath, p); e != nil {
			telemetry.EndSpan(span, e)
			return nil, "", fmt.Errorf("transcode: %w", e)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.FormatVideoMimeType(p.Format), nil
	}, nil
}

// videoWatermarkBuild is the variantBuildFn for video watermarking.
func (s *JobServer) videoWatermarkBuild(_, _, workspaceID string, params json.RawMessage) (variantTransformer, error) {
	return s.videoWatermarkTransformer(workspaceID, params)
}

// videoWatermarkCanonical returns canonical JSON for video watermark params.
func videoWatermarkCanonical(_, _ string, params json.RawMessage) (string, error) {
	var p transform.VideoWatermarkParams
	_ = json.Unmarshal(params, &p)
	p.Normalize()
	b, err := json.Marshal(p)
	return string(b), err
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
	wm, err := s.queries.GetAssetByID(context.Background(), dbgen.GetAssetByIDParams{
		ID:          p.WatermarkAssetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, fmt.Errorf("get watermark asset: %w", err)
	}
	return func(ctx context.Context, sourceKey string) ([]byte, string, error) {
		ctx, span := telemetry.StartBackgroundSpan(ctx, "variant.transform",
			attribute.String("damask.variant_type", "video_watermark"),
		)
		rc, err := s.storage.Get(sourceKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", err
		}
		defer rc.Close()
		srcPath, cleanSrc, err := writeToTempFile(ctx, rc, filepath.Ext(sourceKey))
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("write src temp: %w", err)
		}
		defer cleanSrc()
		wmRC, err := s.storage.Get(wm.StorageKey)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("get watermark file: %w", err)
		}
		defer wmRC.Close()
		ext := transform.FormatExtension(p.Format)
		dstPath := srcPath + "_wm" + ext
		defer os.Remove(dstPath)
		if err := s.trf.VideoWatermark(ctx, srcPath, dstPath, wmRC, p); err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("watermark: %w", err)
		}
		data, err := os.ReadFile(dstPath)
		if err != nil {
			telemetry.EndSpan(span, err)
			return nil, "", fmt.Errorf("read output: %w", err)
		}
		telemetry.EndSpan(span, nil)
		return data, transform.FormatVideoMimeType(p.Format), nil
	}, nil
}

// assetVersionFromPayload constructs a minimal AssetVersion from a VariantJobPayload,
// so user-triggered video/audio jobs can reuse finalizeRebuildVariant.
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
