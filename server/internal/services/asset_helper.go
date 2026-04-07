package services

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"path/filepath"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/versioning"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// -- Types

type FileMeta struct {
	MimeType    string
	Size        int64
	Width       *int64
	Height      *int64
	DurationSec *float64
}

type versionThumbnailPayload struct {
	AssetID     string `json:"asset_id"`
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
	MimeType    string `json:"mime_type"`
}

// -- Media Handler Interface

type MediaHandler interface {
	Supports(mime string) bool
	ExtractMeta(ctx context.Context, filePath string) (FileMeta, error)
}

// -- Handler Registry

var handlers = []MediaHandler{
	ImageHandler{},
	VideoHandler{},
	AudioHandler{},
	PdfHandler{},
}

func getHandler(mime string) MediaHandler {
	for _, h := range handlers {
		if h.Supports(mime) {
			return h
		}
	}
	return nil
}

// -- Helpers

// DetectMimeType sniffs the MIME type of the file at filePath.
func DetectMimeType(filePath string) (string, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return "", err
	}
	defer f.Close()

	sniff := make([]byte, 512)
	n, _ := f.Read(sniff)
	mimeType := http.DetectContentType(sniff[:n])

	if idx := strings.Index(mimeType, ";"); idx != -1 {
		mimeType = strings.TrimSpace(mimeType[:idx])
	}

	return mimeType, nil
}

// ExtractMeta extracts width, height, duration, etc. from a file using the
// registered media handler for the given MIME type. Returns a zero FileMeta if
// no handler is found or extraction fails.
func ExtractMeta(ctx context.Context, filePath, mimeType string) (FileMeta, error) {
	h := getHandler(mimeType)
	if h == nil {
		return FileMeta{}, nil
	}
	return h.ExtractMeta(ctx, filePath)
}

// -- Main Service

// FieldInheritanceFunc is called after asset creation to copy project field values.
// It is injected at the API layer to avoid a circular import.
type FieldInheritanceFunc func(ctx context.Context, db *dbgen.Queries, workspaceID, assetID string, projectID string, userID string)

// AssetOptions holds optional destination fields for CreateAsset.
type AssetOptions struct {
	ProjectID     *string
	FolderID      *string
	UserID        string
	InheritFields FieldInheritanceFunc
}

func CreateAsset(
	ctx context.Context,
	db *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	qu queue.JobQueue,
	workspaceID string,
	filePath string,
	opts AssetOptions,
) (*dbgen.Asset, *fiber.Error) {

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "could not stat uploaded file")
	}

	mimeType, err := DetectMimeType(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "could not detect MIME type")
	}

	assetID := uuid.New().String()
	originalFilename := filepath.Base(filePath)
	storageKey := fmt.Sprintf("%s/%s/%s", workspaceID, assetID, originalFilename)

	f, err := os.Open(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "could not open file")
	}
	defer f.Close()
	if err := stor.Put(storageKey, f); err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "could not store file")
	}

	// Extract metadata via handler
	handler := getHandler(mimeType)
	meta := FileMeta{}

	if handler != nil {
		m, err := handler.ExtractMeta(ctx, filePath)
		if err == nil {
			meta = m
		}
	} else {
		log.Printf("no handler for MIME type %s, skipping metadata extraction and job enqueueing", mimeType)
	}

	// Save asset
	asset, err := db.CreateAsset(ctx, dbgen.CreateAssetParams{
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
		return nil, fiber.NewError(fiber.StatusInternalServerError, "could not save asset")
	}

	log.Printf("created asset %s with MIME type %s", asset.ID, asset.MimeType)

	// AV-2.1: create the v1 asset_versions row and set current_version_id.
	initialVersionID, vErr := createInitialVersion(ctx, db, sqlDB, asset, filePath, storageKey, mimeType, meta, opts.UserID)
	if vErr != nil {
		log.Printf("create initial version for %s: %v", asset.ID, vErr)
		// Non-fatal: the asset row is already committed; versioning can be
		// back-filled by the data migration. Do not fail the upload.
	}

	// Assign folder if specified
	if opts.FolderID != nil {
		if err := db.UpdateAssetFolder(ctx, dbgen.UpdateAssetFolderParams{
			FolderID:    opts.FolderID,
			ID:          asset.ID,
			WorkspaceID: workspaceID,
		}); err != nil {
			log.Printf("set folder for asset %s: %v", asset.ID, err)
		} else {
			asset.FolderID = opts.FolderID
		}
	}

	// Inherit field values from the destination project (CF-3.3)
	if opts.InheritFields != nil && opts.ProjectID != nil && opts.UserID != "" {
		opts.InheritFields(ctx, db, workspaceID, asset.ID, *opts.ProjectID, opts.UserID)
	}

	// Enqueue version thumbnail job (updates both asset_versions.thumbnail_key and
	// assets.thumbnail_key once done). Only enqueue if the initial version was created.
	if handler != nil && initialVersionID != "" {
		if err := enqueueVersionThumbnail(ctx, qu, asset, initialVersionID); err != nil {
			log.Printf("enqueue version thumbnail for %s: %v", asset.ID, err)
		}
	}

	return &asset, nil
}

// createInitialVersion inserts the v1 asset_versions row and sets
// assets.current_version_id — all within a single transaction.
func createInitialVersion(
	ctx context.Context,
	db *dbgen.Queries,
	sqlDB *sql.DB,
	asset dbgen.Asset,
	filePath, storageKey, mimeType string,
	meta FileMeta,
	userID string,
) (string, error) {
	// Hash the file to populate content_hash.
	hash, err := versioning.HashFile(filePath)
	if err != nil {
		return "", fmt.Errorf("hash file: %w", err)
	}

	versionID := uuid.NewString()

	tx, err := sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return "", fmt.Errorf("begin tx: %w", err)
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := db.WithTx(tx)

	createdBy := userID
	if createdBy == "" {
		createdBy = asset.WorkspaceID // fallback: workspace id as sentinel
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
		CreatedBy:   createdBy,
		IsCurrent:   1,
	}); err != nil {
		return "", fmt.Errorf("create version row: %w", err)
	}

	if err := qtx.UpdateAssetCurrentVersion(ctx, dbgen.UpdateAssetCurrentVersionParams{
		CurrentVersionID: &versionID,
		ID:               asset.ID,
	}); err != nil {
		return "", fmt.Errorf("set current_version_id: %w", err)
	}

	return versionID, tx.Commit()
}

// enqueueVersionThumbnail enqueues a version_thumbnail job for the given asset and version.
func enqueueVersionThumbnail(ctx context.Context, qu queue.JobQueue, asset dbgen.Asset, versionID string) error {
	payload, _ := json.Marshal(versionThumbnailPayload{
		AssetID:     asset.ID,
		VersionID:   versionID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  asset.StorageKey,
		MimeType:    asset.MimeType,
	})
	_, err := qu.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeVersionThumbnail, string(payload))
	return err
}
