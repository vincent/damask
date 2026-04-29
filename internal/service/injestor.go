package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"os"
	"path/filepath"

	"damask/server/internal/assetio"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mediatype"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/versioning"

	"github.com/google/uuid"
)

// AssetInjestor extends assetio.Injestor with a richer return used within the service layer.
type AssetInjestor interface {
	assetio.Injestor
	IngestFileWithDetails(ctx context.Context, workspaceID, filePath string, opts assetio.IngestFileOpts) (*AssetDTO, error)
}

type versionThumbnailPayload struct {
	AssetID     string `json:"asset_id"`
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

type injestorImpl struct {
	db    *dbgen.Queries
	sqlDB *sql.DB
	stor  storage.Storage
	q     queue.JobQueue
	media *mediatype.Registry
}

// NewAssetInjestor returns an AssetInjestor backed by the given dependencies.
func NewAssetInjestor(db *dbgen.Queries, sqlDB *sql.DB, stor storage.Storage, q queue.JobQueue, media *mediatype.Registry) AssetInjestor {
	return &injestorImpl{db: db, sqlDB: sqlDB, stor: stor, q: q, media: media}
}

func (s *injestorImpl) IngestFile(ctx context.Context, workspaceID, filePath string, opts assetio.IngestFileOpts) (assetio.AssetSummary, error) {
	asset, err := s.ingest(ctx, workspaceID, filePath, opts)
	if err != nil {
		return assetio.AssetSummary{}, err
	}
	return assetio.AssetSummary{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		StorageKey:       asset.StorageKey,
		MimeType:         asset.MimeType,
		OriginalFilename: asset.OriginalFilename,
	}, nil
}

func (s *injestorImpl) IngestFileWithDetails(ctx context.Context, workspaceID, filePath string, opts assetio.IngestFileOpts) (*AssetDTO, error) {
	asset, err := s.ingest(ctx, workspaceID, filePath, opts)
	if err != nil {
		return nil, err
	}
	return &AssetDTO{
		ID:               asset.ID,
		WorkspaceID:      asset.WorkspaceID,
		ProjectID:        asset.ProjectID,
		FolderID:         asset.FolderID,
		OriginalFilename: asset.OriginalFilename,
		StorageKey:       asset.StorageKey,
		MimeType:         asset.MimeType,
		Size:             asset.Size,
		Width:            asset.Width,
		Height:           asset.Height,
		ThumbnailKey:     asset.ThumbnailKey,
		Metadata:         asset.Metadata,
		CurrentVersionID: asset.CurrentVersionID,
		CreatedAt:        asset.CreatedAt,
		UpdatedAt:        asset.UpdatedAt,
	}, nil
}

// ingest is the shared implementation called by IngestFile and IngestFileFull.
func (s *injestorImpl) ingest(ctx context.Context, workspaceID, filePath string, opts assetio.IngestFileOpts) (dbgen.Asset, error) {
	stat, err := os.Stat(filePath)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not stat uploaded file: %w", err)
	}

	mimeType, err := transform.DetectMimeType(filePath)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not detect MIME type: %w", err)
	}

	assetID := uuid.New().String()
	originalFilename := filepath.Base(filePath)
	if opts.OriginalName != "" {
		originalFilename = opts.OriginalName
	}
	storageKey := fmt.Sprintf("%s/%s/%s", workspaceID, assetID, originalFilename)

	f, err := os.Open(filePath)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not open file: %w", err)
	}
	defer f.Close()
	if err := s.stor.Put(storageKey, f); err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not store file: %w", err)
	}

	meta := mediatype.FileMeta{}
	if s.media.Supports(mimeType) {
		if m, merr := s.media.ExtractMeta(ctx, filePath, mimeType); merr == nil {
			meta = m
		}
	} else {
		slog.Debug("no handler for MIME type, skipping metadata extraction", "mime_type", mimeType)
	}

	asset, err := s.db.CreateAsset(ctx, dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		ProjectID:        opts.ProjectID,
		OriginalFilename: originalFilename,
		StorageKey:       storageKey,
		MimeType:         mimeType,
		Size:             stat.Size(),
		Width:            meta.Width,
		Height:           meta.Height,
	})
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not save asset: %w", err)
	}

	slog.Debug("created asset", "asset_id", asset.ID, "mime_type", asset.MimeType)

	initialVersionID, vErr := s.createInitialVersion(ctx, asset, filePath, storageKey, mimeType, meta, opts.UserID)
	if vErr != nil {
		slog.Error("create initial version", "asset_id", asset.ID, "error", vErr)
	}

	if opts.FolderID != nil {
		if err := s.db.UpdateAssetFolder(ctx, dbgen.UpdateAssetFolderParams{
			FolderID:    opts.FolderID,
			ID:          asset.ID,
			WorkspaceID: workspaceID,
		}); err != nil {
			slog.Error("set folder for asset", "asset_id", asset.ID, "error", err)
		} else {
			asset.FolderID = opts.FolderID
		}
	}

	if opts.InheritFields != nil && opts.ProjectID != nil && opts.UserID != "" {
		opts.InheritFields(ctx, workspaceID, asset.ID, *opts.ProjectID, opts.UserID)
	}

	if s.media.Supports(mimeType) && initialVersionID != "" {
		payload, _ := json.Marshal(versionThumbnailPayload{
			AssetID:     asset.ID,
			VersionID:   initialVersionID,
			WorkspaceID: asset.WorkspaceID,
			StorageKey:  asset.StorageKey,
			MimeType:    asset.MimeType,
		})
		if _, err := s.q.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeVersionThumbnail, string(payload)); err != nil {
			slog.Error("enqueue version thumbnail", "asset_id", asset.ID, "error", err)
		}
	}

	if transform.IsImageMime(mimeType) {
		exifPayload, _ := json.Marshal(map[string]string{
			"asset_id":     asset.ID,
			"workspace_id": workspaceID,
			"user_id":      opts.UserID,
		})
		if _, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeExtractExif, string(exifPayload)); err != nil {
			slog.Error("enqueue extract_exif", "asset_id", asset.ID, "error", err)
		}
	}

	return asset, nil
}

func (s *injestorImpl) createInitialVersion(
	ctx context.Context,
	asset dbgen.Asset,
	filePath, storageKey, mimeType string,
	meta mediatype.FileMeta,
	userID string,
) (string, error) {
	hash, err := versioning.HashFile(filePath)
	if err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	versionID := uuid.NewString()

	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback()

	qtx := s.db.WithTx(tx)

	var createdByPtr *string
	if userID != "" {
		createdByPtr = &userID
	}

	if _, err := qtx.CreateAssetVersion(ctx, dbgen.CreateAssetVersionParams{
		ID:          versionID,
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		VersionNum:  1,
		StorageKey:  storageKey,
		ContentHash: hash,
		MimeType:    mimeType,
		Size:        asset.Size,
		Width:       meta.Width,
		Height:      meta.Height,
		DurationSec: meta.DurationSec,
		CreatedBy:   createdByPtr,
		IsCurrent:   1,
	}); err != nil {
		return "", fmt.Errorf("create version row (asset_id, workspace_id, created_by) (%s, %s, %v): %w", asset.ID, asset.WorkspaceID, createdByPtr, err)
	}

	if err := qtx.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               asset.ID,
	}); err != nil {
		return "", fmt.Errorf("set current_version_id: %w", err)
	}

	return versionID, tx.Commit()
}
