package service

import (
	"context"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"damask/server/internal/apperr"
	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
)

// UploadMeta holds caller-supplied metadata for a file upload.
type UploadMeta struct {
	OriginalFilename string
	ProjectID        *string
	FolderID         *string
	UserID           string
	// InheritFields is called after asset creation to copy project field values.
	// May be nil.
	InheritFields assetio.FieldInheritanceFunc
}

type uploadServiceImpl struct {
	injestor AssetInjestor
	audit    audit.Writer
}

// NewUploadService returns an UploadService.
func NewUploadService(injestor AssetInjestor, aw audit.Writer) UploadService {
	return &uploadServiceImpl{injestor: injestor, audit: aw}
}

// Ingest writes r to a temp file, calls AssetInjestor.IngestFileFull, then removes the temp file.
// Queue enqueue failures are logged but do not fail the upload (fire-and-forget).
func (s *uploadServiceImpl) Ingest(ctx context.Context, workspaceID string, r io.Reader, meta UploadMeta) (*AssetDTO, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID is required: %w", apperr.ErrInvalidInput)
	}
	if meta.OriginalFilename == "" {
		return nil, fmt.Errorf("filename is required: %w", apperr.ErrInvalidInput)
	}

	tmpF, err := os.CreateTemp("", "damask-upload-*"+filepath.Ext(meta.OriginalFilename))
	if err != nil {
		return nil, fmt.Errorf("cannot create temp file: %w", err)
	}
	tmpPath := tmpF.Name()
	defer os.Remove(tmpPath)

	if _, err := io.Copy(tmpF, r); err != nil {
		_ = tmpF.Close()
		return nil, fmt.Errorf("cannot write temp file: %w", err)
	}
	_ = tmpF.Close()

	asset, err := s.injestor.IngestFileWithDetails(ctx, workspaceID, tmpPath, assetio.IngestFileOpts{
		ProjectID:     meta.ProjectID,
		FolderID:      meta.FolderID,
		UserID:        meta.UserID,
		InheritFields: meta.InheritFields,
		OriginalName:  meta.OriginalFilename,
	})
	if err != nil {
		return nil, err
	}

	actor := auth.ActorFromCtx(ctx)
	s.audit.WriteAsset(ctx, audit.AssetEvent{
		WorkspaceID: workspaceID,
		AssetID:     asset.ID,
		UserID:      actor.UserID,
		ActorType:   actor.Type,
		EventType:   audit.EventAssetCreated,
		Payload:     audit.AssetCreatedPayload{V: 1, Filename: asset.OriginalFilename, Source: "upload"},
	})
	return asset, nil
}
