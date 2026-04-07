package services

import (
	"context"
	"strings"
)

type AudioHandler struct{}

func (h AudioHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "audio/")
}

func (h AudioHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	// Placeholder: integrate ffprobe or similar for duration/bitrate
	return FileMeta{}, nil
}

