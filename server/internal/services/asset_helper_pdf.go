package services

import (
	"context"
	"strings"
)

type PdfHandler struct{}

func (h PdfHandler) Supports(mime string) bool {
	return strings.HasSuffix(mime, "/pdf")
}

func (h PdfHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	// Placeholder: integrate ffprobe or similar for duration/bitrate
	return FileMeta{}, nil
}

