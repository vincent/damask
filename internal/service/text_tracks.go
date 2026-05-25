package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/jobs"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	apptelemetry "damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
)

const textTrackSourceManual = "manual"

var ErrUnsupportedTextTrackSource = errors.New("unsupported text track source")

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
	case "ocr":
		if stringParam(p.Params, "storage_key", "") == "" {
			return TextTrackDTO{}, fmt.Errorf("missing OCR storage key: %w", apperr.ErrInvalidInput)
		}
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

	if p.Source == "ocr" {
		payload, err := json.Marshal(jobs.OCRTextTrackPayload{
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
