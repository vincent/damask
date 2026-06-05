package service

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const textTrackSourceManual = "manual"
const textTrackSourceOCR = "ocr"
const textTrackSourceExtractPDF = "extract_pdf"
const textTrackSourceExtractPlain = "extract_plain"
const textTrackSourceExtractDocument = "extract_document"

var ErrUnsupportedTextTrackSource = errors.New("unsupported text track source")

func extractJobType(source string) (string, error) {
	switch source {
	case textTrackSourceExtractPDF:
		return queue.JobTypeExtractPDFTextTrack, nil
	case textTrackSourceExtractPlain:
		return queue.JobTypeExtractPlainTextTrack, nil
	case textTrackSourceExtractDocument:
		return queue.JobTypeExtractDocumentTextTrack, nil
	default:
		return "", ErrUnsupportedTextTrackSource
	}
}

type textTrackService struct {
	queries *dbgen.Queries
	queue   queue.JobQueue
	storage storage.Storage
}

func NewTextTrackService(queries *dbgen.Queries, q queue.JobQueue, stor storage.Storage) TextTrackService {
	return &textTrackService{queries: queries, queue: q, storage: stor}
}

func (s *textTrackService) List(ctx context.Context, workspaceID, assetID string) (out []TextTrackDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.text_tracks.list",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
	)
	defer func() {
		if out != nil {
			span.SetAttributes(attribute.Int("damask.text_tracks.result_count", len(out)))
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track list failed",
				"workspace_id",
				workspaceID,
				"asset_id",
				assetID,
				"error",
				err,
			)
		}
	}()

	rows, err := s.queries.ListTextTracksByAsset(ctx, dbgen.ListTextTracksByAssetParams{
		AssetID:     assetID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out = make([]TextTrackDTO, len(rows))
	for i, row := range rows {
		out[i] = toTextTrackDTO(row)
	}
	return out, nil
}

func (s *textTrackService) Get(ctx context.Context, workspaceID, trackID string) (dto TextTrackDTO, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.text_tracks.get",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		if dto.ID != "" {
			span.SetAttributes(
				attribute.String("damask.asset_id", dto.AssetID),
				attribute.String("damask.text_track.source", dto.Source),
				attribute.String("damask.text_track.status", dto.Status),
			)
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track get failed",
				"workspace_id",
				workspaceID,
				"track_id",
				trackID,
				"error",
				err,
			)
		}
	}()

	row, err := s.queries.GetTextTrack(ctx, dbgen.GetTextTrackParams{
		ID:          trackID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return TextTrackDTO{}, apperr.ErrNotFound
		}
		return TextTrackDTO{}, err
	}
	dto = toTextTrackDTO(row)
	return dto, nil
}

func (s *textTrackService) Create(ctx context.Context, p CreateTextTrackParams) (dto TextTrackDTO, err error) {
	p.Source = strings.TrimSpace(p.Source)
	ctx, span := apptelemetry.StartSpan(ctx, "service.text_tracks.create",
		attribute.String("damask.workspace_id", p.WorkspaceID),
		attribute.String("damask.asset_id", p.AssetID),
		attribute.String("damask.text_track.source", p.Source),
	)
	defer func() {
		if dto.ID != "" {
			span.SetAttributes(
				attribute.String("damask.text_track_id", dto.ID),
				attribute.String("damask.text_track.status", dto.Status),
			)
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track create failed",
				"workspace_id",
				p.WorkspaceID,
				"asset_id",
				p.AssetID,
				"source",
				p.Source,
				"error",
				err,
			)
		}
	}()

	if p.Source == "" {
		return TextTrackDTO{}, fmt.Errorf("source is required: %w", apperr.ErrInvalidInput)
	}

	status := WorkflowRunStatusPending
	content := ""
	switch p.Source {
	case textTrackSourceManual:
		status = "ready"
		content = readyTextContent(p.InitialContent)
	case textTrackSourceOCR:
		if stringParam(p.Params, "storage_key", "") == "" {
			return TextTrackDTO{}, fmt.Errorf("missing OCR storage key: %w", apperr.ErrInvalidInput)
		}
	case textTrackSourceExtractPDF, textTrackSourceExtractPlain, textTrackSourceExtractDocument:
		if stringParam(p.Params, "storage_key", "") == "" {
			return TextTrackDTO{}, fmt.Errorf("missing storage key: %w", apperr.ErrInvalidInput)
		}
		return s.enqueueExtract(ctx, span, p)
	default:
		return TextTrackDTO{}, ErrUnsupportedTextTrackSource
	}

	var meta *string
	if len(p.Params) > 0 {
		b, err := json.Marshal(p.Params)
		if err != nil {
			return TextTrackDTO{}, fmt.Errorf("marshal params: %w", err)
		}
		s := string(b)
		meta = &s
	}
	createdBy := &p.CreatedBy

	row, err := s.queries.CreateTextTrack(ctx, dbgen.CreateTextTrackParams{
		ID:             uuid.NewString(),
		WorkspaceID:    p.WorkspaceID,
		AssetID:        p.AssetID,
		AssetVersionID: p.AssetVersionID,
		Source:         p.Source,
		Lang:           p.Lang,
		Content:        content,
		Meta:           meta,
		Status:         status,
		CreatedBy:      createdBy,
	})
	if err != nil {
		return TextTrackDTO{}, err
	}

	dto = toTextTrackDTO(row)
	if p.Source == textTrackSourceManual {
		if err := s.queries.InsertTextFTS(ctx, dbgen.InsertTextFTSParams{
			TrackID:     row.ID,
			AssetID:     row.AssetID,
			WorkspaceID: row.WorkspaceID,
			Source:      row.Source,
			Lang: func() string {
				if row.Lang != nil {
					return *row.Lang
				}
				return ""
			}(),
			Content: content,
		}); err != nil {
			return TextTrackDTO{}, err
		}
		return dto, nil
	}

	if p.Source == textTrackSourceOCR {
		payload, err := json.Marshal(struct {
			WorkspaceID    string `json:"workspace_id"`
			AssetID        string `json:"asset_id"`
			TrackID        string `json:"track_id"`
			AssetVersionID string `json:"asset_version_id"`
			StorageKey     string `json:"storage_key"`
			MimeType       string `json:"mime_type"`
			Lang           string `json:"lang"`
			OutputFormat   string `json:"output_format"`
		}{
			WorkspaceID:    p.WorkspaceID,
			AssetID:        p.AssetID,
			TrackID:        row.ID,
			AssetVersionID: stringValue(p.AssetVersionID),
			StorageKey:     stringParam(p.Params, "storage_key", ""),
			MimeType:       stringParam(p.Params, "mime_type", ""),
			Lang:           stringValue(p.Lang, "eng"),
			OutputFormat:   stringParam(p.Params, "output_format", "txt"),
		})
		if err != nil {
			return TextTrackDTO{}, fmt.Errorf("marshal OCR payload: %w", err)
		}
		job, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypeOCRTextTrack, string(payload))
		if err != nil {
			return TextTrackDTO{}, err
		}
		span.SetAttributes(attribute.String("damask.job_id", job.ID))
		slog.DebugContext(
			ctx,
			"text track OCR enqueued",
			"workspace_id",
			p.WorkspaceID,
			"asset_id",
			p.AssetID,
			"track_id",
			row.ID,
			"job_id",
			job.ID,
		)
		return dto, nil
	}

	return TextTrackDTO{}, ErrUnsupportedTextTrackSource
}

func (s *textTrackService) enqueueExtract(ctx context.Context, span interface{ SetAttributes(...attribute.KeyValue) }, p CreateTextTrackParams) (TextTrackDTO, error) {
	jobType, err := extractJobType(p.Source)
	if err != nil {
		return TextTrackDTO{}, err
	}
	payload, err := json.Marshal(struct {
		WorkspaceID string `json:"workspace_id"`
		AssetID     string `json:"asset_id"`
		StorageKey  string `json:"storage_key"`
		MimeType    string `json:"mime_type,omitempty"`
	}{
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		StorageKey:  stringParam(p.Params, "storage_key", ""),
		MimeType:    stringParam(p.Params, "mime_type", ""),
	})
	if err != nil {
		return TextTrackDTO{}, fmt.Errorf("marshal extract payload: %w", err)
	}
	job, err := s.queue.Enqueue(ctx, p.WorkspaceID, jobType, string(payload))
	if err != nil {
		return TextTrackDTO{}, err
	}
	span.SetAttributes(attribute.String("damask.job_id", job.ID))
	slog.DebugContext(ctx, "text track extract enqueued",
		"workspace_id", p.WorkspaceID,
		"asset_id", p.AssetID,
		"source", p.Source,
		"job_id", job.ID,
	)
	return TextTrackDTO{
		WorkspaceID: p.WorkspaceID,
		AssetID:     p.AssetID,
		Source:      p.Source,
		Status:      WorkflowRunStatusPending,
	}, nil
}

func (s *textTrackService) Delete(ctx context.Context, workspaceID, trackID string) (err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.text_tracks.delete",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track delete failed",
				"workspace_id",
				workspaceID,
				"track_id",
				trackID,
				"error",
				err,
			)
		}
	}()

	track, err := s.Get(ctx, workspaceID, trackID)
	if err != nil {
		return err
	}
	span.SetAttributes(
		attribute.String("damask.asset_id", track.AssetID),
		attribute.String("damask.text_track.source", track.Source),
		attribute.Bool("damask.text_track.has_file", track.StorageKey != nil),
	)
	if track.StorageKey != nil && s.storage != nil {
		if err := s.storage.Delete(*track.StorageKey); err != nil {
			slog.WarnContext(
				ctx,
				"text track storage delete failed",
				"track_id",
				trackID,
				"storage_key",
				*track.StorageKey,
				"error",
				err,
			)
		}
	}
	if err := s.queries.DeleteTextFTS(ctx, trackID); err != nil {
		return err
	}
	if err := s.queries.DeleteTextTrack(ctx, dbgen.DeleteTextTrackParams{
		ID:          trackID,
		WorkspaceID: workspaceID,
	}); err != nil {
		return err
	}
	return nil
}

func toTextTrackDTO(row dbgen.AssetTextTrack) TextTrackDTO {
	meta := map[string]any{}
	if row.Meta != nil && *row.Meta != "" {
		_ = json.Unmarshal([]byte(*row.Meta), &meta)
	}
	return TextTrackDTO{
		ID:             row.ID,
		WorkspaceID:    row.WorkspaceID,
		AssetID:        row.AssetID,
		AssetVersionID: row.AssetVersionID,
		Source:         row.Source,
		Lang:           row.Lang,
		Content:        row.Content,
		StorageKey:     row.StorageKey,
		ContentType:    row.ContentType,
		Meta:           meta,
		Status:         row.Status,
		Error:          row.Error,
		CreatedBy:      row.CreatedBy,
		CreatedAt:      row.CreatedAt,
		UpdatedAt:      row.UpdatedAt,
	}
}

func readyTextContent(content string) string {
	content = strings.TrimSpace(content)
	if content == "" {
		return " "
	}
	return content
}

func stringValue(value *string, fallback ...string) string {
	if value != nil && *value != "" {
		return *value
	}
	if len(fallback) > 0 {
		return fallback[0]
	}
	return ""
}

func stringParam(params map[string]any, key, fallback string) string {
	if params != nil {
		if value, ok := params[key].(string); ok && strings.TrimSpace(value) != "" {
			return value
		}
	}
	return fallback
}

func (s *textTrackService) RunExtractPDF(ctx context.Context, workspaceID, assetID, trackID, storageKey string) (err error) {
	ctx, span := apptelemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_pdf",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track PDF extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractPDF: read source: %w", err)
	}
	pdfBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractPDF: read bytes: %w", err)
	}

	text, err := transform.ExtractPDFText(ctx, pdfBytes)
	if err != nil {
		errMsg := err.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          trackID,
			WorkspaceID: workspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("RunExtractPDF: extract: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, "pdf", text)
}

func (s *textTrackService) RunExtractPlain(ctx context.Context, workspaceID, assetID, trackID, storageKey string) (err error) {
	ctx, span := apptelemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_plain",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track plain extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractPlain: read source: %w", err)
	}
	textBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractPlain: read bytes: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, "plain", string(textBytes))
}

func (s *textTrackService) RunExtractDocument(ctx context.Context, workspaceID, assetID, trackID, storageKey, mimeType string) (err error) {
	ctx, span := apptelemetry.StartBackgroundSpan(ctx, "service.text_tracks.extract_document",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.text_track_id", trackID),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "text track document extract job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return fmt.Errorf("RunExtractDocument: read source: %w", err)
	}
	docBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunExtractDocument: read bytes: %w", err)
	}

	text, err := transform.ExtractDocumentText(ctx, docBytes, mimeType)
	if err != nil {
		errMsg := err.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          trackID,
			WorkspaceID: workspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("RunExtractDocument: extract: %w", err)
	}

	return s.writeExtractedText(ctx, workspaceID, assetID, trackID, "document", text)
}

func (s *textTrackService) writeExtractedText(ctx context.Context, workspaceID, assetID, trackID, source, text string) error {
	content := readyTextContent(text)
	wordCount := len(strings.Fields(text))
	metaBytes, _ := json.Marshal(map[string]any{"word_count": wordCount})
	meta := string(metaBytes)

	if err := s.queries.SetTextTrackReady(ctx, dbgen.SetTextTrackReadyParams{
		Content:     content,
		StorageKey:  nil,
		ContentType: nil,
		Meta:        &meta,
		ID:          trackID,
		WorkspaceID: workspaceID,
	}); err != nil {
		return fmt.Errorf("writeExtractedText: mark ready: %w", err)
	}

	if err := s.queries.InsertTextFTS(ctx, dbgen.InsertTextFTSParams{
		TrackID:     trackID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Source:      source,
		Lang:        "",
		Content:     content,
	}); err != nil {
		slog.WarnContext(ctx, "text track FTS insert failed", "track_id", trackID, "error", err)
	}
	return nil
}

func (s *textTrackService) RunOCR(ctx context.Context, workspaceID, assetID, trackID, _, storageKey, mimeType, lang, outputFormat string) (err error) {
	ctx, span := apptelemetry.StartBackgroundSpan(ctx, "service.text_tracks.ocr",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.asset_id", assetID),
		attribute.String("damask.asset_mime_type", mimeType),
		attribute.String("damask.text_track_id", trackID),
		attribute.String("damask.text_track.lang", lang),
		attribute.String("damask.text_track.output_format", outputFormat),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(
				ctx,
				"text track OCR job failed",
				"workspace_id", workspaceID,
				"asset_id", assetID,
				"track_id", trackID,
				"error", err,
			)
		}
	}()

	_, readSpan := apptelemetry.StartSpan(ctx, "service.text_tracks.ocr.read_source",
		attribute.String("damask.storage_key", storageKey),
	)
	rc, err := s.storage.Get(storageKey)
	apptelemetry.EndSpan(readSpan, err)
	if err != nil {
		return fmt.Errorf("RunOCR: read source: %w", err)
	}
	imageBytes, err := io.ReadAll(rc)
	_ = rc.Close()
	if err != nil {
		return fmt.Errorf("RunOCR: read bytes: %w", err)
	}
	span.SetAttributes(attribute.Int("damask.source_bytes", len(imageBytes)))

	ocrCtx, ocrSpan := apptelemetry.StartSpan(ctx, "service.text_tracks.ocr.run")
	result, err := transform.RunOCR(ocrCtx, imageBytes, transform.OCRParams{
		Lang:         lang,
		OutputFormat: outputFormat,
	})
	apptelemetry.EndSpan(ocrSpan, err)
	if err != nil {
		errMsg := err.Error()
		_ = s.queries.SetTextTrackFailed(ctx, dbgen.SetTextTrackFailedParams{
			ID:          trackID,
			WorkspaceID: workspaceID,
			Error:       &errMsg,
		})
		return fmt.Errorf("RunOCR: OCR: %w", err)
	}

	var resultStorageKey *string
	var contentType *string
	if outputFormat == "hocr" {
		key := fmt.Sprintf("%s/%s/text-tracks/%s%s", workspaceID, assetID, trackID, result.Extension)
		_, storeSpan := apptelemetry.StartSpan(ctx, "service.text_tracks.ocr.store_companion",
			attribute.String("damask.storage_key", key),
		)
		if err = s.storage.Put(key, bytes.NewReader(result.FileContent)); err != nil {
			apptelemetry.EndSpan(storeSpan, err)
			return fmt.Errorf("RunOCR: store companion file: %w", err)
		}
		apptelemetry.EndSpan(storeSpan, nil)
		resultStorageKey = &key
		ct := result.ContentType
		contentType = &ct
	}

	plainText := readyTextContent(result.PlainText)
	wordCount := len(strings.Fields(result.PlainText))
	metaBytes, _ := json.Marshal(map[string]any{
		"lang":          lang,
		"model":         "tesseract",
		"output_format": outputFormat,
		"word_count":    wordCount,
	})
	meta := string(metaBytes)

	writeCtx, writeSpan := apptelemetry.StartSpan(ctx, "service.text_tracks.ocr.mark_ready")
	if err = s.queries.SetTextTrackReady(writeCtx, dbgen.SetTextTrackReadyParams{
		Content:     plainText,
		StorageKey:  resultStorageKey,
		ContentType: contentType,
		Meta:        &meta,
		ID:          trackID,
		WorkspaceID: workspaceID,
	}); err != nil {
		apptelemetry.EndSpan(writeSpan, err)
		return fmt.Errorf("RunOCR: update track: %w", err)
	}
	apptelemetry.EndSpan(writeSpan, nil)

	ftsCtx, ftsSpan := apptelemetry.StartSpan(ctx, "service.text_tracks.ocr.index_fts")
	if err = s.queries.InsertTextFTS(ftsCtx, dbgen.InsertTextFTSParams{
		TrackID:     trackID,
		AssetID:     assetID,
		WorkspaceID: workspaceID,
		Source:      textTrackSourceOCR,
		Lang:        lang,
		Content:     plainText,
	}); err != nil {
		apptelemetry.EndSpan(ftsSpan, err)
		slog.WarnContext(ctx, "text track FTS insert failed", "track_id", trackID, "error", err)
	} else {
		apptelemetry.EndSpan(ftsSpan, nil)
	}
	span.SetAttributes(
		attribute.Int("damask.text_track.word_count", wordCount),
		attribute.Bool("damask.text_track.has_file", resultStorageKey != nil),
	)
	slog.DebugContext(
		ctx,
		"text track OCR completed",
		"workspace_id", workspaceID,
		"asset_id", assetID,
		"track_id", trackID,
		"word_count", wordCount,
		"output_format", outputFormat,
	)

	return nil
}
