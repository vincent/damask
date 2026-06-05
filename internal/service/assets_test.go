package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/audit"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"
)

func newAssetSvcSpy(t *testing.T) (service.AssetService, *memory.AssetRepo, *spyWriter) {
	t.Helper()
	spy := newSpy()
	repo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()
	return service.NewAssetService(
		repo,
		memory.NewVersionRepo(),
		memory.NewTagRepo(),
		memory.NewRealFieldRepo(),
		stor,
		spy,
		nil,
	), repo, spy
}

func newAssetSvc(t *testing.T) (service.AssetService, *memory.AssetRepo) {
	t.Helper()
	repo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()
	return service.NewAssetService(
		repo,
		memory.NewVersionRepo(),
		memory.NewTagRepo(),
		memory.NewRealFieldRepo(),
		stor,
		audit.NopWriter{},
		nil,
	), repo
}

// coverFlagRepo wraps AssetRepo and lets a single asset report as a project cover or workspace icon.
type coverFlagRepo struct {
	*memory.AssetRepo

	coverID string
	iconID  string
}

func (r *coverFlagRepo) IsProjectCover(_ context.Context, _, assetID string) (bool, error) {
	return assetID == r.coverID, nil
}
func (r *coverFlagRepo) IsWorkspaceIcon(_ context.Context, _, assetID string) (bool, error) {
	return assetID == r.iconID, nil
}

// --- Get ---

func TestAssetService_Get_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newAssetSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nonexistent")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Get_WrongWorkspace(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})
	_, err := svc.Get(context.Background(), "ws_B", "ast_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong workspace, got %v", err)
	}
}

func TestAssetService_Get_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "hero.jpg",
		MimeType:         "image/jpeg",
	})
	dto, err := svc.Get(context.Background(), "ws_1", "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.OriginalFilename != "hero.jpg" {
		t.Errorf("OriginalFilename: got %q, want %q", dto.OriginalFilename, "hero.jpg")
	}
	if dto.WorkspaceID != "ws_1" {
		t.Errorf("WorkspaceID: got %q, want %q", dto.WorkspaceID, "ws_1")
	}
	if dto.MimeType != "image/jpeg" {
		t.Errorf("MimeType: got %q, want %q", dto.MimeType, "image/jpeg")
	}
}

// --- List ---

func TestAssetService_List_Empty(t *testing.T) {
	t.Parallel()
	svc, _ := newAssetSvc(t)
	out, err := svc.List(context.Background(), service.ListAssetsParams{WorkspaceID: "ws_1", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d items", len(out))
	}
}

func TestAssetService_List_WorkspaceIsolation(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(
		repository.Asset{ID: "a1", WorkspaceID: "ws_A", OriginalFilename: "a.jpg"},
		repository.Asset{ID: "a2", WorkspaceID: "ws_B", OriginalFilename: "b.jpg"},
	)
	out, err := svc.List(context.Background(), service.ListAssetsParams{WorkspaceID: "ws_A", Limit: 50})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 {
		t.Fatalf("expected 1 asset for ws_A, got %d", len(out))
	}
	if out[0].ID != "a1" {
		t.Errorf("unexpected asset ID: %q", out[0].ID)
	}
}

func TestAssetService_List_SimilarToIDsRestrictsResults(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	folderID := "folder_1"
	otherFolderID := "folder_2"
	repo.Seed(
		repository.Asset{ID: "a1", WorkspaceID: "ws_1", FolderID: &folderID, OriginalFilename: "a.jpg"},
		repository.Asset{ID: "a2", WorkspaceID: "ws_1", FolderID: &folderID, OriginalFilename: "b.jpg"},
		repository.Asset{ID: "a3", WorkspaceID: "ws_1", FolderID: &otherFolderID, OriginalFilename: "c.jpg"},
		repository.Asset{ID: "a4", WorkspaceID: "ws_1", FolderID: &folderID, OriginalFilename: "d.jpg"},
	)

	out, err := svc.List(context.Background(), service.ListAssetsParams{
		WorkspaceID:  "ws_1",
		FolderID:     &folderID,
		SimilarToIDs: []string{"a1", "a3"},
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "a1" {
		t.Fatalf("expected only a1 after allowlist and folder intersection, got %#v", out)
	}
}

func TestAssetService_List_SimilarToIDsEmptyReturnsNoRows(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "a.jpg"})

	out, err := svc.List(context.Background(), service.ListAssetsParams{
		WorkspaceID:  "ws_1",
		SimilarToIDs: []string{},
		Limit:        50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Fatalf("expected empty allowlist to return no rows, got %d", len(out))
	}
}

func TestAssetService_List_SimilarToIDsNilAppliesNoRestriction(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(
		repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "a.jpg"},
		repository.Asset{ID: "a2", WorkspaceID: "ws_1", OriginalFilename: "b.jpg"},
	)

	out, err := svc.List(context.Background(), service.ListAssetsParams{
		WorkspaceID: "ws_1",
		Limit:       50,
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 2 {
		t.Fatalf("expected nil allowlist to apply no restriction, got %d", len(out))
	}
}

// --- Rename ---

func TestAssetService_Rename_EmptyStem(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	_, err := svc.Rename(context.Background(), "ws_1", "a1", "   ")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAssetService_Rename_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newAssetSvc(t)
	_, err := svc.Rename(context.Background(), "ws_1", "nope", "newname")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Rename_PreservesExtension(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	dto, err := svc.Rename(context.Background(), "ws_1", "a1", "banner")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.OriginalFilename != "banner.jpg" {
		t.Errorf("OriginalFilename: got %q, want %q", dto.OriginalFilename, "banner.jpg")
	}
}

func TestAssetService_Rename_NoOp(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	dto, err := svc.Rename(context.Background(), "ws_1", "a1", "photo")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.OriginalFilename != "photo.jpg" {
		t.Errorf("no-op rename should keep original filename, got %q", dto.OriginalFilename)
	}
}

// --- Move ---

func TestAssetService_Move_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newAssetSvc(t)
	folderID := "f1"
	_, err := svc.Move(context.Background(), "ws_1", "nope", service.MoveAssetParams{FolderID: &folderID})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Move_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "doc.pdf"})
	folderID := "folder_42"
	dto, err := svc.Move(context.Background(), "ws_1", "a1", service.MoveAssetParams{FolderID: &folderID})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.FolderID == nil || *dto.FolderID != "folder_42" {
		t.Errorf("FolderID: got %v, want %q", dto.FolderID, "folder_42")
	}
}

// --- Delete ---

func TestAssetService_Delete_NotFound(t *testing.T) {
	t.Parallel()
	svc, _ := newAssetSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Delete_OK(t *testing.T) {
	t.Parallel()
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "old.png"})
	if err := svc.Delete(context.Background(), "ws_1", "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.Get(context.Background(), "ws_1", "a1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}

func TestAssetService_Delete_ConflictProjectCover(t *testing.T) {
	t.Parallel()
	inner := memory.NewAssetRepo()
	inner.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "cover.jpg"})
	repo := &coverFlagRepo{AssetRepo: inner, coverID: "a1"}
	stor, _ := storage.NewAferoMemoryStorage()
	svc := service.NewAssetService(
		repo,
		memory.NewVersionRepo(),
		memory.NewTagRepo(),
		memory.NewRealFieldRepo(),
		stor,
		audit.NopWriter{},
		nil,
	)

	err := svc.Delete(context.Background(), "ws_1", "a1")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict when asset is project cover, got %v", err)
	}
}

func TestAssetService_Delete_ConflictWorkspaceIcon(t *testing.T) {
	t.Parallel()
	inner := memory.NewAssetRepo()
	inner.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "icon.png"})
	repo := &coverFlagRepo{AssetRepo: inner, iconID: "a1"}
	stor, _ := storage.NewAferoMemoryStorage()
	svc := service.NewAssetService(
		repo,
		memory.NewVersionRepo(),
		memory.NewTagRepo(),
		memory.NewRealFieldRepo(),
		stor,
		audit.NopWriter{},
		nil,
	)

	err := svc.Delete(context.Background(), "ws_1", "a1")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict when asset is workspace icon, got %v", err)
	}
}

// --- Audit events ---

func TestAssetService_Rename_EmitsAuditEvent(t *testing.T) {
	t.Parallel()
	svc, repo, spy := newAssetSvcSpy(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	if _, err := svc.Rename(context.Background(), "ws_1", "a1", "banner"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetRenamed {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetRenamed)
	}
	if e.AssetID != "a1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "a1")
	}
	if e.WorkspaceID != "ws_1" {
		t.Errorf("WorkspaceID: got %q, want %q", e.WorkspaceID, "ws_1")
	}
}

func TestAssetService_Rename_NoAuditOnNoOp(t *testing.T) {
	t.Parallel()
	svc, repo, spy := newAssetSvcSpy(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	if _, err := svc.Rename(context.Background(), "ws_1", "a1", "photo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if spy.assetCount() != 0 {
		t.Errorf("expected no audit event on no-op rename, got %d", spy.assetCount())
	}
}

func TestAssetService_Move_EmitsAuditEvent(t *testing.T) {
	t.Parallel()
	svc, repo, spy := newAssetSvcSpy(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "doc.pdf"})
	folderID := "folder_42"
	if _, err := svc.Move(
		context.Background(),
		"ws_1",
		"a1",
		service.MoveAssetParams{FolderID: &folderID},
	); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetMoved {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetMoved)
	}
	if e.AssetID != "a1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "a1")
	}
}

func TestAssetService_HardDelete_EmitsAuditEvent(t *testing.T) {
	t.Parallel()
	svc, repo, spy := newAssetSvcSpy(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "old.png"})
	if err := svc.HardDelete(context.Background(), "ws_1", "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetDeleted {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetDeleted)
	}
	if e.AssetID != "a1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "a1")
	}
}

// hookWriter wraps spyWriter and fires OnWriteAsset before recording the event,
// allowing tests to assert repository state at the moment the audit write occurs.
type hookWriter struct {
	spyWriter

	OnWriteAsset func(audit.AssetEvent)
}

func (h *hookWriter) WriteAsset(ctx context.Context, e audit.AssetEvent) {
	if h.OnWriteAsset != nil {
		h.OnWriteAsset(e)
	}
	h.spyWriter.WriteAsset(ctx, e)
}

// TestAssetService_HardDelete_AuditBeforeDelete verifies that the audit event is
// written before the asset row is removed, preventing a FK constraint failure in
// the real DB (asset_events.asset_id REFERENCES assets.id).
func TestAssetService_HardDelete_AuditBeforeDelete(t *testing.T) {
	t.Parallel()
	repo := memory.NewAssetRepo()
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1"})
	stor, _ := storage.NewAferoMemoryStorage()

	hw := &hookWriter{}
	hw.OnWriteAsset = func(_ audit.AssetEvent) {
		if _, err := repo.GetByID(context.Background(), "ws_1", "a1"); err != nil {
			t.Errorf("audit event fired after asset was deleted: %v", err)
		}
	}

	svc := service.NewAssetService(
		repo,
		memory.NewVersionRepo(),
		memory.NewTagRepo(),
		memory.NewRealFieldRepo(),
		stor,
		hw,
		nil,
	)
	if err := svc.HardDelete(context.Background(), "ws_1", "a1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if hw.assetCount() == 0 {
		t.Error("expected audit event to be emitted")
	}
}
