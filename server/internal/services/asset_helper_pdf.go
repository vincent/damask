package services

import (
	"context"
	"encoding/json"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

type PdfHandler struct{}

func (h PdfHandler) Supports(mime string) bool {
	return strings.HasSuffix(mime, "/pdf")
}

func (h PdfHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	// Placeholder: integrate ffprobe or similar for duration/bitrate
	return FileMeta{}, nil
}

func (h PdfHandler) EnqueueJobs(ctx context.Context, qu *queue.Queue, asset dbgen.Asset) error {
	payload, _ := json.Marshal(thumbnailJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  asset.StorageKey,
		MimeType:    asset.MimeType,
	})
	_, err := qu.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeAssetThumbnail, string(payload))
	return err
}
