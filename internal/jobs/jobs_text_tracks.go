package jobs

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"go.opentelemetry.io/otel/attribute"
)

var runOCR = transform.RunOCR

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

func (s *JobServer) jobOCRTextTrack(ctx context.Context, rawPayload string) (err error) {
	var p OCRTextTrackPayload
	if err = json.Unmarshal([]byte(rawPayload), &p); err != nil {
		return fmt.Errorf("jobOCRTextTrack: unmarshal: %w", err)
	}
	ctx, span := telemetry.StartBackgroundSpan(ctx, "jobs.text_tracks.ocr",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.text_track_id", p.TrackID),
		attribute.String("damask.text_track.lang", p.Lang),
		attribute.String("damask.text_track.output_format", p.OutputFormat),
	)
	defer func() {
		telemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track OCR job failed",
				"workspace_id",
				p.WorkspaceID,
				"asset_id",
				p.AssetID,
				"track_id",
				p.TrackID,
				"error",
				err,
			)
		}
	}()

	_, readSpan := telemetry.StartSpan(ctx, "jobs.text_tracks.ocr.read_source",
		attribute.String("damask.storage_key", p.StorageKey),
	)
	rc, err := s.storage.Get(p.StorageKey)
	telemetry.EndSpan(readSpan, err)
	if err != nil {
		return fmt.Errorf("jobOCRTextTrack: read source: %w", err)
	}
	imageBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("jobOCRTextTrack: read bytes: %w", err)
	}
	span.SetAttributes(attribute.Int("damask.source_bytes", len(imageBytes)))

	ocrCtx, ocrSpan := telemetry.StartSpan(ctx, "jobs.text_tracks.ocr.run")
	result, err := runOCR(ocrCtx, imageBytes, transform.OCRParams{
		Lang:         p.Lang,
		OutputFormat: p.OutputFormat,
	})
	telemetry.EndSpan(ocrSpan, err)
	if err != nil {
		errMsg := err.Error()
		_ = s.db.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          p.TrackID,
			WorkspaceID: p.WorkspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("jobOCRTextTrack: OCR: %w", err)
	}

	var storageKey *string
	var contentType *string
	if p.OutputFormat == "hocr" {
		key := fmt.Sprintf("%s/%s/text-tracks/%s%s", p.WorkspaceID, p.AssetID, p.TrackID, result.Extension)
		_, storeSpan := telemetry.StartSpan(ctx, "jobs.text_tracks.ocr.store_companion",
			attribute.String("damask.storage_key", key),
		)
		if err = s.storage.Put(key, bytes.NewReader(result.FileContent)); err != nil {
			telemetry.EndSpan(storeSpan, err)
			return fmt.Errorf("jobOCRTextTrack: store companion file: %w", err)
		}
		telemetry.EndSpan(storeSpan, nil)
		storageKey = &key
		ct := result.ContentType
		contentType = &ct
	}

	plainText := readyTrackContent(result.PlainText)
	wordCount := len(strings.Fields(result.PlainText))
	metaBytes, _ := json.Marshal(map[string]any{
		"lang":          p.Lang,
		"model":         "tesseract",
		"output_format": p.OutputFormat,
		"word_count":    wordCount,
	})
	meta := string(metaBytes)

	writeCtx, writeSpan := telemetry.StartSpan(ctx, "jobs.text_tracks.ocr.mark_ready")
	if err = s.db.SetTextTrackReady(writeCtx, dbgen.SetTextTrackReadyParams{
		Content:     plainText,
		StorageKey:  storageKey,
		ContentType: contentType,
		Meta:        &meta,
		ID:          p.TrackID,
		WorkspaceID: p.WorkspaceID,
	}); err != nil {
		telemetry.EndSpan(writeSpan, err)
		return fmt.Errorf("jobOCRTextTrack: update track: %w", err)
	}
	telemetry.EndSpan(writeSpan, nil)

	ftsCtx, ftsSpan := telemetry.StartSpan(ctx, "jobs.text_tracks.ocr.index_fts")
	if err = s.db.InsertTextFTS(ftsCtx, dbgen.InsertTextFTSParams{
		TrackID:     p.TrackID,
		AssetID:     p.AssetID,
		WorkspaceID: p.WorkspaceID,
		Source:      "ocr",
		Lang:        p.Lang,
		Content:     plainText,
	}); err != nil {
		telemetry.EndSpan(ftsSpan, err)
		slog.WarnContext(ctx, "text track FTS insert failed", "track_id", p.TrackID, "error", err)
	} else {
		telemetry.EndSpan(ftsSpan, nil)
	}
	span.SetAttributes(
		attribute.Int("damask.text_track.word_count", wordCount),
		attribute.Bool("damask.text_track.has_file", storageKey != nil),
	)
	slog.DebugContext(
		ctx,
		"text track OCR completed",
		"workspace_id",
		p.WorkspaceID,
		"asset_id",
		p.AssetID,
		"track_id",
		p.TrackID,
		"word_count",
		wordCount,
		"output_format",
		p.OutputFormat,
	)

	return nil
}

func readyTrackContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return " "
	}
	return content
}
