package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newAssetSvc(t *testing.T) (service.AssetService, *memory.AssetRepo) {
	t.Helper()
	repo := memory.NewAssetRepo()
	return service.NewAssetService(repo), repo
}

// --- Get ---

func TestAssetService_Get_NotFound(t *testing.T) {
	svc, _ := newAssetSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nonexistent")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Get_WrongWorkspace(t *testing.T) {
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})
	_, err := svc.Get(context.Background(), "ws_B", "ast_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong workspace, got %v", err)
	}
}

func TestAssetService_Get_OK(t *testing.T) {
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

// --- Rename ---

func TestAssetService_Rename_EmptyStem(t *testing.T) {
	svc, repo := newAssetSvc(t)
	repo.Seed(repository.Asset{ID: "a1", WorkspaceID: "ws_1", OriginalFilename: "photo.jpg"})
	_, err := svc.Rename(context.Background(), "ws_1", "a1", "   ")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestAssetService_Rename_NotFound(t *testing.T) {
	svc, _ := newAssetSvc(t)
	_, err := svc.Rename(context.Background(), "ws_1", "nope", "newname")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Rename_PreservesExtension(t *testing.T) {
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
	svc, _ := newAssetSvc(t)
	folderID := "f1"
	_, err := svc.Move(context.Background(), "ws_1", "nope", service.MoveAssetParams{FolderID: &folderID})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Move_OK(t *testing.T) {
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
	svc, _ := newAssetSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Delete_OK(t *testing.T) {
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
