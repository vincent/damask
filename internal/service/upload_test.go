package service_test

import (
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/service"
	"damask/server/internal/storage"
)

func newUploadSvc(t *testing.T) service.UploadService {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { sqlDB.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	q := queue.New(queries, 1)
	return service.NewUploadService(queries, sqlDB, stor, q)
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
	t.Cleanup(func() { sqlDB.Close() })

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
	svc := service.NewUploadService(queries, sqlDB, stor, q2)

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
