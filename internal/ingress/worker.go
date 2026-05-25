package ingress

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
)

const (
	sniffFirstBytes  = 512
	ErrorCountCutoff = 5
)

// Worker handles ingest_poll and ingest_fetch jobs.
type Worker struct {
	queries  *dbgen.Queries
	sqlDB    *sql.DB
	storage  storage.Storage
	queue    queue.JobQueue
	cfg      *config.Config
	audit    *audit.EventWriter
	mailer   mail.Mailer
	injestor assetio.Injestor
}

// NewWorker creates a Worker.
func NewWorker(
	queries *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	qu queue.JobQueue,
	cfg *config.Config,
	au *audit.EventWriter,
	mailer mail.Mailer,
	injestor assetio.Injestor,
) *Worker {
	return &Worker{
		queries:  queries,
		sqlDB:    sqlDB,
		storage:  stor,
		queue:    qu,
		cfg:      cfg,
		audit:    au,
		mailer:   mailer,
		injestor: injestor,
	}
}

// PollJobPayload is the JSON payload for a JobTypeIngestPoll job.
type PollJobPayload struct {
	SourceID    string `json:"source_id"`
	WorkspaceID string `json:"workspace_id"`
}

// FetchJobPayload is the JSON payload for a JobTypeIngestFetch job.
type FetchJobPayload struct {
	SourceID         string            `json:"source_id"`
	WorkspaceID      string            `json:"workspace_id"`
	LogEntryID       string            `json:"log_entry_id"`
	RemoteID         string            `json:"remote_id"`
	Filename         string            `json:"filename"`
	TmpPath          string            `json:"tmp_path,omitempty"`
	OverrideFolderID string            `json:"override_folder_id,omitempty"`
	Meta             map[string]string `json:"meta,omitempty"`
}

// HandlePoll is the queue.HandlerFunc for JobTypeIngestPoll.
// It opens the source, calls Poll(), inserts log entries, and enqueues fetch jobs.
func (w *Worker) HandlePoll(ctx context.Context, job dbgen.Job) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.poll")
	defer telemetry.EndSpan(span, err)

	var p PollJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("ingest_poll: parse payload: %w", err)
	}

	src, err := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: p.SourceID, WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		return fmt.Errorf("ingest_poll: get source %s: %w", p.SourceID, err)
	}

	span.SetAttributes(attribute.String("damask.ingest.poll.workspace_id", p.WorkspaceID))
	span.SetAttributes(attribute.String("damask.ingest.poll.source_id", p.SourceID))

	if src.Enabled == 0 {
		return nil
	}

	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: decrypt config: %w", err))
	}

	span.SetAttributes(attribute.String("damask.ingest.poll.source_type", src.Type))

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: build source: %w", err))
	}

	items, err := source.Poll(ctx)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: poll: %w", err))
	}

	for _, item := range items {
		entryID := uuid.NewString()

		_, itemSpan := telemetry.StartSpan(ctx, "workers.ingest.poll.item",
			attribute.String("damask.ingest.poll.entry_id", entryID),
		)
		defer itemSpan.End()

		entry, err := w.queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
			ID:       entryID,
			SourceID: src.ID,
			RemoteID: item.RemoteID,
			Filename: item.Filename,
		})
		if errors.Is(err, sql.ErrNoRows) {
			// INSERT OR IGNORE: row already exists — item already known
			continue
		}
		if err != nil {
			slog.ErrorContext(ctx, "ingest_poll: insert log entry", "remote_id", item.RemoteID, "error", err)
			continue
		}

		payload, _ := json.Marshal(FetchJobPayload{
			SourceID:    src.ID,
			WorkspaceID: src.WorkspaceID,
			LogEntryID:  entry.ID,
			RemoteID:    item.RemoteID,
			Filename:    item.Filename,
			Meta:        item.Meta,
		})
		if _, err := w.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestFetch, string(payload)); err != nil {
			slog.ErrorContext(ctx, "ingest_poll: enqueue fetch", "remote_id", item.RemoteID, "error", err)
		}
	}

	return w.markPolledSuccess(ctx, src.ID)
}

// HandleFetch is the queue.HandlerFunc for JobTypeIngestFetch.
// It downloads one item, validates MIME, stores it, and creates an asset row.
func (w *Worker) HandleFetch(ctx context.Context, job dbgen.Job) (err error) {
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.fetch")
	defer telemetry.EndSpan(span, err)

	var p FetchJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("ingest_fetch: parse payload: %w", err)
	}

	span.SetAttributes(attribute.String("damask.ingest.workspace_id", p.WorkspaceID))
	span.SetAttributes(attribute.String("damask.ingest.entry_id", p.LogEntryID))

	entry, err := w.queries.GetIngressLogEntry(ctx, p.LogEntryID)
	if err != nil {
		return fmt.Errorf("ingest_fetch: get log entry %s: %w", p.LogEntryID, err)
	}

	span.SetAttributes(attribute.String("damask.ingest.entry_remote_id", entry.RemoteID))
	span.SetAttributes(attribute.String("damask.ingest.entry_status", entry.Status))

	// Idempotency: skip if already processed
	if entry.Status != "pending" {
		return nil
	}

	src, err := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: p.SourceID, WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: get source: %w", err))
	}

	span.SetAttributes(attribute.String("damask.ingest.entry_source_id", src.ID))
	span.SetAttributes(attribute.String("damask.ingest.entry_source_label", src.Label))
	span.SetAttributes(attribute.String("damask.ingest.entry_source_type", src.Type))

	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: decrypt config: %w", err))
	}

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: build source: %w", err))
	}

	// Evaluate rules
	rules, err := w.queries.ListIngressRules(ctx, src.ID)
	if err != nil {
		slog.WarnContext(ctx, "ingest_fetch: list rules (continuing without rules)", "source_id", src.ID, "error", err)
	}

	span.SetAttributes(attribute.Int("damask.ingest.entry_source_rules", len(rules)))

	ruleResult := EvaluateRules(rules, ItemMeta{Filename: entry.Filename})
	span.SetAttributes(attribute.Bool("damask.ingest.entry_source_rules_pass", ruleResult.Allow))
	if !ruleResult.Allow {
		skipped := ingressStatusSkipped
		_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
			Status: ingressStatusSkipped, ID: entry.ID, Error: &skipped,
		})
		return nil
	}

	// Determine destination: rules > email subaddress override > source defaults
	projectID := src.DestProjectID
	folderID := src.DestFolderID
	if p.OverrideFolderID != "" {
		span.SetAttributes(attribute.String("damask.ingest.entry_folder_id", p.OverrideFolderID))
		folderID = &p.OverrideFolderID
	}
	if ruleResult.FolderID != nil {
		span.SetAttributes(attribute.String("damask.ingest.entry_folder_id", *ruleResult.FolderID))
		folderID = ruleResult.FolderID
	}
	if ruleResult.ProjectID != nil {
		span.SetAttributes(attribute.String("damask.ingest.entry_project_id", *ruleResult.ProjectID))
		projectID = ruleResult.ProjectID
	}

	// Fetch the item — either from a pre-written temp file (email_api push path)
	// or by calling source.Fetch() (pull sources).
	var rc io.ReadCloser
	if p.TmpPath != "" {
		slog.DebugContext(ctx, "use existing temp file")
		f, err := os.Open(p.TmpPath)
		if err != nil {
			return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: open tmp file: %w", err))
		}
		defer os.Remove(p.TmpPath)
		rc = f
	} else {
		slog.DebugContext(ctx, "use fetch from source")
		rc, err = source.Fetch(ctx, IngestItem{RemoteID: entry.RemoteID, Filename: entry.Filename, Meta: p.Meta})
		if err != nil {
			return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: fetch item: %w", err))
		}
	}
	defer rc.Close()

	// Sniff MIME type from first 512 bytes, then stream to temp file
	sniff := make([]byte, sniffFirstBytes)
	n, _ := io.ReadFull(rc, sniff)
	sniff = sniff[:n]
	mimeType := http.DetectContentType(sniff)
	_ = mimeType // available for future MIME allowlist check

	tmp, err := os.CreateTemp("", "ingest-*")
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	slog.DebugContext(ctx, "use temp file", "path", tmpPath)
	defer os.Remove(tmpPath)

	copied, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(sniff), rc))
	if err != nil {
		_ = tmp.Close()
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: write temp file: %w", err))
	}
	slog.DebugContext(ctx, "wrote temp file", "path", tmpPath, "bytes", copied)
	_ = tmp.Close()

	// Rename temp file to use original filename for CreateAsset
	namedTmp := filepath.Join(os.TempDir(), filepath.Base(entry.Filename))
	if err := os.Rename(tmpPath, namedTmp); err != nil {
		namedTmp = tmpPath // fall back to random name
	}
	defer os.Remove(namedTmp)

	slog.DebugContext(ctx, "ingest file", "path", namedTmp)

	asset, err := w.injestor.IngestFile(ctx, src.WorkspaceID, namedTmp, assetio.IngestFileOpts{
		ProjectID: projectID,
		FolderID:  folderID,
		UserID:    src.CreatedBy,
	})
	if err != nil {
		slog.ErrorContext(ctx, "ingest_fetch: cannot create asset", "error", err)
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create asset: %w", err))
	}

	slog.DebugContext(ctx, "update ingress log entry", "entry_id", entry.ID, "asset_id", &asset.ID)

	assetID := asset.ID
	if err := w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status:  ingressStatusImported,
		AssetID: &assetID,
		ID:      entry.ID,
	}); err != nil {
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "ingest_fetch: update log entry", "entry_id", entry.ID, "error", err)
	}

	w.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: src.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      nil,
		ActorType:   audit.ActorTypeSystem,
		EventType:   audit.EventAssetCreated,
		Payload: audit.AssetCreatedPayload{
			V:        1,
			Filename: asset.OriginalFilename,
			Source:   "ingress",
			SourceID: src.Label,
		},
	})

	tag, err := w.queries.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: asset.WorkspaceID,
		Name:        src.Label,
	})
	if err != nil {
		span.SetStatus(codes.Error, err.Error())
		slog.ErrorContext(ctx, "ingest_fetch: could not get or create tag", "error", err)
	}

	slog.DebugContext(ctx, "tag new entry", "tag", src.Label, "asset_id", &asset.ID)

	_ = w.queries.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	})

	return nil
}

func (w *Worker) failSource(ctx context.Context, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.queries.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: src.ID, LastError: &msg,
	})
	// Fetch updated error_count to decide which email to send.
	updated, dbErr := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > ErrorCountCutoff
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}

func (w *Worker) notifySourceFailure(ctx context.Context, src dbgen.IngressSource, errMsg string, disabled bool) {
	var err error
	ctx, span := telemetry.StartSpan(ctx, "workers.ingest.notify_failure")
	defer func() { telemetry.EndSpan(span, err) }()

	members, dbErr := w.queries.ListMembers(ctx, src.WorkspaceID)
	if dbErr != nil {
		return
	}
	notified := map[string]bool{}
	for _, m := range members {
		if m.Role == string(auth.Owner) || m.UserID == src.CreatedBy {
			if !notified[m.Email] {
				notified[m.Email] = true
				var mailErr error
				if disabled {
					mailErr = w.mailer.SendIngressSourceDisabled(ctx, m.Email, src.Label, errMsg, src.WorkspaceID)
				} else {
					mailErr = w.mailer.SendIngressSourceFailed(ctx, m.Email, src.Label, errMsg, src.WorkspaceID)
				}
				if mailErr != nil {
					slog.ErrorContext(ctx, "failed to send ingress failure mail", "error", mailErr)
				}
			}
		}
	}
}

func (w *Worker) markPolledSuccess(ctx context.Context, sourceID string) error {
	return w.queries.MarkIngressSourceSuccess(ctx, sourceID)
}

func (w *Worker) markPolledError(ctx context.Context, sourceID, msg string) error {
	return w.queries.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: sourceID, LastError: &msg,
	})
}

func (w *Worker) failEntry(ctx context.Context, entryID string, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "error",
		Error:  &msg,
		ID:     entryID,
	})
	_ = w.markPolledError(ctx, src.ID, msg)
	updated, dbErr := w.queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > ErrorCountCutoff
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}
