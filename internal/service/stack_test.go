package service_test

import (
	"bytes"
	"context"
	"errors"
	"strings"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/storage"
)

func newStackSvc(t *testing.T) (service.StackService, *memory.AssetRepo) {
	t.Helper()
	assetRepo := memory.NewAssetRepo()
	versionRepo := memory.NewRealVersionRepo()
	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}
	// nil queue: EnqueueMerge tests that need the queue use their own setup
	svc := service.NewStackService(assetRepo, versionRepo, stor, nil)
	return svc, assetRepo
}

// -- ExportZip --

func TestStackService_ExportZip_EmptyIDs(t *testing.T) {
	svc, _ := newStackSvc(t)
	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{}, &buf)
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestStackService_ExportZip_AssetNotInWorkspace(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_B", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
	}, &buf)
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStackService_ExportZip_OK(t *testing.T) {
	assetRepo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()

	assetRepo.Seed(repository.Asset{
		ID:               "ast_1",
		WorkspaceID:      "ws_1",
		OriginalFilename: "photo.jpg",
	})

	// Put a file in storage at a key we'll wire via the DB.
	// Since NewStackService needs dbgen.Queries for GetCurrentVersion,
	// and we don't have a real DB here, the export will log a "missing" file
	// (version not found) and produce a zip with _missing_files.txt.
	// That is the expected behavior when no version row exists.
	_ = stor
	svc := service.NewStackService(assetRepo, memory.NewRealVersionRepo(), stor, nil)

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
		Filename: "my-export",
	}, &buf)
	// Should succeed (missing file goes into _missing_files.txt, not an error)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if buf.Len() == 0 {
		t.Error("expected non-empty zip output")
	}
}

// -- MergeParams.Validate --

func TestMergeParams_Validate_EmptyIDs(t *testing.T) {
	err := service.MergeParams{OutputType: "gif"}.Validate()
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

func TestMergeParams_Validate_BadOutputType(t *testing.T) {
	err := service.MergeParams{AssetIDs: []string{"x"}, OutputType: "webm"}.Validate()
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput for bad output type, got %v", err)
	}
}

func TestMergeParams_Validate_OK(t *testing.T) {
	for _, ot := range []string{"gif", "pdf"} {
		err := service.MergeParams{AssetIDs: []string{"x"}, OutputType: ot}.Validate()
		if err != nil {
			t.Errorf("output_type %q: unexpected error: %v", ot, err)
		}
	}
}

// -- EnqueueMerge --

func TestStackService_EnqueueMerge_AssetNotInWorkspace(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_A"})

	_, err := svc.EnqueueMerge(context.Background(), "ws_B", "user_1", service.MergeParams{
		AssetIDs:   []string{"ast_1"},
		OutputType: "gif",
	})
	if !errors.Is(err, apperr.ErrForbidden) {
		t.Fatalf("expected ErrForbidden, got %v", err)
	}
}

func TestStackService_EnqueueMerge_InvalidOutputType(t *testing.T) {
	svc, assetRepo := newStackSvc(t)
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1"})

	_, err := svc.EnqueueMerge(context.Background(), "ws_1", "user_1", service.MergeParams{
		AssetIDs:   []string{"ast_1"},
		OutputType: "avi",
	})
	if !errors.Is(err, apperr.ErrInvalidInput) {
		t.Fatalf("expected ErrInvalidInput, got %v", err)
	}
}

// -- sanitiseStackFilename (tested via ExportZip Filename field) --

func TestStackService_ExportZip_FilenameDefault(t *testing.T) {
	// Passing a blank filename should not cause a panic or error (default applied internally).
	assetRepo := memory.NewAssetRepo()
	stor, _ := storage.NewAferoMemoryStorage()
	assetRepo.Seed(repository.Asset{ID: "ast_1", WorkspaceID: "ws_1", OriginalFilename: "f.jpg"})
	svc := service.NewStackService(assetRepo, memory.NewRealVersionRepo(), stor, nil)

	var buf bytes.Buffer
	err := svc.ExportZip(context.Background(), "ws_1", service.ExportZipParams{
		AssetIDs: []string{"ast_1"},
		Filename: strings.Repeat("/", 5), // all stripped → empty → default applied
	}, &buf)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
}
