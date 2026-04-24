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

func TestAssetService_Get_NotFound(t *testing.T) {
	svc := service.NewAssetService(memory.NewAssetRepo())
	_, err := svc.Get(context.Background(), "ws_1", "nonexistent")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestAssetService_Get_WrongWorkspace(t *testing.T) {
	repo := memory.NewAssetRepo()
	repo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})
	svc := service.NewAssetService(repo)
	_, err := svc.Get(context.Background(), "ws_B", "ast_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for wrong workspace, got %v", err)
	}
}

func TestAssetService_Get_OK(t *testing.T) {
	repo := memory.NewAssetRepo()
	repo.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "hero.jpg",
		MimeType:         "image/jpeg",
	})
	svc := service.NewAssetService(repo)
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
