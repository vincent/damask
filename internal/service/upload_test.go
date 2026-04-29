package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mediatype"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

func newUploadSvcSpy(t *testing.T) (service.UploadService, *spyWriter) {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(t.TempDir() + "/upload_spy.db?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	stor, _ := storage.NewAferoMemoryStorage()
	spy := newSpy()
	q := queue.New(queries, 1)
	injestor := service.NewAssetInjestor(queries, sqlDB, stor, q, mediatype.NewRegistry(transform.NewTransformer()))
	return service.NewUploadService(injestor, spy), spy
}

func newUploadSvc(t *testing.T) service.UploadService {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	q := queue.New(queries, 1)
	injestor := service.NewAssetInjestor(queries, sqlDB, stor, q, mediatype.NewRegistry(transform.NewTransformer()))
	return service.NewUploadService(injestor, audit.NopWriter{})
}

// -- Validate inputs --

func TestUploadService_Ingest_EmptyWorkspace(t *testing.T) {
	svc := newUploadSvc(t)
	_, err := svc.Ingest(context.Background(), "", strings.NewReader("data"), service.UploadMeta{
		OriginalFilename: "test.txt",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty workspaceID, got %v", err)
	}
}

func TestUploadService_Ingest_EmptyFilename(t *testing.T) {
	svc := newUploadSvc(t)
	_, err := svc.Ingest(context.Background(), "ws_1", strings.NewReader("data"), service.UploadMeta{
		OriginalFilename: "",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for empty filename, got %v", err)
	}
}

// -- Happy path --

func TestUploadService_Ingest_OK(t *testing.T) {
	stor, _ := storage.NewAferoMemoryStorage()
	queries, sqlDB, err := dbpkg.Open(t.TempDir() + "/upload_test.db?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	wsID := "ws_upload"
	userID := "user_upload"
	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{ID: userID, Email: "u@t.com", PasswordHash: "x", Name: "t"}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	q2 := queue.New(queries, 1)
	svc := service.NewUploadService(service.NewAssetInjestor(queries, sqlDB, stor, q2, mediatype.NewRegistry(transform.NewTransformer())), audit.NopWriter{})

	dto, err := svc.Ingest(ctx, wsID, strings.NewReader("fake image bytes"), service.UploadMeta{
		OriginalFilename: "photo.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.OriginalFilename != "photo.jpg" {
		t.Errorf("OriginalFilename: got %q, want %q", dto.OriginalFilename, "photo.jpg")
	}
	if dto.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", dto.WorkspaceID, wsID)
	}
	if dto.ID == "" {
		t.Errorf("expected non-empty asset ID")
	}
}

// --- Audit events ---

func TestUploadService_Ingest_EmitsAuditEvent(t *testing.T) {
	var svc service.UploadService
	_, spy := newUploadSvcSpy(t)

	queries, sqlDB, err := dbpkg.Open(t.TempDir() + "/upload_audit.db?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()
	wsID := "ws_audit"
	userID := "user_audit"
	if _, err := queries.CreateWorkspace(ctx, dbgen.CreateWorkspaceParams{ID: wsID, Name: "test"}); err != nil {
		t.Fatalf("seed workspace: %v", err)
	}
	if _, err := queries.CreateUser(ctx, dbgen.CreateUserParams{ID: userID, Email: "a@t.com", PasswordHash: "x", Name: "t"}); err != nil {
		t.Fatalf("seed user: %v", err)
	}

	// Rebuild svc with the seeded DB so the workspace FK constraint passes.
	stor, _ := storage.NewAferoMemoryStorage()
	q := queue.New(queries, 1)
	svc = service.NewUploadService(service.NewAssetInjestor(queries, sqlDB, stor, q, mediatype.NewRegistry(transform.NewTransformer())), spy)

	_, err = svc.Ingest(ctx, wsID, strings.NewReader("bytes"), service.UploadMeta{
		OriginalFilename: "shot.jpg",
		UserID:           userID,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetCreated {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetCreated)
	}
	if e.WorkspaceID != wsID {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, wsID)
	}
}
