package transform

import (
	"context"
	"damask/server/internal/storage"
	"io"
	"log/slog"
	"path/filepath"
)

type Thumbnailer interface {
	GenerateThumbnailData(ctx context.Context, storage storage.Storage, mimeType, storageKey string) (data []byte, ext string, err error)
}

func NewThumbnailer(transformer Transformer) Thumbnailer {
	return &thumbnailer{transformer}
}

type thumbnailer struct {
	transformer Transformer
}

// generateThumbnailData produces thumbnail bytes and a file extension for any
// supported MIME type. It returns (nil, "", nil) for types that are unsupported
// or skipped (e.g. video when ffmpeg is unavailable) — callers should no-op.
func (t *thumbnailer) GenerateThumbnailData(ctx context.Context, storage storage.Storage, mimeType, storageKey string) (data []byte, ext string, err error) {
	switch {
	case mimeType == "image/gif":
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		if !t.transformer.FFmpegAvailable() {
			slog.Debug("thumbnail: ffmpeg not available, falling back to static thumbnail for gif", "storage_key", storageKey)
			return t.ThumbnailFromImage(rc)
		}
		return t.ThumbnailFromVideo(ctx, rc, mimeType)

	case IsImageMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return t.ThumbnailFromImage(rc)

	case IsVideoMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		if !t.transformer.FFmpegAvailable() {
			slog.Debug("thumbnail: ffmpeg not available, skipping video", "storage_key", storageKey)
			return nil, "", nil
		}
		return t.ThumbnailFromVideo(ctx, rc, mimeType)

	case IsPdfMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return t.ThumbnailFromPDF(ctx, rc, mimeType)

	case IsAudioMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return t.ThumbnailFromAudio(ctx, rc, mimeType)

	case IsDocumentMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		if !t.transformer.LibreOfficeAvailable() {
			slog.Debug("thumbnail: soffice not available, skipping document", "storage_key", storageKey)
			return nil, "", nil
		}
		return t.ThumbnailFromDocument(ctx, rc, mimeType)

	case IsTextMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return t.ThumbnailFromText(ctx, rc, mimeType)

	case IsFontMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return t.ThumbnailFromFontFile(ctx, rc, mimeType, storageKey)

	default:
		slog.Debug("thumbnail: unsupported MIME type, skipping", "mime_type", mimeType)
		return nil, "", nil
	}
}

func (t *thumbnailer) ThumbnailFromImage(rc io.ReadCloser) ([]byte, string, error) {
	data, _, err := t.transformer.ImageResize(rc, ResizeParams{
		Width:   400,
		Height:  400,
		Fit:     "contain",
		Quality: 75,
		Format:  "jpeg",
	})
	if err != nil {
		return nil, "", err
	}
	return data, ".jpg", nil
}

func (t *thumbnailer) ThumbnailFromText(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	text := make([]byte, 4096) // cap thumbnail text at 4KB
	n, err := rc.Read(text)
	if err != nil && err != io.EOF {
		return nil, "", err
	}
	text = text[:n]
	data, err := t.transformer.GenerateImageOfText(ctx, ImageOfTextOptions{TextContent: string(text)})
	if err != nil {
		return nil, "", err
	}
	return data, ".png", nil
}

func (t *thumbnailer) ThumbnailFromFontFile(ctx context.Context, rc io.ReadCloser, mimeType string, fileName string) ([]byte, string, error) {
	text := filepath.Base(fileName) + "\n\nThe quick brown fox jumps over the lazy dog."
	data, err := t.transformer.GenerateImageOfText(ctx, ImageOfTextOptions{
		TextContent: string(text),
		FontFile:    rc,
	})
	if err != nil {
		return nil, "", err
	}
	return data, ".png", nil
}

func (t *thumbnailer) ThumbnailFromVideo(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	srcExt := mimeToExt(mimeType)
	tmpPath, cleanup, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	data, err := t.transformer.VideoClipThumbnail(ctx, tmpPath, VideoClipParams{})
	if err != nil {
		return nil, "", err
	}
	return data, ".mp4", nil
}

func (t *thumbnailer) ThumbnailFromPDF(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	if !t.transformer.ImageMagickAvailable() || !t.transformer.FFmpegAvailable() {
		slog.Debug("thumbnail: convert or ffmpeg not available, skipping PDF slideshow")
		return nil, "", nil
	}
	data, err := t.transformer.PDFSlideshowThumbnail(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	return data, ".mp4", nil
}

func (t *thumbnailer) ThumbnailFromDocument(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, err := t.transformer.DocumentThumbnail(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	return data, ".png", nil
}

func (t *thumbnailer) ThumbnailFromAudio(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := t.transformer.AudioWaveform(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		slog.Warn("thumbnail: empty waveform for audio", "mime_type", mimeType)
		return nil, "", nil
	}
	return data, mimeToExt(contentType), nil
}
