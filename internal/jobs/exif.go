package jobs

import (
	"context"
	"encoding/json"
	"fmt"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// ExtractExifPayload is the payload for the extract_exif job.
type ExtractExifPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	UserID      string `json:"user_id"` // required: field_definitions.created_by and asset_field_values.created_by are NOT NULL
}

// EnqueueExtractExifJob enqueues an extract_exif job for an image asset.
func EnqueueExtractExifJob(ctx context.Context, q queue.JobQueue, workspaceID, assetID, userID string) error {
	payload, _ := json.Marshal(ExtractExifPayload{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		UserID:      userID,
	})
	_, err := q.Enqueue(ctx, workspaceID, queue.JobTypeExtractExif, string(payload))
	return err
}

func (s *JobServer) jobExtractExif(ctx context.Context, job dbgen.Job) error {
	var p ExtractExifPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("exif job: parse payload: %w", err)
	}
	return s.exifSvc.ExtractForAsset(ctx, p.WorkspaceID, p.AssetID, p.UserID)
}
