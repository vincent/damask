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

func newFolderSvc(t *testing.T) (service.FolderService, *memory.RealFolderRepo) {
	t.Helper()
	repo := memory.NewRealFolderRepo()
	return service.NewFolderService(repo), repo
}

// --- Create ---

func TestFolderService_Create_OK(t *testing.T) {
	svc, _ := newFolderSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "Drafts"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Drafts" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Drafts")
	}
	if dto.ProjectID != "proj_1" {
		t.Errorf("ProjectID: got %q, want %q", dto.ProjectID, "proj_1")
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestFolderService_Create_EmptyName(t *testing.T) {
	svc, _ := newFolderSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFolderService_Create_ParentNotFound(t *testing.T) {
	svc, _ := newFolderSvc(t)
	parentID := "nonexistent"
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "Child",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFolderService_Create_MaxDepth(t *testing.T) {
	svc, repo := newFolderSvc(t)
	parentID := "parent_1"
	grandparentID := "grand_1"
	// parent_1 itself has a parent (grandparent), so creating a child under parent_1 exceeds depth 2
	repo.Seed(repository.Folder{
		ID:          parentID,
		WorkspaceID: "ws_1",
		ProjectID:   "proj_1",
		ParentID:    &grandparentID,
		Name:        "Level2",
	})
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "TooDeep",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (max depth), got %v", err)
	}
}

func TestFolderService_Create_ParentDifferentProject(t *testing.T) {
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{
		ID:          "parent_1",
		WorkspaceID: "ws_1",
		ProjectID:   "proj_OTHER",
		Name:        "OtherProj",
	})
	parentID := "parent_1"
	_, err := svc.Create(context.Background(), "ws_1", "proj_1", service.CreateFolderParams{
		Name:     "BadChild",
		ParentID: &parentID,
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (wrong project), got %v", err)
	}
}

// --- Get ---

func TestFolderService_Get_NotFound(t *testing.T) {
	svc, _ := newFolderSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

// --- Update ---

func TestFolderService_Update_EmptyName(t *testing.T) {
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{ID: "f1", WorkspaceID: "ws_1", ProjectID: "p1", Name: "Original"})
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", "f1", service.UpdateFolderParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestFolderService_Update_OK(t *testing.T) {
	svc, repo := newFolderSvc(t)
	repo.Seed(repository.Folder{ID: "f1", WorkspaceID: "ws_1", ProjectID: "p1", Name: "Old"})
	newName := "New"
	dto, err := svc.Update(context.Background(), "ws_1", "f1", service.UpdateFolderParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "New" {
		t.Errorf("Name: got %q, want %q", dto.Name, "New")
	}
}

// --- Delete ---

func TestFolderService_Delete_NotFound(t *testing.T) {
	svc, _ := newFolderSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestFolderService_Delete_CascadesChildren(t *testing.T) {
	svc, repo := newFolderSvc(t)
	parentID := "parent_1"
	repo.Seed(
		repository.Folder{ID: parentID, WorkspaceID: "ws_1", ProjectID: "p1", Name: "Parent"},
		repository.Folder{ID: "child_1", WorkspaceID: "ws_1", ProjectID: "p1", ParentID: &parentID, Name: "Child"},
	)
	if err := svc.Delete(context.Background(), "ws_1", parentID); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if _, err := svc.Get(context.Background(), "ws_1", parentID); !errors.Is(err, apperr.ErrNotFound) {
		t.Error("parent should be deleted")
	}
	if _, err := svc.Get(context.Background(), "ws_1", "child_1"); !errors.Is(err, apperr.ErrNotFound) {
		t.Error("child should be deleted")
	}
}
