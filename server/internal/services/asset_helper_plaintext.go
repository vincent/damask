package services

import (
	"context"
	"strings"
)

type PlainTextHandler struct{}

func (h PlainTextHandler) Supports(mime string) bool {
	return strings.HasPrefix(mime, "text/")
}

func (h PlainTextHandler) ExtractMeta(ctx context.Context, filePath string) (FileMeta, error) {
	return FileMeta{}, nil
}
