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

func newWorkspaceSvc(t *testing.T) (service.WorkspaceService, *memory.RealWorkspaceRepo) {
	t.Helper()
	repo := memory.NewRealWorkspaceRepo()
	return service.NewWorkspaceService(repo, memory.NewUserRepo(), "test-app-secret", ""), repo
}

// --- Get ---

func TestWorkspaceService_Get_NotFound(t *testing.T) {
	svc, _ := newWorkspaceSvc(t)
	_, err := svc.Get(context.Background(), "ws_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestWorkspaceService_Get_OK(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Acme", VersionRetentionCount: 5})
	dto, err := svc.Get(context.Background(), "ws_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Name != "Acme" {
		t.Errorf("Name: got %q, want %q", dto.Name, "Acme")
	}
	if dto.VersionRetentionCount != 5 {
		t.Errorf("VersionRetentionCount: got %d, want 5", dto.VersionRetentionCount)
	}
}

// --- Update ---

func TestWorkspaceService_Update_VersionRetention(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Test", VersionRetentionCount: 3})
	newCount := int64(10)
	dto, err := svc.Update(context.Background(), "ws_1", service.UpdateWorkspaceParams{VersionRetentionCount: &newCount})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.VersionRetentionCount != 10 {
		t.Errorf("VersionRetentionCount: got %d, want 10", dto.VersionRetentionCount)
	}
}

func TestWorkspaceService_Update_ExifSettings(t *testing.T) {
	svc, repo := newWorkspaceSvc(t)
	repo.Seed(repository.Workspace{ID: "ws_1", Name: "Test", ExifKeep: false, ExifKeepGps: false})
	keepTrue := true
	dto, err := svc.Update(context.Background(), "ws_1", service.UpdateWorkspaceParams{ExifKeep: &keepTrue})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !dto.ExifKeep {
		t.Error("ExifKeep should be true after update")
	}
}

func TestWorkspaceService_Update_NotFound(t *testing.T) {
	svc, _ := newWorkspaceSvc(t)
	count := int64(5)
	_, err := svc.Update(context.Background(), "ws_nope", service.UpdateWorkspaceParams{VersionRetentionCount: &count})
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}
