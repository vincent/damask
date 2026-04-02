package services

import (
	"context"
	"encoding/json"
	"log"
	"strings"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/transform"
)

type VideoHandler struct{}

func (h VideoHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

func (h VideoHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	res, err := transform.ExtractVideoResolution(ctx, filePath)

	var width, height *int64
	if err == nil {
		width = &res.Width
		height = &res.Height
	} else {
		log.Printf("video meta extraction failed: %v", err)
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}

func (h VideoHandler) EnqueueJobs(ctx context.Context, qu *queue.Queue, asset dbgen.Asset) error {
	params, _ := json.Marshal(transform.VideoThumbnailParams{Timestamp: 1.0})

	payload, _ := json.Marshal(variantJobPayload{
		AssetID:     asset.ID,
		WorkspaceID: asset.WorkspaceID,
		StorageKey:  asset.StorageKey,
		Params:      params,
	})

	_, err := qu.Enqueue(ctx, asset.WorkspaceID, queue.JobTypeVideoThumbnail, string(payload))
	return err
}
