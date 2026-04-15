package jobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"os"
	"path/filepath"
	"sort"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// RebuildVariantsPayload is the job payload for rebuild_variants.
type RebuildVariantsPayload struct {
	AssetID         string `json:"asset_id"`
	NewVersionID    string `json:"new_version_id"`
	SourceVersionID string `json:"source_version_id"` // version whose variant params to copy
}

// canonicalParamsHash returns the first 8 hex chars of SHA-256 of the
// canonical (sorted-key) JSON representation of the params string.
// This ensures two logically identical param sets always produce the same hash.
func canonicalParamsHash(paramsJSON string) string {
	// Parse into a generic map so we can sort keys deterministically.
	var m map[string]any
	if err := json.Unmarshal([]byte(paramsJSON), &m); err != nil {
		// Fall back to hashing the raw string.
		h := sha256.Sum256([]byte(paramsJSON))
		return hex.EncodeToString(h[:])[:8]
	}
	b, _ := json.Marshal(sortedMapJSON(m))
	h := sha256.Sum256(b)
	return hex.EncodeToString(h[:])[:8]
}

// sortedMapJSON recursively sorts map keys so JSON marshalling is deterministic.
func sortedMapJSON(m map[string]any) map[string]any {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	sort.Strings(keys)
	out := make(map[string]any, len(m))
	for _, k := range keys {
		v := m[k]
		if sub, ok := v.(map[string]any); ok {
			v = sortedMapJSON(sub)
		}
		out[k] = v
	}
	return out
}

// jobRebuildVariants copies all non-manual variant definitions from
// source_version_id and recreates them against new_version_id.
// The job is idempotent: GetVariantByTypeAndParams guards against duplicates.
func (s *JobServer) jobRebuildVariants(ctx context.Context, job dbgen.Job) error {
	var p RebuildVariantsPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	// Load the new version to get storage key, mime, workspace, version num.
	newVer, err := s.db.GetVersionByIDUnchecked(ctx, p.NewVersionID)
	if err != nil {
		return fmt.Errorf("load new version: %w", err)
	}

	// Copy variant params from the source version (manual excluded by query).
	specs, err := s.db.CopyVariantParamsByVersion(ctx, p.SourceVersionID)
	if err != nil {
		return fmt.Errorf("copy variant params: %w", err)
	}
	if len(specs) == 0 {
		return nil // nothing to rebuild
	}

	for _, spec := range specs {
		paramsStr := "{}"
		if spec.TransformParams != nil {
			paramsStr = *spec.TransformParams
		}
		paramsHash := canonicalParamsHash(paramsStr)

		// Idempotency guard: skip if already rebuilt.
		canonicalParams := paramsStr
		_, lookupErr := s.db.GetVariantByTypeAndParams(ctx, dbgen.GetVariantByTypeAndParamsParams{
			AssetVersionID:  p.NewVersionID,
			Type:            spec.Type,
			TransformParams: &canonicalParams,
		})
		if lookupErr == nil {
			// Already exists.
			continue
		}
		if !errors.Is(lookupErr, sql.ErrNoRows) {
			slog.Error("rebuild-variants: dedup check", "version_id", p.NewVersionID, "type", spec.Type, "error", lookupErr)
			continue
		}

		if err := s.rebuildOneVariant(ctx, newVer, spec.Type, paramsStr, paramsHash); err != nil {
			slog.Error("rebuild-variants: variant failed", "version_id", p.NewVersionID, "type", spec.Type, "error", err)
			// Continue with remaining variants even if one fails.
		}
	}

	return nil
}

// rebuildOneVariant runs the transform for a single variant spec and writes the result.
func (s *JobServer) rebuildOneVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
) error {
	switch variantType {
	case queue.JobTypeImageResize, queue.JobTypeImageConvert,
		queue.JobTypeImageCrop, queue.JobTypeImageWatermark,
		queue.JobTypeImageSmartCrop:
		return s.rebuildImageVariant(ctx, ver, variantType, paramsJSON, paramsHash)

	case queue.JobTypeImageBgRemove:
		return s.rebuildBgRemoveVariant(ctx, ver, paramsHash)

	case queue.JobTypeVideoCaptureImage:
		return s.rebuildVideoCaptureVariant(ctx, ver, paramsJSON, paramsHash)

	case queue.JobTypeVideoTranscode:
		return s.rebuildVideoTranscodeVariant(ctx, ver, paramsJSON, paramsHash)

	default:
		return fmt.Errorf("unknown variant type for rebuild: %s", variantType)
	}
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
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse resize params: %w", err)
		}
		data, contentType, err = transform.ImageResize(rc, params)
	case queue.JobTypeImageConvert:
		var params transform.ConvertParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse convert params: %w", err)
		}
		data, contentType, err = transform.ImageConvert(rc, params)
	case queue.JobTypeImageCrop:
		var params transform.CropParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse crop params: %w", err)
		}
		data, contentType, err = transform.ImageCrop(rc, params)
	case queue.JobTypeImageWatermark:
		var params transform.WatermarkParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse watermark params: %w", err)
		}
		data, contentType, err = transform.ImageWatermark(rc, params)
	case queue.JobTypeImageSmartCrop:
		var params transform.SmartCropParams
		if err := json.Unmarshal(rawParams, &params); err != nil {
			return fmt.Errorf("parse smartcrop params: %w", err)
		}
		data, contentType, err = transform.ImageSmartCrop(rc, params)
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
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              uuid.NewString(),
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            variantType,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	return err
}

func (s *JobServer) rebuildBgRemoveVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsHash string,
) error {
	if s.cfg.RemoveBgAPIKey == "" {
		return fmt.Errorf("REMOVEBG_API_KEY not configured")
	}

	rc, err := s.storage.Get(ver.StorageKey)
	if err != nil {
		return fmt.Errorf("get source: %w", err)
	}
	defer rc.Close()

	imgData, err := io.ReadAll(rc)
	if err != nil {
		return fmt.Errorf("read source: %w", err)
	}

	result, err := transform.RemoveBackground(ctx, imgData, s.cfg.RemoveBgAPIKey)
	if err != nil {
		return fmt.Errorf("remove background: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeImageBgRemove, paramsHash, ".png")
	if err := s.storage.Put(storageKey, bytes.NewReader(result)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	emptyParams := "{}"
	sz := int64(len(result))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              uuid.NewString(),
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeImageBgRemove,
		StorageKey:      storageKey,
		TransformParams: &emptyParams,
		Size:            &sz,
	})
	return err
}

func (s *JobServer) rebuildVideoCaptureVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	if !transform.FFmpegAvailable() {
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

	data, err := transform.VideoExtractThumbnail(ctx, tmpPath, params)
	if err != nil {
		return fmt.Errorf("extract thumbnail: %w", err)
	}

	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, queue.JobTypeVideoCaptureImage, paramsHash, ".jpg")
	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              uuid.NewString(),
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeVideoCaptureImage,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	return err
}

func (s *JobServer) rebuildVideoTranscodeVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	paramsJSON, paramsHash string,
) error {
	if !transform.FFmpegAvailable() {
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

	if err := transform.VideoTranscode(ctx, srcPath, dstPath, params); err != nil {
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

	sz := int64(len(dstData))
	_, err = s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              uuid.NewString(),
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            queue.JobTypeVideoTranscode,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	return err
}
