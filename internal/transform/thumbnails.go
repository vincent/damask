package transform

import (
	"context"
	"damask/server/internal/storage"
	"io"
	"log/slog"
	"path/filepath"
)

// generateThumbnailData produces thumbnail bytes and a file extension for any
// supported MIME type. It returns (nil, "", nil) for types that are unsupported
// or skipped (e.g. video when ffmpeg is unavailable) — callers should no-op.
func GenerateThumbnailData(ctx context.Context, storage storage.Storage, mimeType, storageKey string) (data []byte, ext string, err error) {
	switch {
	case IsImageMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromImage(rc)

	case IsVideoMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		if !FFmpegAvailable() {
			slog.Debug("thumbnail: ffmpeg not available, skipping video", "storage_key", storageKey)
			return nil, "", nil
		}
		return ThumbnailFromVideo(ctx, rc, mimeType)

	case IsPdfMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromPDF(ctx, rc, mimeType)

	case IsAudioMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromAudio(ctx, rc, mimeType)

	case IsTextMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromText(ctx, rc, mimeType)

	case IsFontMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromFontFile(ctx, rc, mimeType, storageKey)

	default:
		slog.Debug("thumbnail: unsupported MIME type, skipping", "mime_type", mimeType)
		return nil, "", nil
	}
}

func ThumbnailFromImage(rc io.ReadCloser) ([]byte, string, error) {
	data, _, err := ImageResize(rc, ResizeParams{
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

func ThumbnailFromText(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	text := make([]byte, 4096) // cap thumbnail text at 4KB
	n, err := rc.Read(text)
	if err != nil && err != io.EOF {
		return nil, "", err
	}
	text = text[:n]
	data, err := GenerateImageOfText(ctx, ImageOfTextOptions{TextContent: string(text)})
	if err != nil {
		return nil, "", err
	}
	return data, ".png", nil
}

func ThumbnailFromFontFile(ctx context.Context, rc io.ReadCloser, mimeType string, fileName string) ([]byte, string, error) {
	text := filepath.Base(fileName) + "\n\nThe quick brown fox jumps over the lazy dog."
	data, err := GenerateImageOfText(ctx, ImageOfTextOptions{
		TextContent: string(text),
		FontFile:    rc,
	})
	if err != nil {
		return nil, "", err
	}
	return data, ".png", nil
}

func ThumbnailFromVideo(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	srcExt := mimeToExt(mimeType)
	tmpPath, cleanup, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	data, err := VideoClipThumbnail(ctx, tmpPath, VideoClipParams{})
	if err != nil {
		return nil, "", err
	}
	return data, ".mp4", nil
}

func ThumbnailFromPDF(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	if !ImageMagickAvailable() || !FFmpegAvailable() {
		slog.Debug("thumbnail: convert or ffmpeg not available, skipping PDF slideshow")
		return nil, "", nil
	}
	data, err := PDFSlideshowThumbnail(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	return data, ".mp4", nil
}

func ThumbnailFromAudio(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := AudioWaveform(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		slog.Warn("thumbnail: empty waveform for audio", "mime_type", mimeType)
		return nil, "", nil
	}
	return data, mimeToExt(contentType), nil
}
