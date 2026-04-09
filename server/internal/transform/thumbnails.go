package transform

import (
	"context"
	"damask/server/internal/storage"
	"io"
	"log"
	"path/filepath"
)

// generateThumbnailData produces thumbnail bytes and a file extension for any
// supported MIME type. It returns (nil, "", nil) for types that are unsupported
// or skipped (e.g. video when ffmpeg is unavailable) — callers should no-op.
func GenerateThumbnailData(ctx context.Context, storage storage.Storage, mimeType, storageKey string) (data []byte, ext string, err error) {
	switch {
	case isImageMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromImage(rc)

	case isVideoMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		if !FFmpegAvailable() {
			log.Printf("thumbnail: ffmpeg not available, skipping video %q", storageKey)
			return nil, "", nil
		}
		return ThumbnailFromVideo(ctx, rc, mimeType)

	case isPdfMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromPDF(ctx, rc, mimeType)

	case isAudioMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromAudio(ctx, rc, mimeType)

	case isTextMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromText(ctx, rc, mimeType)

	case isFontMime(mimeType):
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return ThumbnailFromFontFile(ctx, rc, mimeType, storageKey)

	default:
		log.Printf("thumbnail: unsupported MIME type %q, skipping", mimeType)
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
	data, err := VideoExtractThumbnail(ctx, tmpPath, VideoThumbnailParams{Timestamp: 1.0})
	if err != nil {
		return nil, "", err
	}
	return data, ".jpg", nil
}

func ThumbnailFromPDF(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := MagikFirstThumbnail(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	return data, mimeToExt(contentType), nil
}

func ThumbnailFromAudio(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := AudioWaveform(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		log.Printf("thumbnail: empty waveform for audio %q", mimeType)
		return nil, "", nil
	}
	return data, mimeToExt(contentType), nil
}
