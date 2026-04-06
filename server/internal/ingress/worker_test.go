package ingress

import (
	"context"
	"encoding/json"
	"io"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"

	"github.com/google/uuid"
)

const testAppSecret = "test-app-secret-for-tests!!"

// pollSource is a fakeSource that returns a fixed list of items from Poll
// and serves file content from Fetch.
type pollSource struct {
	typ   string
	items []IngestItem
}

func (p *pollSource) Type() string                     { return p.typ }
func (p *pollSource) Validate(_ context.Context) error { return nil }
func (p *pollSource) Poll(_ context.Context) ([]IngestItem, error) {
	return p.items, nil
}
func (p *pollSource) Fetch(_ context.Context, item IngestItem) (io.ReadCloser, error) {
	return io.NopCloser(strings.NewReader("fetched:" + item.RemoteID)), nil
}

func setupWorkerTest(t *testing.T) (*Worker, *dbgen.Queries) {
	t.Helper()
	dir := t.TempDir()
	queries, sqlDB, err := dbpkg.Open(filepath.Join(dir, "test.db"))
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	stor, err := storage.NewLocalStorage(filepath.Join(dir, "storage"))
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	cfg := &config.Config{AppSecret: testAppSecret}
	q := queue.New(queries, 1)
	w := NewWorker(queries, sqlDB, stor, q, cfg)
	return w, queries
}

// insertWorkspaceAndSource inserts the minimum rows needed for HandleFetch:
// a workspace, a user, and an ingress source with the given label.
func insertWorkspaceAndSource(t *testing.T, queries *dbgen.Queries, label string) (workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = uuid.NewString()
	userID := uuid.NewString()
	sourceID = uuid.NewString()

	// Insert workspace
	_, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	// Insert user (required by ingress_sources.created_by FK)
	_, err = queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           userID,
		Email:        "worker-test@example.com",
		PasswordHash: "x",
		Name:         "Worker Test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	// Encrypt an empty config for a fake source type
	Register("fake_worker_test", func(configJSON []byte) (Source, error) {
		return &fakeSource{typ: "fake_worker_test"}, nil
	})
	configJSON, err := EncryptConfig(testAppSecret, []byte(`{}`))
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	_, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            "fake_worker_test",
		Label:           label,
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})
	if err != nil {
		t.Fatalf("create ingress source: %v", err)
	}

	return workspaceID, sourceID
}

// insertWorkspaceAndPollSource is like insertWorkspaceAndSource but registers
// a pollSource that returns items and can serve Fetch content.
func insertWorkspaceAndPollSource(t *testing.T, queries *dbgen.Queries, label string, items []IngestItem) (workspaceID, sourceID string) {
	t.Helper()
	ctx := context.Background()
	workspaceID = uuid.NewString()
	userID := uuid.NewString()
	sourceID = uuid.NewString()

	_, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: "Test Workspace",
	})
	if err != nil {
		t.Fatalf("create workspace: %v", err)
	}

	_, err = queries.CreateUser(ctx, dbgen.CreateUserParams{
		ID:           userID,
		Email:        "poll-test@example.com",
		PasswordHash: "x",
		Name:         "Poll Test",
	})
	if err != nil {
		t.Fatalf("create user: %v", err)
	}

	const typ = "fake_poll_source"
	Register(typ, func(configJSON []byte) (Source, error) {
		return &pollSource{typ: typ, items: items}, nil
	})
	configJSON, err := EncryptConfig(testAppSecret, []byte(`{}`))
	if err != nil {
		t.Fatalf("encrypt config: %v", err)
	}

	_, err = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            typ,
		Label:           label,
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})
	if err != nil {
		t.Fatalf("create ingress source: %v", err)
	}

	return workspaceID, sourceID
}

func TestHandleFetch_TagsAssetWithSourceLabel(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	const label = "my-source-label"
	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, label)

	// Insert a log entry
	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-1",
		Filename: "test.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Write a real temp file so HandleFetch can use TmpPath
	tmp, err := os.CreateTemp("", "worker-test-*.txt")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	if _, err := tmp.WriteString("hello world"); err != nil {
		t.Fatalf("write temp file: %v", err)
	}
	_ = tmp.Close()

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-1",
		Filename:    "test.txt",
		TmpPath:     tmp.Name(),
	})

	job := dbgen.Job{Payload: string(payload)}
	if err := w.HandleFetch(ctx, job); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	// Verify the log entry was marked imported and has an asset ID
	updated, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if updated.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", updated.Status)
	}
	if updated.AssetID == nil || *updated.AssetID == "" {
		t.Fatal("expected non-nil asset_id on log entry")
	}

	// Verify the asset has the source label as a tag
	tags, err := queries.GetTagsForAsset(ctx, *updated.AssetID)
	if err != nil {
		t.Fatalf("get tags for asset: %v", err)
	}
	for _, tag := range tags {
		if tag.Name == label {
			return // found
		}
	}
	t.Fatalf("expected tag %q on asset %s, got %v", label, *updated.AssetID, tags)
}

func TestHandleFetch_Idempotent_AlreadyImported(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, "label")

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-idem",
		Filename: "idem.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// Mark it already imported — no real asset needed, just a non-pending status
	if err := queries.UpdateIngressLogEntry(ctx, dbgen.UpdateIngressLogEntryParams{
		Status: "imported",
		ID:     entry.ID,
	}); err != nil {
		t.Fatalf("pre-mark imported: %v", err)
	}

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-idem",
		Filename:    "idem.txt",
	})

	// HandleFetch should return nil without doing anything
	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch on already-imported entry should not error: %v", err)
	}

	// Status must still be "imported" (unchanged)
	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
}

func TestHandleFetch_DenyRule_MarksSkipped(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndSource(t, queries, "deny-label")

	// Add a deny rule matching *.exe filenames
	_, err := queries.CreateIngressRule(ctx, dbgen.CreateIngressRuleParams{
		ID:       uuid.NewString(),
		SourceID: sourceID,
		Position: 1,
		Field:    "filename",
		Operator: "ends_with",
		Value:    ".exe",
		Action:   "deny",
	})
	if err != nil {
		t.Fatalf("create rule: %v", err)
	}

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "remote-exe",
		Filename: "malware.exe",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	tmp, err := os.CreateTemp("", "worker-deny-*.exe")
	if err != nil {
		t.Fatalf("create temp file: %v", err)
	}
	_, _ = tmp.WriteString("MZ")
	_ = tmp.Close()

	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "remote-exe",
		Filename:    "malware.exe",
		TmpPath:     tmp.Name(),
	})

	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch: %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "skipped" {
		t.Fatalf("expected status 'skipped', got %q", after.Status)
	}
	if after.AssetID != nil {
		t.Fatal("expected no asset_id on skipped entry")
	}
}

func TestHandleFetch_PullSource_FetchesViaSource(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	const typ = "fake_pull_fetch"
	Register(typ, func(configJSON []byte) (Source, error) {
		return &pollSource{typ: typ}, nil
	})

	userID := uuid.NewString()
	workspaceID := uuid.NewString()
	sourceID := uuid.NewString()

	_, _ = queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: workspaceID, Name: "WS"})
	_, _ = queries.CreateUser(ctx, dbgen.CreateUserParams{ID: userID, Email: "pull@example.com", PasswordHash: "x", Name: "Pull"})
	configJSON, _ := EncryptConfig(testAppSecret, []byte(`{}`))
	_, _ = queries.CreateIngressSource(ctx, dbgen.CreateIngressSourceParams{
		ID:              sourceID,
		WorkspaceID:     workspaceID,
		CreatedBy:       userID,
		Type:            typ,
		Label:           "pull-label",
		Config:          configJSON,
		PublicToken:     uuid.NewString(),
		Enabled:         1,
		PollIntervalMin: 60,
	})

	entryID := uuid.NewString()
	entry, err := queries.InsertIngressLogEntry(ctx, dbgen.InsertIngressLogEntryParams{
		ID:       entryID,
		SourceID: sourceID,
		RemoteID: "pull-remote-1",
		Filename: "pulled.txt",
	})
	if err != nil {
		t.Fatalf("insert log entry: %v", err)
	}

	// No TmpPath — worker must call source.Fetch()
	payload, _ := json.Marshal(FetchJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
		LogEntryID:  entry.ID,
		RemoteID:    "pull-remote-1",
		Filename:    "pulled.txt",
	})

	if err := w.HandleFetch(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandleFetch (pull): %v", err)
	}

	after, err := queries.GetIngressLogEntry(ctx, entryID)
	if err != nil {
		t.Fatalf("get log entry: %v", err)
	}
	if after.Status != "imported" {
		t.Fatalf("expected status 'imported', got %q", after.Status)
	}
	if after.AssetID == nil || *after.AssetID == "" {
		t.Fatal("expected asset_id on pulled entry")
	}
}

func TestHandleFetch_BadPayload_ReturnsError(t *testing.T) {
	w, _ := setupWorkerTest(t)
	ctx := context.Background()

	err := w.HandleFetch(ctx, dbgen.Job{Payload: "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}

func TestHandlePoll_EnqueuesItemsFromSource(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	items := []IngestItem{
		{RemoteID: "r1", Filename: "file1.jpg"},
		{RemoteID: "r2", Filename: "file2.png"},
	}
	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "poll-label", items)

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandlePoll: %v", err)
	}

	// Both items should have log entries
	// Note: ListIngressSourceLog SQL uses LIMIT ?3 OFFSET ?2, so the struct
	// fields are positionally swapped: Limit goes to OFFSET, Offset goes to LIMIT.
	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    0,  // maps to SQL OFFSET
		Offset:   10, // maps to SQL LIMIT
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 2 {
		t.Fatalf("expected 2 log entries, got %d", len(log))
	}
}

func TestHandlePoll_DuplicateItem_IsSkipped(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	items := []IngestItem{
		{RemoteID: "dup-1", Filename: "dup.jpg"},
	}
	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "dedup-label", items)

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	// First poll — creates log entry
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("first HandlePoll: %v", err)
	}

	// Second poll — same remote_id must be deduplicated
	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("second HandlePoll: %v", err)
	}

	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    0,  // maps to SQL OFFSET
		Offset:   10, // maps to SQL LIMIT
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 1 {
		t.Fatalf("expected 1 log entry after dedup, got %d", len(log))
	}
}

func TestHandlePoll_DisabledSource_DoesNothing(t *testing.T) {
	w, queries := setupWorkerTest(t)
	ctx := context.Background()

	workspaceID, sourceID := insertWorkspaceAndPollSource(t, queries, "disabled-label", []IngestItem{
		{RemoteID: "x", Filename: "x.jpg"},
	})

	// Disable the source
	src, err := queries.GetIngressSource(ctx, dbgen.GetIngressSourceParams{
		ID: sourceID, WorkspaceID: workspaceID,
	})
	if err != nil {
		t.Fatalf("get source: %v", err)
	}
	_, _ = queries.UpdateIngressSource(ctx, dbgen.UpdateIngressSourceParams{
		Label:           src.Label,
		Config:          src.Config,
		DestFolderID:    src.DestFolderID,
		DestProjectID:   src.DestProjectID,
		Enabled:         0,
		PollIntervalMin: src.PollIntervalMin,
		ID:              src.ID,
		WorkspaceID:     src.WorkspaceID,
	})

	payload, _ := json.Marshal(PollJobPayload{
		SourceID:    sourceID,
		WorkspaceID: workspaceID,
	})

	if err := w.HandlePoll(ctx, dbgen.Job{Payload: string(payload)}); err != nil {
		t.Fatalf("HandlePoll on disabled source: %v", err)
	}

	log, err := queries.ListIngressSourceLog(ctx, dbgen.ListIngressSourceLogParams{
		SourceID: sourceID,
		Limit:    0,  // maps to SQL OFFSET
		Offset:   10, // maps to SQL LIMIT
	})
	if err != nil {
		t.Fatalf("list log: %v", err)
	}
	if len(log) != 0 {
		t.Fatalf("expected no log entries for disabled source, got %d", len(log))
	}
}

func TestHandlePoll_BadPayload_ReturnsError(t *testing.T) {
	w, _ := setupWorkerTest(t)
	ctx := context.Background()

	err := w.HandlePoll(ctx, dbgen.Job{Payload: "not-json"})
	if err == nil {
		t.Fatal("expected error for invalid JSON payload")
	}
}
