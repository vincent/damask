package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"mime"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

// VersionThumbnailJobPayload is the payload for version-specific thumbnail generation.
type VersionThumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// EnqueueVersionThumbnailJob enqueues a version thumbnail job.
func EnqueueVersionThumbnailJob(ctx context.Context, q queue.JobQueue, workspaceID string, p VersionThumbnailJobPayload) error {
	payload, _ := json.Marshal(p)
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeVersionThumbnail, string(payload))
	return err
}

// EnqueueRebuildVariantsJob enqueues a rebuild_variants job when a new version is uploaded.
// sourceVersionID is the version that was current before the upload — its variant params are copied.
// If sourceVersionID is empty (first upload), this is a no-op.
func EnqueueRebuildVariantsJob(ctx context.Context, q queue.JobQueue, workspaceID, assetID, newVersionID, sourceVersionID string) error {
	if sourceVersionID == "" {
		return nil
	}
	payload, _ := json.Marshal(RebuildVariantsPayload{
		AssetID:         assetID,
		NewVersionID:    newVersionID,
		SourceVersionID: sourceVersionID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeRebuildVariants, string(payload))
	return err
}

// jobVersionThumbnail generates a thumbnail for a specific asset version.
// It writes the thumbnail to storage and updates asset_versions.thumbnail_key.
// If this is the current version, it also updates assets.thumbnail_key.
func (s *JobServer) jobVersionThumbnail(ctx context.Context, job dbgen.Job) error {
	var p VersionThumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	thumbData, thumbExt, err := transform.GenerateThumbnailData(ctx, s.storage, p.MimeType, p.StorageKey)
	if err != nil {
		return fmt.Errorf("generate thumbnail: %w", err)
	}
	if thumbData == nil {
		return nil // unsupported or skipped (e.g. no ffmpeg)
	}

	thumbKey := fmt.Sprintf("%s/%s/versions/%s/thumb%s", p.WorkspaceID, p.AssetID, p.VersionID, thumbExt)
	if err := s.storage.Put(thumbKey, bytes.NewReader(thumbData)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	thumbContentType := mime.TypeByExtension(thumbExt)
	if thumbContentType == "" {
		thumbContentType = "image/jpeg"
	}

	if err := s.db.SetVersionThumbnail(ctx, dbgen.SetVersionThumbnailParams{
		ThumbnailKey:         &thumbKey,
		ThumbnailContentType: thumbContentType,
		ID:                   p.VersionID,
	}); err != nil {
		return fmt.Errorf("set version thumbnail: %w", err)
	}

	// If this version is still current, sync the asset thumbnail too.
	ver, err := s.db.GetVersionByIDUnchecked(ctx, p.VersionID)
	if err == nil && ver.IsCurrent == 1 {
		if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey:         &thumbKey,
			ThumbnailContentType: thumbContentType,
			ID:                   p.AssetID,
		}); err == nil {
			s.hub.Publish(p.WorkspaceID, events.Event{
				Type:         "thumbnail_ready",
				AssetID:      p.AssetID,
				ThumbnailKey: thumbKey,
			})
		}
	}

	return nil
}
