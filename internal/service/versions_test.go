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
)

func newVersionSvc(t *testing.T) (service.VersionService, *memory.RealVersionRepo) {
	t.Helper()
	repo := memory.NewRealVersionRepo()
	return service.NewVersionService(repo, audit.NopWriter{}), repo
}

func newVersionSvcSpy(t *testing.T) (service.VersionService, *memory.RealVersionRepo, *spyWriter) {
	t.Helper()
	spy := newSpy()
	repo := memory.NewRealVersionRepo()
	return service.NewVersionService(repo, spy), repo, spy
}

// --- List ---

func TestVersionService_List_Empty(t *testing.T) {
	svc, _ := newVersionSvc(t)
	out, err := svc.List(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

func TestVersionService_List_ByAsset(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(
		repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1"},
		repository.AssetVersion{ID: "v2", AssetID: "ast_2", WorkspaceID: "ws_1"},
	)
	out, err := svc.List(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 1 || out[0].ID != "v1" {
		t.Errorf("expected [v1], got %v", out)
	}
}

// --- Get ---

func TestVersionService_Get_NotFound(t *testing.T) {
	svc, _ := newVersionSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_Get_WrongWorkspace(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_A"})
	_, err := svc.Get(context.Background(), "ws_B", "v1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong workspace, got %v", err)
	}
}

// --- Delete ---

func TestVersionService_Delete_NotFound(t *testing.T) {
	svc, _ := newVersionSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVersionService_Delete_CurrentVersion(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: true,
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (current version), got %v", err)
	}
}

func TestVersionService_Delete_AssetMismatch(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_OTHER", WorkspaceID: "ws_1", IsCurrent: false,
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound (wrong asset), got %v", err)
	}
}

func TestVersionService_Delete_ReferencedAsCover(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	repo.MarkAsCover("v1")
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1")
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict (cover), got %v", err)
	}
}

func TestVersionService_Delete_OK(t *testing.T) {
	svc, repo := newVersionSvc(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	v, err := repo.GetByID(context.Background(), "v1")
	if err != nil {
		t.Fatalf("unexpected error getting soft-deleted version: %v", err)
	}
	if v.DeletedAt == nil {
		t.Error("expected DeletedAt to be set after soft-delete")
	}
}

// --- Audit events ---

func TestVersionService_Delete_EmitsAuditEvent(t *testing.T) {
	svc, repo, spy := newVersionSvcSpy(t)
	repo.Seed(repository.AssetVersion{
		ID: "v1", AssetID: "ast_1", WorkspaceID: "ws_1", IsCurrent: false,
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "v1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionDeleted {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionDeleted)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVersionService_WriteVersionUploaded_EmitsAuditEvent(t *testing.T) {
	svc, _, spy := newVersionSvcSpy(t)
	v := &service.VersionDTO{ID: "v1", VersionNum: 2, Size: 1024}
	svc.WriteVersionUploaded(context.Background(), "ws_1", "ast_1", v, "initial upload")
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionUploaded {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionUploaded)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}

func TestVersionService_WriteVersionRestored_EmitsAuditEvent(t *testing.T) {
	svc, _, spy := newVersionSvcSpy(t)
	svc.WriteVersionRestored(context.Background(), "ws_1", "ast_1", 3, 1)
	e := spy.lastAsset()
	if e.EventType != audit.EventAssetVersionRestored {
		t.Errorf("EventType: got %q, want %q", e.EventType, audit.EventAssetVersionRestored)
	}
	if e.AssetID != "ast_1" {
		t.Errorf("AssetID: got %q, want %q", e.AssetID, "ast_1")
	}
}
