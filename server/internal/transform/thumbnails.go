package transform

import (
	"context"
	"damask/server/internal/storage"
	"io"
	"log"
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
		if !FFmpegAvailable() {
			log.Printf("thumbnail: ffmpeg not available, skipping video %q", storageKey)
			return nil, "", nil
		}
		rc, err := storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
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

	default:
		log.Printf("thumbnail: unsupported MIME type %q, skipping", mimeType)
		return nil, "", nil
	}
}

func ThumbnailFromImage(rc io.ReadCloser) ([]byte, string, error) {
	data, _, err := Resize(rc, ResizeParams{
		Width:   400,
		Height:  400,
		Fit:     "contain",
		Quality: 85,
		Format:  "jpeg",
	})
	if err != nil {
		return nil, "", err
	}
	return data, ".jpg", nil
}

func ThumbnailFromVideo(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	srcExt := mimeToExt(mimeType)
	tmpPath, cleanup, err := writeToTempFile(ctx, rc, srcExt)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	data, err := ExtractVideoThumbnail(ctx, tmpPath, VideoThumbnailParams{Timestamp: 1.0})
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
