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

func newProjectSvc(t *testing.T) (service.ProjectService, *memory.ProjectRepo) {
	t.Helper()
	repo := memory.NewProjectRepo()
	svc := service.NewProjectService(repo, nil) // nil sqlDB: memory repo ignores transactions
	return svc, repo
}

// --- Create ---

func TestProjectService_Create_OK(t *testing.T) {
	svc, _ := newProjectSvc(t)
	dto, err := svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "Alpha"})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Alpha" || dto.WorkspaceID != "ws_1" {
		t.Errorf("unexpected dto: %+v", dto)
	}
	if dto.ID == "" {
		t.Error("expected non-empty ID")
	}
}

func TestProjectService_Create_EmptyName(t *testing.T) {
	svc, _ := newProjectSvc(t)
	_, err := svc.Create(context.Background(), "ws_1", service.CreateProjectParams{Name: "   "})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// --- Get ---

func TestProjectService_Get_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "missing")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Get_WrongWorkspace(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_A", Name: "X"})
	_, err := svc.Get(context.Background(), "ws_B", "p1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Get_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Beta"})
	dto, err := svc.Get(context.Background(), "ws_1", "p1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Beta" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Beta")
	}
}

// --- Update ---

func TestProjectService_Update_EmptyName(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Gamma"})
	empty := ""
	_, err := svc.Update(context.Background(), "ws_1", "p1", service.UpdateProjectParams{Name: &empty})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestProjectService_Update_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	name := "X"
	_, err := svc.Update(context.Background(), "ws_1", "nope", service.UpdateProjectParams{Name: &name})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Update_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "Old"})
	newName := "New"
	dto, err := svc.Update(context.Background(), "ws_1", "p1", service.UpdateProjectParams{Name: &newName})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "New" {
		t.Errorf("Name: got %q, want %q", dto.Name, "New")
	}
}

// --- Delete ---

func TestProjectService_Delete_NotFound(t *testing.T) {
	svc, _ := newProjectSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestProjectService_Delete_OK(t *testing.T) {
	svc, repo := newProjectSvc(t)
	repo.Seed(repository.Project{ID: "p1", WorkspaceID: "ws_1", Name: "ToDelete"})
	if err := svc.Delete(context.Background(), "ws_1", "p1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.Get(context.Background(), "ws_1", "p1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
