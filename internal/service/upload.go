package service

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"os"
	"path/filepath"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/fileproc"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
)

// UploadMeta holds caller-supplied metadata for a file upload.
type UploadMeta struct {
	OriginalFilename string
	ProjectID        *string
	FolderID         *string
	UserID           string
	// InheritFields is called after asset creation to copy project field values.
	// May be nil.
	InheritFields fileproc.FieldInheritanceFunc
}

// UploadedAssetDTO is the result of UploadService.Ingest.
type UploadedAssetDTO struct {
	ID               string
	WorkspaceID      string
	OriginalFilename string
	StorageKey       string
	MimeType         string
	Size             int64
}

type uploadServiceImpl struct {
	db      *dbgen.Queries
	sqlDB   *sql.DB
	storage storage.Storage
	q       queue.JobQueue
}

// NewUploadService returns an UploadService.
func NewUploadService(db *dbgen.Queries, sqlDB *sql.DB, stor storage.Storage, q queue.JobQueue) UploadService {
	return &uploadServiceImpl{db: db, sqlDB: sqlDB, storage: stor, q: q}
}

// Ingest writes r to a temp file, calls fileproc.CreateAsset, then removes the temp file.
// Queue enqueue failures are logged but do not fail the upload (fire-and-forget).
func (s *uploadServiceImpl) Ingest(ctx context.Context, workspaceID string, r io.Reader, meta UploadMeta) (*UploadedAssetDTO, error) {
	if workspaceID == "" {
		return nil, fmt.Errorf("workspaceID is required: %w", apperr.ErrInvalidInput)
	}
	if meta.OriginalFilename == "" {
		return nil, fmt.Errorf("filename is required: %w", apperr.ErrInvalidInput)
	}

	// Write reader to a temp file so fileproc can stat + sniff MIME.
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

	// fileproc.CreateAsset handles: stat, MIME detection, storage.Put, DB writes,
	// version creation, field inheritance, thumbnail + EXIF job enqueueing.
	asset, fErr := fileproc.CreateAsset(ctx, s.db, s.sqlDB, s.storage, s.q, workspaceID, tmpPath, fileproc.AssetOptions{
		ProjectID:     meta.ProjectID,
		FolderID:      meta.FolderID,
		UserID:        meta.UserID,
		InheritFields: meta.InheritFields,
		OriginalName:  meta.OriginalFilename,
	})
	if fErr != nil {
		return nil, fmt.Errorf("%s: %w", fErr.Message, apperr.ErrInvalidInput)
	}

	return &UploadedAssetDTO{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		OriginalFilename: asset.OriginalFilename,
		StorageKey:       asset.StorageKey,
		MimeType:         asset.MimeType,
		Size:             asset.Size,
	}, nil
}
