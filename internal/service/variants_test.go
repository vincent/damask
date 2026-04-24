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

func newVariantSvc(t *testing.T) (service.VariantService, *memory.RealVariantRepo, *memory.AssetRepo) {
	t.Helper()
	varRepo := memory.NewRealVariantRepo()
	assetRepo := memory.NewAssetRepo()
	return service.NewVariantService(varRepo, assetRepo), varRepo, assetRepo
}

// --- List ---

func TestVariantService_List_Empty(t *testing.T) {
	svc, _, assetRepo := newVariantSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})
	out, err := svc.List(context.Background(), "ws_1", "ast_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(out) != 0 {
		t.Errorf("expected empty list, got %d", len(out))
	}
}

// --- Get ---

func TestVariantService_Get_NotFound(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	_, err := svc.Get(context.Background(), "ws_1", "nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Get_OK(t *testing.T) {
	svc, varRepo, _ := newVariantSvc(t)
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "v1",
		Type:           "image_resize",
		StorageKey:     "variants/var_1.jpg",
	})
	dto, err := svc.Get(context.Background(), "ws_1", "var_1")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if dto.Type != "image_resize" {
		t.Errorf("Type: got %q, want %q", dto.Type, "image_resize")
	}
}

// --- Delete ---

func TestVariantService_Delete_AssetNotFound(t *testing.T) {
	svc, _, _ := newVariantSvc(t)
	err := svc.Delete(context.Background(), "ws_1", "ast_nope", "var_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Delete_VariantNotFound(t *testing.T) {
	svc, _, assetRepo := newVariantSvc(t)
	currentVID := "v1"
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_nope")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound, got %v", err)
	}
}

func TestVariantService_Delete_OldVersion(t *testing.T) {
	svc, varRepo, assetRepo := newVariantSvc(t)
	currentVID := "v_current"
	assetRepo.Seed(repository.Asset{
		ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID,
	})
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: "v_old", // not the current version
		Type:           "image_resize",
	})
	err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_1")
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput (old version), got %v", err)
	}
}

func TestVariantService_Delete_OK(t *testing.T) {
	svc, varRepo, assetRepo := newVariantSvc(t)
	currentVID := "v1"
	assetRepo.Seed(repository.Asset{
		ID: "ast_1", WorkspaceID: "ws_1", CurrentVersionID: &currentVID,
	})
	varRepo.Seed(repository.Variant{
		ID:             "var_1",
		WorkspaceID:    "ws_1",
		AssetVersionID: currentVID,
		Type:           "image_resize",
	})
	if err := svc.Delete(context.Background(), "ws_1", "ast_1", "var_1"); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	_, err := svc.Get(context.Background(), "ws_1", "var_1")
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound after delete, got %v", err)
	}
}
