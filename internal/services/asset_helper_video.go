package services

import (
	"context"
	"log/slog"
	"strings"

	"damask/server/internal/transform"
)

type VideoHandler struct{}

func (h VideoHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "video/")
}

func (h VideoHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	res, err := transform.VideoExtractResolution(ctx, filePath)

	var width, height *int64
	if err == nil {
		width = &res.Width
		height = &res.Height
	} else {
		slog.Warn("video meta extraction failed", "error", err)
	}

	return FileMeta{
		Width:  width,
		Height: height,
	}, nil
}
