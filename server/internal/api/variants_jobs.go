package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// RegisterJobHandlers wires transform job handlers into the queue.
func (s *Server) RegisterJobHandlers() {
	s.queue.Register(queue.JobTypeImageThumbnail, s.jobImageThumbnail)
	s.queue.Register(queue.JobTypeImageResize, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageConvert, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageCrop, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageWatermark, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageSmartCrop, s.jobImageTransform)
	s.queue.Register(queue.JobTypeVideoThumbnail, s.jobVideoThumbnail)
	s.queue.Register(queue.JobTypeVideoTranscode, s.jobVideoTranscode)
	s.queue.Register(queue.JobTypeImageBgRemove, s.jobImageBgRemove)
	s.queue.Register(queue.JobTypePdfThumbnail, s.jobMagikFirstThumbnail)
	s.queue.Register(queue.JobTypeAudioWaveform, s.jobAudioWaveform)
}

type variantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
	StorageKey  string          `json:"storage_key"`
	MimeType    string          `json:"mime_type"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
}

type thumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
}

func (s *Server) jobImageThumbnail(ctx context.Context, job dbgen.Job) error {
	var p thumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	data, _, err := transform.Resize(rc, transform.ResizeParams{
		Width:   400,
		Height:  400,
		Fit:     "contain",
		Quality: 85,
		Format:  "jpeg",
	})
	if err != nil {
		return fmt.Errorf("thumb: %w", err)
	}

	thumbKey := fmt.Sprintf("%s/%s/thumb.jpg", p.WorkspaceID, p.AssetID)
	if err := s.storage.Put(thumbKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: &thumbKey,
		ID:           p.AssetID,
	}); err != nil {
		return err
	}
	s.hub.Publish(p.WorkspaceID, Event{
		Type:         "thumbnail_ready",
		AssetID:      p.AssetID,
		ThumbnailKey: thumbKey,
	})
	return nil
}

func (s *Server) jobMagikFirstThumbnail(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	thumbData, contentType, err := transform.MagikFirstThumbnail(ctx, rc, p.MimeType)
	if err != nil {
		return fmt.Errorf("magik first thumbnail: %w", err)
	}

	ext := mimeToExt(contentType)
	thumbKey := fmt.Sprintf("%s/%s/thumb."+ext, p.WorkspaceID, p.AssetID)
	if err := s.storage.Put(thumbKey, bytes.NewReader(thumbData)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: &thumbKey,
		ID:           p.AssetID,
	}); err != nil {
		return err
	}
	s.hub.Publish(p.WorkspaceID, Event{
		Type:         "thumbnail_ready",
		AssetID:      p.AssetID,
		ThumbnailKey: thumbKey,
	})
	return nil
}

func (s *Server) jobAudioWaveform(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	thumbData, contentType, err := transform.AudioWaveform(ctx, rc, p.MimeType)
	if err != nil {
		return fmt.Errorf("magik first thumbnail: %w", err)
	}

	if len(thumbData) <= 0 {
		log.Println("thumbnail: empty waveform result", p.StorageKey)
	} else {
		ext := mimeToExt(contentType)
		thumbKey := fmt.Sprintf("%s/%s/thumb."+ext, p.WorkspaceID, p.AssetID)
		if err := s.storage.Put(thumbKey, bytes.NewReader(thumbData)); err != nil {
			return fmt.Errorf("store thumb: %w", err)
		}

		if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey: &thumbKey,
			ID:           p.AssetID,
		}); err != nil {
			return err
		}
		s.hub.Publish(p.WorkspaceID, Event{
			Type:         "thumbnail_ready",
			AssetID:      p.AssetID,
			ThumbnailKey: thumbKey,
		})
	}

	return nil
}

func (s *Server) jobImageTransform(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
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

	ext := mimeToExt(contentType)
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

func (s *Server) jobVideoThumbnail(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p variantJobPayload
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

	srcExt := filepath.Ext(p.StorageKey)
	tmpPath, cleanup, err := s.writeToTempFile(ctx, p.StorageKey, srcExt)
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
			s.hub.Publish(p.WorkspaceID, Event{
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
		Type:            queue.JobTypeVideoThumbnail,
		StorageKey:      storageKey,
		TransformParams: &pj,
		Size:            &sz,
	})
	return err
}

func (s *Server) jobVideoTranscode(ctx context.Context, job dbgen.Job) error {
	if !transform.FFmpegAvailable() {
		return fmt.Errorf("ffmpeg not found in PATH")
	}

	var p variantJobPayload
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

	srcExt := filepath.Ext(p.StorageKey)
	srcPath, cleanSrc, err := s.writeToTempFile(ctx, p.StorageKey, srcExt)
	if err != nil {
		return fmt.Errorf("write src temp: %w", err)
	}
	defer cleanSrc()

	ext := transform.TranscodeExtension(params.Format)
	dstPath := srcPath + "_out" + ext
	defer removeFile(dstPath)

	if err := transform.TranscodeVideo(ctx, srcPath, dstPath, params); err != nil {
		return fmt.Errorf("transcode: %w", err)
	}

	dstData, err := readFile(dstPath)
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

func (s *Server) jobImageBgRemove(ctx context.Context, job dbgen.Job) error {
	var p variantJobPayload
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

	result, err := transform.RemoveBackground(ctx, imgData, s.removeBgAPIKey)
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

// ---- OS helpers ----

func (s *Server) writeToTempFile(ctx context.Context, storageKey, ext string) (string, func(), error) {
	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	if _, copyErr := io.Copy(f, rc); copyErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("copy to temp: %w", copyErr)
	}
	err = f.Close()
	if err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

func readFile(path string) ([]byte, error) {
	return os.ReadFile(path)
}

func removeFile(path string) {
	_ = os.Remove(path)
}

// ---- Mime helpers ----

func mimeToExt(ct string) string {
	ms, err := mime.ExtensionsByType(ct)
	if err == nil && len(ms) > 0 {
		return ms[0]
	}
	return "application/octet-stream"
}
