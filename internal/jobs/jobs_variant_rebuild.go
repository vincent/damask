package jobs

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"sort"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
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
			slog.ErrorContext(ctx, "rebuild-variants: dedup check", "version_id", p.NewVersionID, "type", spec.Type, "error", lookupErr)
			continue
		}

		if err := s.rebuildOneVariant(ctx, newVer, spec.Type, paramsStr, paramsHash); err != nil {
			slog.ErrorContext(ctx, "rebuild-variants: variant failed", "version_id", p.NewVersionID, "type", spec.Type, "error", err)
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

	case queue.JobTypeVideoWatermark:
		return s.rebuildVideoWatermarkVariant(ctx, ver, paramsJSON, paramsHash)

	case queue.JobTypeExtractAudio, queue.JobTypeTranscodeAudio, queue.JobTypeNormalizeAudio:
		return s.rebuildAudioVariant(ctx, ver, variantType, paramsJSON, paramsHash)

	default:
		return fmt.Errorf("unknown variant type for rebuild: %s", variantType)
	}
}
