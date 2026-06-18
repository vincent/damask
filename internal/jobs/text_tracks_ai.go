package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/ai"
	dbgen "damask/server/internal/db/gen"
)

// AIImageDescriptionPayload is the job payload for ai_image_description_text_track.
type AIImageDescriptionPayload struct {
	WorkspaceID string `json:"workspace_id"`
	AssetID     string `json:"asset_id"`
	TrackID     string `json:"track_id"`
	StorageKey  string `json:"storage_key"` // source image key (current version)
	MimeType    string `json:"mime_type"`   // image MIME (for base64 data URL)
	Model       string `json:"model"`
	Prompt      string `json:"prompt"` // final prompt (already includes lang instruction)
	Lang        string `json:"lang"`   // ISO 639-1, stored in track meta
}

func (s *JobServer) jobAIImageDescriptionTextTrack(ctx context.Context, job dbgen.Job) error {
	var p AIImageDescriptionPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("jobAIImageDescriptionTextTrack: unmarshal: %w", err)
	}

	if procErr := s.queries.SetTextTrackProcessing(ctx, dbgen.SetTextTrackProcessingParams{
		ID:          p.TrackID,
		WorkspaceID: p.WorkspaceID,
	}); procErr != nil {
		slog.ErrorContext(ctx, "ai image description: failed to mark track processing",
			"track_id", p.TrackID, "error", procErr)
	}

	apiKey, _, err := s.aiAPIKeyResolver(ctx, p.WorkspaceID, string(ai.ProviderOpenRouter))
	if err != nil || apiKey == "" {
		errMsg := "OpenRouter is not configured"
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobAIImageDescriptionTextTrack: %s", errMsg)
	}

	rc, err := s.storage.Get(p.StorageKey)
	if err != nil {
		errMsg := err.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobAIImageDescriptionTextTrack: read source: %w", err)
	}
	imageBytes, readErr := io.ReadAll(io.LimitReader(rc, ai.MaxDescribeImageBytes+1))
	_ = rc.Close()
	if readErr != nil {
		errMsg := readErr.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobAIImageDescriptionTextTrack: read bytes: %w", readErr)
	}
	if len(imageBytes) > ai.MaxDescribeImageBytes {
		errMsg := "source image exceeds maximum size for AI description"
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobAIImageDescriptionTextTrack: %s", errMsg)
	}

	client := ai.NewOpenRouterClient(apiKey)
	description, err := client.DescribeImage(ctx, p.Model, p.Prompt, imageBytes, p.MimeType)
	if err != nil {
		errMsg := err.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobAIImageDescriptionTextTrack: describe: %w", err)
	}

	wordCount := len(strings.Fields(description))
	metaBytes, _ := json.Marshal(map[string]any{
		"model":      p.Model,
		"lang":       p.Lang,
		"word_count": wordCount,
	})
	meta := string(metaBytes)

	if err = s.queries.SetTextTrackReady(ctx, dbgen.SetTextTrackReadyParams{
		ID:          p.TrackID,
		WorkspaceID: p.WorkspaceID,
		Content:     description,
		Meta:        &meta,
	}); err != nil {
		return fmt.Errorf("jobAIImageDescriptionTextTrack: mark ready: %w", err)
	}

	if ftsErr := s.queries.InsertTextFTS(ctx, dbgen.InsertTextFTSParams{
		TrackID:     p.TrackID,
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
		Source:      "ai_image_description",
		Lang:        p.Lang,
		Content:     description,
	}); ftsErr != nil {
		slog.WarnContext(ctx, "ai image description: FTS insert failed", "track_id", p.TrackID, "error", ftsErr)
	}

	slog.DebugContext(ctx, "ai image description completed",
		"workspace_id", p.WorkspaceID,
		"asset_id", p.AssetID,
		"track_id", p.TrackID,
		"model", p.Model,
		"word_count", wordCount,
	)
	return nil
}
