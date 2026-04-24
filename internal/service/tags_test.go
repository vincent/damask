package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
)

func newTagSvc(t *testing.T) (service.TagService, *memory.RealTagRepo) {
	t.Helper()
	repo := memory.NewRealTagRepo()
	return service.NewTagService(repo), repo
}

// --- Create ---

func TestTagService_Create_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "Nature"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "nature" {
		t.Errorf("Name: got %q, want %q (should be lowercased)", dto.Name, "nature")
	}
}

func TestTagService_Create_EmptyName(t *testing.T) {
	svc, _ := newTagSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestTagService_Create_Conflict(t *testing.T) {
	svc, _ := newTagSvc(t)
	if _, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "dup"}); err != nil {
		t.Fatalf("first create failed: %v", err)
	}
	_, err := svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "dup"})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

// --- List ---

func TestTagService_List_WorkspaceIsolation(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_A", service.CreateTagParams{Name: "alpha"})  //nolint
	svc.Create(context.Background(), "ws_B", service.CreateTagParams{Name: "beta"})   //nolint
	tags, err := svc.List(context.Background(), "ws_A")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(tags) != 1 || tags[0].Name != "alpha" {
		t.Errorf("expected [alpha], got %v", tags)
	}
}

// --- Patch (rename) ---

func TestTagService_Patch_Rename_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "old"}) //nolint
	newName := "new"
	dto, err := svc.Patch(context.Background(), "ws_1", "old", service.PatchTagParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "new" {
		t.Errorf("Name: got %q, want %q", dto.Name, "new")
	}
}

func TestTagService_Patch_Rename_Conflict(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "first"})  //nolint
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "second"}) //nolint
	conflict := "second"
	_, err := svc.Patch(context.Background(), "ws_1", "first", service.PatchTagParams{Name: &conflict})
	if !errors.Is(err, apperr.ErrConflict) {
		t.Fatalf("expected ErrConflict, got %v", err)
	}
}

func TestTagService_Patch_NotFound(t *testing.T) {
	svc, _ := newTagSvc(t)
	name := "x"
	_, err := svc.Patch(context.Background(), "ws_1", "missing", service.PatchTagParams{Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- AddToAsset / RemoveFromAsset ---

func TestTagService_AddToAsset_Idempotent(t *testing.T) {
	svc, _ := newTagSvc(t)
	if _, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("first add: %v", err)
	}
	if _, err := svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("second add (should be idempotent): %v", err)
	}
	tags, err := svc.ListForAsset(context.Background(), "ast_1")
	if err != nil {
		t.Fatalf("list: %v", err)
	}
	if len(tags) != 1 {
		t.Errorf("expected 1 tag, got %d", len(tags))
	}
}

func TestTagService_RemoveFromAsset_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.AddToAsset(context.Background(), "ws_1", "ast_1", "photo") //nolint
	if err := svc.RemoveFromAsset(context.Background(), "ws_1", "ast_1", "photo"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, _ := svc.ListForAsset(context.Background(), "ast_1")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after remove, got %d", len(tags))
	}
}

// --- Delete ---

func TestTagService_Delete_OK(t *testing.T) {
	svc, _ := newTagSvc(t)
	svc.Create(context.Background(), "ws_1", service.CreateTagParams{Name: "gone"}) //nolint
	if err := svc.Delete(context.Background(), "ws_1", []string{"gone"}); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	tags, _ := svc.List(context.Background(), "ws_1")
	if len(tags) != 0 {
		t.Errorf("expected 0 tags after delete, got %d", len(tags))
	}
}
