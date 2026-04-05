package ingress

import (
	"bytes"
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log"
	"net/http"
	"os"
	"path/filepath"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/services"
	"damask/server/internal/storage"

	"github.com/google/uuid"
)

// Worker handles ingest_poll and ingest_fetch jobs.
type Worker struct {
	db        *dbgen.Queries
	storage   storage.Storage
	queue     *queue.Queue
	appSecret string
}

// NewWorker creates a Worker.
func NewWorker(db *dbgen.Queries, stor storage.Storage, qu *queue.Queue, appSecret string) *Worker {
	return &Worker{db: db, storage: stor, queue: qu, appSecret: appSecret}
}

// PollJobPayload is the JSON payload for a JobTypeIngestPoll job.
type PollJobPayload struct {
	SourceID    string `json:"source_id"`
	WorkspaceID string `json:"workspace_id"`
}

// FetchJobPayload is the JSON payload for a JobTypeIngestFetch job.
type FetchJobPayload struct {
	SourceID    string `json:"source_id"`
	WorkspaceID string `json:"workspace_id"`
	LogEntryID  string `json:"log_entry_id"`
	RemoteID    string `json:"remote_id"`
	Filename    string `json:"filename"`
	TmpPath     string `json:"tmp_path,omitempty"`
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

	configJSON, err := DecryptConfig(w.appSecret, src.Config)
	if err != nil {
		return w.failSource(ctx, src.ID, fmt.Errorf("ingest_poll: decrypt config: %w", err))
	}

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failSource(ctx, src.ID, fmt.Errorf("ingest_poll: build source: %w", err))
	}

	items, err := source.Poll(ctx)
	if err != nil {
		return w.failSource(ctx, src.ID, fmt.Errorf("ingest_poll: poll: %w", err))
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
			log.Printf("ingest_poll: insert log entry for %s: %v", item.RemoteID, err)
			continue
		}

		payload, _ := json.Marshal(FetchJobPayload{
			SourceID:    src.ID,
			WorkspaceID: src.WorkspaceID,
			LogEntryID:  entry.ID,
			RemoteID:    item.RemoteID,
			Filename:    item.Filename,
		})
		if _, err := w.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestFetch, string(payload)); err != nil {
			log.Printf("ingest_poll: enqueue fetch for %s: %v", item.RemoteID, err)
		}
	}

	return w.markPolled(ctx, src.ID, nil)
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
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: get source: %w", err))
	}

	configJSON, err := DecryptConfig(w.appSecret, src.Config)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: decrypt config: %w", err))
	}

	source, err := Build(src.Type, configJSON)
	if err != nil {
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: build source: %w", err))
	}

	// Evaluate rules
	rules, err := w.db.ListIngressRules(ctx, src.ID)
	if err != nil {
		log.Printf("ingest_fetch: list rules for %s: %v (continuing without rules)", src.ID, err)
	}
	ruleResult := EvaluateRules(rules, ItemMeta{Filename: entry.Filename})
	if !ruleResult.Allow {
		skipped := "skipped"
		_ = w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
			Status: "skipped", ID: entry.ID, Error: &skipped,
		})
		return nil
	}

	// Determine destination: rule overrides source defaults
	projectID := src.DestProjectID
	folderID := src.DestFolderID
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
			return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: open tmp file: %w", err))
		}
		defer os.Remove(p.TmpPath)
		rc = f
	} else {
		rc, err = source.Fetch(ctx, IngestItem{RemoteID: entry.RemoteID, Filename: entry.Filename})
		if err != nil {
			return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: fetch item: %w", err))
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
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: create temp file: %w", err))
	}
	tmpPath := tmp.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmp, io.MultiReader(bytes.NewReader(sniff), rc)); err != nil {
		_ = tmp.Close()
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: write temp file: %w", err))
	}
	_ = tmp.Close()

	// Rename temp file to use original filename for CreateAsset
	namedTmp := filepath.Join(os.TempDir(), filepath.Base(entry.Filename))
	if err := os.Rename(tmpPath, namedTmp); err != nil {
		namedTmp = tmpPath // fall back to random name
	}
	defer os.Remove(namedTmp)

	asset, fErr := services.CreateAsset(ctx, w.db, w.storage, w.queue,
		src.WorkspaceID, namedTmp,
		services.AssetOptions{ProjectID: projectID, FolderID: folderID},
	)
	if fErr != nil {
		return w.failEntry(ctx, entry.ID, src.ID, fmt.Errorf("ingest_fetch: create asset: %s", fErr.Message))
	}

	assetID := asset.ID
	if err := w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status:  "imported",
		AssetID: &assetID,
		ID:      entry.ID,
	}); err != nil {
		log.Printf("ingest_fetch: update log entry %s: %v", entry.ID, err)
	}

	tag, err := w.db.GetOrCreateTag(ctx, dbgen.GetOrCreateTagParams{
		ID:          uuid.NewString(),
		WorkspaceID: asset.WorkspaceID,
		Name:        src.Label,
	})
	if err != nil {
		log.Printf("ingest_fetch: could not get or create tag: %v", err)
	}

	_ = w.db.AddTagToAsset(ctx, dbgen.AddTagToAssetParams{
		AssetID: assetID,
		TagID:   tag.ID,
	})

	return nil
}

func (w *Worker) failSource(ctx context.Context, sourceID string, err error) error {
	msg := err.Error()
	_ = w.db.MarkIngressSourcePolled(ctx, dbgen.MarkIngressSourcePolledParams{
		ID: sourceID, LastError: &msg,
	})
	return err
}

func (w *Worker) markPolled(ctx context.Context, sourceID string, pollErr error) error {
	var msg *string
	if pollErr != nil {
		s := pollErr.Error()
		msg = &s
	}
	return w.db.MarkIngressSourcePolled(ctx, dbgen.MarkIngressSourcePolledParams{
		ID: sourceID, LastError: msg,
	})
}

func (w *Worker) failEntry(ctx context.Context, entryID, sourceID string, err error) error {
	msg := err.Error()
	_ = w.db.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "error",
		Error:  &msg,
		ID:     entryID,
	})
	_ = w.db.MarkIngressSourcePolled(ctx, dbgen.MarkIngressSourcePolledParams{
		ID: sourceID, LastError: &msg,
	})
	return err
}
