package services

import (
	"context"
	"image"
	_ "image/gif"
	_ "image/jpeg"
	_ "image/png"
	"os"
	"strings"
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

