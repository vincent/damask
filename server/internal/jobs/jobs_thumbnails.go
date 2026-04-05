package jobs

import (
	"bytes"
	"context"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/transform"
	"encoding/json"
	"fmt"
)

// assetThumbnailPayload mirrors services.thumbnailJobPayload (JSON-compatible).
type assetThumbnailPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// ---- Thumbnail job — asset upload ----

func (s *JobServer) jobAssetThumbnail(ctx context.Context, job dbgen.Job) error {
	var p assetThumbnailPayload
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

	thumbKey := fmt.Sprintf("%s/%s/thumb%s", p.WorkspaceID, p.AssetID, thumbExt)
	if err := s.storage.Put(thumbKey, bytes.NewReader(thumbData)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
		ThumbnailKey: &thumbKey,
		ID:           p.AssetID,
	}); err != nil {
		return err
	}
	s.hub.Publish(p.WorkspaceID, events.Event{
		Type:         "thumbnail_ready",
		AssetID:      p.AssetID,
		ThumbnailKey: thumbKey,
	})
	return nil
}
