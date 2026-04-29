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

	"github.com/google/uuid"
)

// Worker handles ingest_poll and ingest_fetch jobs.
type Worker struct {
	db       *dbgen.Queries
	sqlDB    *sql.DB
	storage  storage.Storage
	queue    queue.JobQueue
	cfg      *config.Config
	audit    *audit.EventWriter
	mailer   mail.Mailer
	injestor assetio.Injestor
}

// NewWorker creates a Worker.
func NewWorker(db *dbgen.Queries, sqlDB *sql.DB, stor storage.Storage, qu queue.JobQueue, cfg *config.Config, au *audit.EventWriter, mailer mail.Mailer, injestor assetio.Injestor) *Worker {
	return &Worker{db: db, sqlDB: sqlDB, storage: stor, queue: qu, cfg: cfg, audit: au, mailer: mailer, injestor: injestor}
}

// PollJobPayload is the JSON payload for a JobTypeIngestPoll job.
type PollJobPayload struct {
	SourceID    string `json:"source_id"`
	WorkspaceID string `json:"workspace_id"`
}

// FetchJobPayload is the JSON payload for a JobTypeIngestFetch job.
type FetchJobPayload struct {
	SourceID         string `json:"source_id"`
	WorkspaceID      string `json:"workspace_id"`
	LogEntryID       string `json:"log_entry_id"`
	RemoteID         string `json:"remote_id"`
	Filename         string `json:"filename"`
	TmpPath          string            `json:"tmp_path,omitempty"`
	OverrideFolderID string            `json:"override_folder_id,omitempty"`
	Meta             map[string]string `json:"meta,omitempty"`
}

// HandlePoll is the queue.HandlerFunc for JobTypeIngestPoll.
// It opens the source, calls Poll(), inserts log entries, and enqueues fetch jobs.
func (w *Worker) HandlePoll(ctx context.Context, job dbgen.Job) error {
	var p PollJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("ingest_poll: parse payload: %w", err)
	}

	src, err := w.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: p.SourceID, WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		return fmt.Errorf("ingest_poll: get source %s: %w", p.SourceID, err)
	}
	if src.Enabled == 0 {
		return nil
	}

	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return w.failSource(ctx, src, fmt.Errorf("ingest_poll: decrypt config: %w", err))
	}

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
		entry, err := w.db.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
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
			slog.Error("ingest_poll: insert log entry", "remote_id", item.RemoteID, "error", err)
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
			slog.Error("ingest_poll: enqueue fetch", "remote_id", item.RemoteID, "error", err)
		}
	}

	return w.markPolledSuccess(ctx, src.ID)
}

// HandleFetch is the queue.HandlerFunc for JobTypeIngestFetch.
// It downloads one item, validates MIME, stores it, and creates an asset row.
func (w *Worker) HandleFetch(ctx context.Context, job dbgen.Job) error {
	var p FetchJobPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("ingest_fetch: parse payload: %w", err)
	}

	entry, err := w.db.GetIngressLogEntry(ctx, p.LogEntryID)
	if err != nil {
		return fmt.Errorf("ingest_fetch: get log entry %s: %w", p.LogEntryID, err)
	}
	// Idempotency: skip if already processed
	if entry.Status != "pending" {
		return nil
	}

	src, err := w.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: p.SourceID, WorkspaceID: p.WorkspaceID,
	})
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: get source: %w", err))
	}

	configJSON, err := DecryptConfig(w.cfg.AppSecret, src.Config)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: decrypt config: %w", err))
	}

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: build source: %w", err))
	}

	// Evaluate rules
	rules, err := w.db.ListIngressRules(ctx, src.ID)
	if err != nil {
		slog.Warn("ingest_fetch: list rules (continuing without rules)", "source_id", src.ID, "error", err)
	}
	ruleResult := EvaluateRules(rules, ItemMeta{Filename: entry.Filename})
	if !ruleResult.Allow {
		skipped := "skipped"
		_ = w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
			Status: "skipped", ID: entry.ID, Error: &skipped,
		})
		return nil
	}

	// Determine destination: rules > email subaddress override > source defaults
	projectID := src.DestProjectID
	folderID := src.DestFolderID
	if p.OverrideFolderID != "" {
		folderID = &p.OverrideFolderID
	}
	if ruleResult.ProjectID != nil {
		projectID = ruleResult.ProjectID
	}
	if ruleResult.FolderID != nil {
		folderID = ruleResult.FolderID
	}

	// Fetch the item — either from a pre-written temp file (email_api push path)
	// or by calling source.Fetch() (pull sources).
	var rc io.ReadCloser
	if p.TmpPath != "" {
		f, err := os.Open(p.TmpPath)
		if err != nil {
			return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: open tmp file: %w", err))
		}
		defer os.Remove(p.TmpPath)
		rc = f
	} else {
		rc, err = source.Fetch(ctx, IngestItem{RemoteID: entry.RemoteID, Filename: entry.Filename, Meta: p.Meta})
		if err != nil {
			return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: fetch item: %w", err))
		}
	}
	defer rc.Close()

	// Sniff MIME type from first 512 bytes, then stream to temp file
	sniff := make([]byte, 512)
	n, _ := io.ReadFull(rc, sniff)
	sniff = sniff[:n]
	mimeType := http.DetectContentType(sniff)
	_ = mimeType // available for future MIME allowlist check

	tmp, err := os.CreateTemp("", "ingest-*")
	if err != nil {
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(sniff), rc)); err != nil {
		_ = tmp.Close()
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: write temp file: %w", err))
	}
	_ = tmp.Close()

	// Rename temp file to use original filename for CreateAsset
	namedTmp := filepath.Join(os.TempDir(), filepath.Base(entry.Filename))
	if err := os.Rename(tmpPath, namedTmp); err != nil {
		namedTmp = tmpPath // fall back to random name
	}
	defer os.Remove(namedTmp)

	asset, err := w.injestor.IngestFile(ctx, src.WorkspaceID, namedTmp, assetio.IngestFileOpts{
		ProjectID: projectID,
		FolderID:  folderID,
		UserID:    src.CreatedBy,
	})
	if err != nil {
		slog.Error("ingest_fetch: cannot create asset", "error", err)
		return w.failEntry(ctx, entry.ID, src, fmt.Errorf("ingest_fetch: create asset: %w", err))
	}

	assetID := asset.ID
	if err := w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status:  "imported",
		AssetID: &assetID,
		ID:      entry.ID,
	}); err != nil {
		slog.Error("ingest_fetch: update log entry", "entry_id", entry.ID, "error", err)
	}

	w.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: src.WorkspaceID,
		AssetID:     asset.ID,
		UserID:      nil,
		ActorType:   audit.ActorTypeSystem,
		EventType:   audit.EventAssetCreated,
		Payload:     audit.AssetCreatedPayload{V: 1, Filename: asset.OriginalFilename, Source: "ingress", SourceID: src.Label},
	})

	tag, err := w.db.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: asset.WorkspaceID,
		Name:        src.Label,
	})
	if err != nil {
		slog.Error("ingest_fetch: could not get or create tag", "error", err)
	}

	_ = w.db.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	})

	return nil
}

func (w *Worker) failSource(ctx context.Context, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.db.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: src.ID, LastError: &msg,
	})
	// Fetch updated error_count to decide which email to send.
	updated, dbErr := w.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > 5
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}

func (w *Worker) notifySourceFailure(ctx context.Context, src dbgen.IngressSource, errMsg string, disabled bool) {
	members, dbErr := w.db.ListMembers(ctx, src.WorkspaceID)
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
	return w.db.MarkIngressSourceSuccess(ctx, sourceID)
}

func (w *Worker) markPolledError(ctx context.Context, sourceID, msg string) error {
	return w.db.MarkIngressSourceError(ctx, dbgen.MarkIngressSourceErrorParams{
		ID: sourceID, LastError: &msg,
	})
}

func (w *Worker) failEntry(ctx context.Context, entryID string, src dbgen.IngressSource, err error) error {
	msg := err.Error()
	_ = w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "error",
		Error:  &msg,
		ID:     entryID,
	})
	_ = w.markPolledError(ctx, src.ID, msg)
	updated, dbErr := w.db.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: src.ID, WorkspaceID: src.WorkspaceID,
	})
	disabled := dbErr == nil && updated.ErrorCount > 5
	w.notifySourceFailure(ctx, src, msg, disabled)
	return err
}
