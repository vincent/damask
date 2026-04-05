package api

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"log"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

// versionThumbnailJobPayload is the payload for version-specific thumbnail generation.
type versionThumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// enqueueVersionThumbnailJob enqueues a version thumbnail job.
func enqueueVersionThumbnailJob(ctx context.Context, q *queue.Queue, workspaceID string, p versionThumbnailJobPayload) error {
	payload, _ := json.Marshal(p)
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeVersionThumbnail, string(payload))
	return err
}

// jobVersionThumbnail generates a thumbnail for a specific asset version.
// It writes the thumbnail to storage and updates asset_versions.thumbnail_key.
// If this is the current version, it also updates assets.thumbnail_key.
func (s *Server) jobVersionThumbnail(ctx context.Context, job dbgen.Job) error {
	var p versionThumbnailJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		return fmt.Errorf("get file: %w", err)
	}
	defer rc.Close()

	var thumbData []byte
	var thumbExt string

	switch {
	case isImageMime(p.MimeType):
		data, _, tErr := transform.Resize(rc, transform.ResizeParams{
			Width:   400,
			Height:  400,
			Fit:     "contain",
			Quality: 85,
			Format:  "jpeg",
		})
		if tErr != nil {
			return fmt.Errorf("resize: %w", tErr)
		}
		thumbData = data
		thumbExt = ".jpg"

	default:
		// For non-image types (video, PDF, audio) we skip version-specific
		// thumbnail generation here — the existing variant jobs handle those.
		log.Printf("version thumbnail: unsupported MIME %s for version %s", p.MimeType, p.VersionID)
		return nil
	}

	thumbKey := fmt.Sprintf("%s/%s/versions/%s/thumb%s", p.WorkspaceID, p.AssetID, p.VersionID, thumbExt)
	if err := s.storage.Put(thumbKey, bytes.NewReader(thumbData)); err != nil {
		return fmt.Errorf("store thumb: %w", err)
	}

	// Update the version row's thumbnail_key.
	if err := s.db.SetVersionThumbnail(ctx, dbgen.SetVersionThumbnailParams{
		ThumbnailKey: &thumbKey,
		ID:           p.VersionID,
	}); err != nil {
		return fmt.Errorf("set version thumbnail: %w", err)
	}

	// If this version is still current, sync the asset thumbnail too.
	ver, err := s.db.GetVersionByIDUnchecked(ctx, p.VersionID)
	if err == nil && ver.IsCurrent == 1 {
		if err := s.db.UpdateAssetThumbnail(ctx, dbgen.UpdateAssetThumbnailParams{
			ThumbnailKey: &thumbKey,
			ID:           p.AssetID,
		}); err == nil {
			s.hub.Publish(p.WorkspaceID, Event{
				Type:         "thumbnail_ready",
				AssetID:      p.AssetID,
				ThumbnailKey: thumbKey,
			})
		}
	}

	return nil
}

