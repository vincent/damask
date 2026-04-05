package services

import (
	"context"
	"encoding/json"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

type ImageHandler struct{}

func (h ImageHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "image/")
}

func (h ImageHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	f, err := os.Open(filePath)
	if err != nil {
		return FileMeta{}, err
	}
	defer f.Close()

	cfg, _, err := image.DecodeConfig(f)

	var width, height *int64
	if err == nil {
		w, h := int64(cfg.Width), int64(cfg.Height)
		width, height = &w, &h
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}

func (h ImageHandler) EnqueueJobs(ctx context.Context, qu *queue.Queue, asset dbgen.Asset) error {
	payload, _ := json.Marshal(thumbnailJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  asset.StorageKey,
		MimeType:    asset.MimeType,
	})

	_, err := qu.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeAssetThumbnail, string(payload))
	return err
}
