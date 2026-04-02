package services

import (
	"context"
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

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

// -- Types

type FileMeta struct {
	MimeType string
	Size     int64
	Width    *int64
	Height   *int64
}

type variantJobPayload struct {
	AssetID     string          `json:"asset_id"`
	WorkspaceID string          `json:"workspace_id"`
	StorageKey  string          `json:"storage_key"`
	MimeType    string          `json:"mime_type"`
	Type        string          `json:"type"`
	Params      json.RawMessage `json:"params"`
}

type thumbnailJobPayload struct {
	AssetID     string `json:"asset_id"`
	WorkspaceID string `json:"workspace_id"`
	StorageKey  string `json:"storage_key"`
}

// -- Media Handler Interface

type MediaHandler interface {
	Supports(mime string) bool
	ExtractMeta(ctx context.Context, filePath string) (FileMeta, error)
	EnqueueJobs(ctx context.Context, qu *queue.Queue, asset dbgen.Asset) error
}

// -- Handler Registry

var handlers = []MediaHandler{
	ImageHandler{},
	VideoHandler{},
	AudioHandler{},
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

func detectMimeType(filePath string) (string, error) {
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

func storeFile(storage storage.Storage, key string, filePath string) error {
	f, err := os.Open(filePath)
	if err != nil {
		return err
	}
	defer f.Close()

	return storage.Put(key, f)
}

// -- Main Service

func CreateAsset(
	ctx context.Context,
	db *dbgen.Queries,
	storage storage.Storage,
	qu *queue.Queue,
	workspaceID string,
	filePath string,
) (*dbgen.Asset, *fiber.Error) {

	stat, err := os.Stat(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusInternalServerError, "could not stat uploaded file")
	}

	mimeType, err := detectMimeType(filePath)
	if err != nil {
		return nil, fiber.NewError(fiber.StatusBadRequest, "could not detect MIME type")
	}

	assetID := uuid.New().String()
	originalFilename := filepath.Base(filePath)
	storageKey := fmt.Sprintf("%s/%s/%s", workspaceID, assetID, originalFilename)

	// Store file
	if err := storeFile(storage, storageKey, filePath); err != nil {
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

	// Enqueue jobs
	if handler != nil {
		if err := handler.EnqueueJobs(ctx, qu, asset); err != nil {
			log.Printf("enqueue failed for %s: %v", asset.ID, err)
		}
	}

	return &asset, nil
}
