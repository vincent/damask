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
	apptelemetry "damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/versioning"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
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
func (s *injestorImpl) ingest(ctx context.Context, workspaceID, filePath string, opts assetio.IngestFileOpts) (asset dbgen.Asset, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.injestor.ingest",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.Bool("damask.upload.has_project", opts.ProjectID != nil),
		attribute.Bool("damask.upload.has_folder", opts.FolderID != nil),
	)
	defer func() {
		if asset.ID != "" {
			span.SetAttributes(
				attribute.String("damask.asset_id", asset.ID),
				attribute.String("damask.mime_type", asset.MimeType),
				attribute.Int64("damask.asset.size", asset.Size),
			)
		}
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "asset ingest failed", "workspace_id", workspaceID, "error", err)
		}
	}()

	slog.DebugContext(ctx, "starting asset ingest", "workspace_id", workspaceID, "file_path", filePath, "opts", opts)

	stat, err := os.Stat(filePath)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not stat uploaded file: %w", err)
	}
	span.SetAttributes(attribute.Int64("damask.upload.bytes", stat.Size()))

	mimeType, err := transform.DetectMimeType(filePath)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not detect MIME type: %w", err)
	}
	span.SetAttributes(attribute.String("damask.mime_type", mimeType))

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
	_, storeSpan := apptelemetry.StartSpan(ctx, "service.injestor.storage_put",
		attribute.String("damask.storage.key", storageKey),
		attribute.Int64("damask.upload.bytes", stat.Size()),
	)
	err = s.stor.Put(storageKey, f)
	apptelemetry.EndSpan(storeSpan, err)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not store file: %w", err)
	}

	meta := mediatype.FileMeta{}
	if s.media.Supports(mimeType) {
		metaCtx, metaSpan := apptelemetry.StartSpan(ctx, "service.injestor.extract_metadata",
			attribute.String("damask.mime_type", mimeType),
		)
		if m, merr := s.media.ExtractMeta(ctx, filePath, mimeType); merr == nil {
			meta = m
		} else {
			apptelemetry.RecordError(metaSpan, merr)
			slog.WarnContext(metaCtx, "metadata extraction failed", "mime_type", mimeType, "error", merr)
		}
		metaSpan.End()
	} else {
		slog.DebugContext(ctx, "no handler for MIME type, skipping metadata extraction", "mime_type", mimeType)
	}

	_, createSpan := apptelemetry.StartSpan(ctx, "service.injestor.create_asset")
	asset, err = s.db.CreateAsset(ctx, dbgen.CreateAssetParams{
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
	apptelemetry.EndSpan(createSpan, err)
	if err != nil {
		return dbgen.Asset{}, fmt.Errorf("could not save asset: %w", err)
	}

	slog.DebugContext(ctx, "created asset", "asset_id", asset.ID, "mime_type", asset.MimeType, "size", asset.Size)

	initialVersionID, vErr := s.createInitialVersion(ctx, asset, filePath, storageKey, mimeType, meta, opts.UserID)
	if vErr != nil {
		slog.ErrorContext(ctx, "create initial version", "asset_id", asset.ID, "error", vErr)
	}

	if opts.FolderID != nil {
		if err := s.db.UpdateAssetFolder(ctx, dbgen.UpdateAssetFolderParams{
			FolderID:    opts.FolderID,
			ID:          asset.ID,
			WorkspaceID: workspaceID,
		}); err != nil {
			slog.ErrorContext(ctx, "set folder for asset", "asset_id", asset.ID, "error", err)
		} else {
			asset.FolderID = opts.FolderID
		}
	}

	if opts.InheritFields != nil && opts.ProjectID != nil && opts.UserID != "" {
		inheritCtx, inheritSpan := apptelemetry.StartSpan(ctx, "service.injestor.inherit_project_fields",
			attribute.String("damask.asset_id", asset.ID),
			attribute.String("damask.project_id", *opts.ProjectID),
		)
		opts.InheritFields(inheritCtx, workspaceID, asset.ID, *opts.ProjectID, opts.UserID)
		inheritSpan.End()
	}

	slog.DebugContext(ctx, "asset ingest completed", "asset_id", asset.ID, "workspace_id", workspaceID, "mime_type", asset.MimeType, "size", asset.Size, "supported_media", s.media.Supports(mimeType))

	if s.media.Supports(mimeType) && initialVersionID != "" {
		payload, _ := json.Marshal(versionThumbnailPayload{
			AssetID:     asset.ID,
			VersionID:   initialVersionID,
			WorkspaceID: asset.WorkspaceID,
			StorageKey:  asset.StorageKey,
			MimeType:    asset.MimeType,
		})
		_, enqueueSpan := apptelemetry.StartSpan(ctx, "service.injestor.enqueue_thumbnail",
			attribute.String("damask.asset_id", asset.ID),
			attribute.String("damask.job.type", string(queue.JobTypeVersionThumbnail)),
		)
		_, err := s.q.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeVersionThumbnail, string(payload))
		apptelemetry.EndSpan(enqueueSpan, err)
		if err != nil {
			slog.ErrorContext(ctx, "enqueue version thumbnail", "asset_id", asset.ID, "error", err)
		}
	}

	if transform.IsImageMime(mimeType) {
		exifPayload, _ := json.Marshal(map[string]string{
			"asset_id":     asset.ID,
			"workspace_id": workspaceID,
			"user_id":      opts.UserID,
		})
		_, enqueueSpan := apptelemetry.StartSpan(ctx, "service.injestor.enqueue_exif",
			attribute.String("damask.asset_id", asset.ID),
			attribute.String("damask.job.type", string(queue.JobTypeExtractExif)),
		)
		_, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeExtractExif, string(exifPayload))
		apptelemetry.EndSpan(enqueueSpan, err)
		if err != nil {
			slog.ErrorContext(ctx, "enqueue extract_exif", "asset_id", asset.ID, "error", err)
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
) (versionID string, err error) {
	ctx, span := apptelemetry.StartSpan(ctx, "service.injestor.create_initial_version",
		attribute.String("damask.workspace_id", asset.WorkspaceID),
		attribute.String("damask.asset_id", asset.ID),
		attribute.Int64("damask.asset.size", asset.Size),
	)
	defer func() {
		apptelemetry.EndSpan(span, err)
		if err != nil {
			slog.ErrorContext(ctx, "create initial version failed", "asset_id", asset.ID, "error", err)
		}
	}()

	hash, err := versioning.HashFile(filePath)
	if err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	versionID = uuid.NewString()

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

	if _, err = qtx.CreateAssetVersion(ctx, dbgen.CreateAssetVersionParams{
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

	if err = qtx.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               asset.ID,
	}); err != nil {
		return "", fmt.Errorf("set current_version_id: %w", err)
	}

	err = tx.Commit()
	return versionID, err
}
