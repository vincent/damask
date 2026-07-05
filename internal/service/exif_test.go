package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"

	"github.com/google/uuid"
)

// newExifEnv opens a fresh in-memory SQLite DB (sqlc-backed repos are needed here:
// AssetFieldRepository has no memory implementation yet) and seeds a workspace.
func newExifEnv(t *testing.T) (*service.ExifService, *dbpkg.DB, string) {
	t.Helper()
	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = database.Close() })

	wsID := uuid.NewString()
	if _, wsErr := database.WQ.CreateWorkspace(context.Background(), dbgen.CreateWorkspaceParams{
		ID: wsID, Name: "test-workspace",
	}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}

	svc := service.NewExifService(
		reposqlc.NewWorkspaceRepo(database),
		reposqlc.NewAssetRepo(database),
		reposqlc.NewFieldRepo(database),
		reposqlc.NewAssetFieldRepo(database),
		nil,
	)
	return svc, database, wsID
}

func TestExifService_ExtractForAsset_ExifKeepDisabled_NoOp(t *testing.T) {
	svc, _, wsID := newExifEnv(t)
	// exif_keep defaults to 0 (disabled); should return immediately without
	// touching storage or looking up the asset at all.
	if err := svc.ExtractForAsset(context.Background(), wsID, "nonexistent-asset", "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

func TestExifService_ExtractForAsset_AssetNotFound_NoOp(t *testing.T) {
	svc, database, wsID := newExifEnv(t)
	if _, err := database.Writer.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, wsID); err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}
	if err := svc.ExtractForAsset(context.Background(), wsID, "nonexistent-asset", "user-1"); err != nil {
		t.Fatalf("expected nil (not-found is a no-op), got: %v", err)
	}
}

func TestExifService_ExtractForAsset_NonImageAsset_NoOp(t *testing.T) {
	svc, database, wsID := newExifEnv(t)
	if _, err := database.Writer.Exec(`UPDATE workspaces SET exif_keep = 1 WHERE id = ?`, wsID); err != nil {
		t.Fatalf("enable exif_keep: %v", err)
	}
	assetID := uuid.NewString()
	if _, err := database.WQ.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:                   assetID,
		WorkspaceID:          wsID,
		OriginalFilename:     "doc.pdf",
		StorageKey:           "key/doc.pdf",
		MimeType:             "application/pdf",
		Size:                 10,
		ThumbnailContentType: "image/webp",
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
	// Non-image asset: should return before ever touching storage (nil storage would panic otherwise).
	if err := svc.ExtractForAsset(context.Background(), wsID, assetID, "user-1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}

// -- Cross-workspace negative test --
// FieldRepository.EnsureSystemField/ListBySource (added for exif extraction) must
// not leak system field definitions across workspaces.

func TestFieldRepository_SystemFieldCrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewRealFieldRepo()

	wsA, wsB := "ws-a", "ws-b"
	if err := repo.EnsureSystemField(ctx, repository.EnsureSystemFieldParams{
		ID: "field-1", WorkspaceID: wsA, Source: "exif", Name: "Camera maker", Key: "_exif_make", FieldType: "text",
	}); err != nil {
		t.Fatalf("ensure system field: %v", err)
	}

	out, err := repo.ListBySource(ctx, wsB, "exif")
	if err != nil || len(out) != 0 {
		t.Errorf("ListBySource cross-workspace: expected empty, got %v, err=%v", out, err)
	}

	if _, keyErr := repo.GetByKey(ctx, wsB, "_exif_make"); !errors.Is(keyErr, apperr.ErrNotFound) {
		t.Errorf("GetByKey cross-workspace: expected ErrNotFound, got %v", keyErr)
	}

	// Sanity: same-workspace access still works after all the negative checks above.
	sameWS, err := repo.ListBySource(ctx, wsA, "exif")
	if err != nil || len(sameWS) != 1 {
		t.Errorf("ListBySource same-workspace: expected 1 field, got %v, err=%v", sameWS, err)
	}
}
