package jobs

import (
	"bytes"
	"context"
	"fmt"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

// variantTransformer executes a media transform given a storage source key
// and returns (data, contentType, error).
type variantTransformer func(ctx context.Context, sourceKey string) ([]byte, string, error)

// resolveVariantID returns p.VariantID if set, otherwise generates a new UUID.
func resolveVariantID(p VariantJobPayload) string {
	if p.VariantID != "" {
		return p.VariantID
	}
	return uuid.NewString()
}

// finalizeVariant stores the transform output, creates the variant DB row (with
// title and is_shared from the payload), then publishes the ready event and
// enqueues the thumbnail job. Used by user-triggered variant jobs.
func (s *JobServer) finalizeVariant(
	ctx context.Context,
	p VariantJobPayload,
	variantID, jobType string,
	data []byte,
	contentType string,
) error {
	ext := transform.MimeToExt(contentType)
	paramsStr := string(p.Params)
	paramsHash := CanonicalParamsHash(paramsStr)
	storageKey := storage.VersionedVariantKey(p.WorkspaceID, p.AssetID, p.VersionNum, jobType, paramsHash, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	_, err := s.db.CreateVariantFull(ctx, dbgen.CreateVariantFullParams{
		ID:              variantID,
		WorkspaceID:     p.WorkspaceID,
		AssetVersionID:  p.VersionID,
		Type:            jobType,
		StorageKey:      storageKey,
		TransformParams: &paramsStr,
		Size:            &sz,
		Status:          variantStatusReady,
		Title:           p.Title,
		IsShared:        boolToInt64(p.IsShared),
	})
	if err == nil {
		s.publishVariantReady(ctx, p.WorkspaceID, p.AssetID, variantID)
		s.enqueueVariantThumb(ctx, p, variantID, storageKey, contentType)
	}
	return err
}

// finalizeRebuildVariant stores the transform output, creates the variant DB
// row (no title/is_shared), then publishes the ready event and enqueues the
// thumbnail job. Used by rebuild variant jobs.
func (s *JobServer) finalizeRebuildVariant(
	ctx context.Context,
	ver dbgen.AssetVersion,
	variantType, paramsJSON, paramsHash string,
	data []byte,
	contentType string,
) error {
	ext := transform.MimeToExt(contentType)
	storageKey := storage.VersionedVariantKey(ver.WorkspaceID, ver.AssetID, ver.VersionNum, variantType, paramsHash, ext)

	if err := s.storage.Put(storageKey, bytes.NewReader(data)); err != nil {
		return fmt.Errorf("store variant: %w", err)
	}

	sz := int64(len(data))
	vid := uuid.NewString()
	_, err := s.db.CreateVariant(ctx, dbgen.CreateVariantParams{
		ID:              vid,
		WorkspaceID:     ver.WorkspaceID,
		AssetVersionID:  ver.ID,
		Type:            variantType,
		StorageKey:      storageKey,
		TransformParams: &paramsJSON,
		Size:            &sz,
	})
	if err == nil {
		s.publishVariantReady(ctx, ver.WorkspaceID, ver.AssetID, vid)
		s.enqueueVariantThumbRaw(ctx, ver.WorkspaceID, ver.AssetID, vid, storageKey, contentType)
	}
	return err
}
