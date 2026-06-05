package jobs

import (
	"context"
	"encoding/json"
	"fmt"
)

type OCRTextTrackPayload struct {
	WorkspaceID    string `json:"workspace_id"`
	AssetID        string `json:"asset_id"`
	TrackID        string `json:"track_id"`
	AssetVersionID string `json:"asset_version_id"`
	StorageKey     string `json:"storage_key"`
	MimeType       string `json:"mime_type"`
	Lang           string `json:"lang"`
	OutputFormat   string `json:"output_format"`
}

func (s *JobServer) jobOCRTextTrack(ctx context.Context, rawPayload string) error {
	var p OCRTextTrackPayload
	if err := json.Unmarshal([]byte(rawPayload), &p); err != nil {
		return fmt.Errorf("jobOCRTextTrack: unmarshal: %w", err)
	}
	return s.textTrackSvc.RunOCR(
		ctx,
		p.WorkspaceID,
		p.AssetID,
		p.TrackID,
		p.AssetVersionID,
		p.StorageKey,
		p.MimeType,
		p.Lang,
		p.OutputFormat,
	)
}
