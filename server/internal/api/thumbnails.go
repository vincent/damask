package api

import (
	"context"
	"io"
	"log"
	"path/filepath"

	"damask/server/internal/transform"
)

// generateThumbnailData produces thumbnail bytes and a file extension for any
// supported MIME type. It returns (nil, "", nil) for types that are unsupported
// or skipped (e.g. video when ffmpeg is unavailable) — callers should no-op.
func (s *Server) generateThumbnailData(ctx context.Context, mimeType, storageKey string) (data []byte, ext string, err error) {
	switch {
	case isImageMime(mimeType):
		rc, err := s.storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return s.thumbnailFromImage(rc)

	case isVideoMime(mimeType):
		return s.thumbnailFromVideo(ctx, storageKey)

	case isPdfMime(mimeType):
		rc, err := s.storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return s.thumbnailFromPDF(ctx, rc, mimeType)

	case isAudioMime(mimeType):
		rc, err := s.storage.Get(storageKey)
		if err != nil {
			return nil, "", err
		}
		defer rc.Close()
		return s.thumbnailFromAudio(ctx, rc, mimeType)

	default:
		log.Printf("thumbnail: unsupported MIME type %q, skipping", mimeType)
		return nil, "", nil
	}
}

func (s *Server) thumbnailFromImage(rc io.ReadCloser) ([]byte, string, error) {
	data, _, err := transform.Resize(rc, transform.ResizeParams{
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

func (s *Server) thumbnailFromVideo(ctx context.Context, storageKey string) ([]byte, string, error) {
	if !transform.FFmpegAvailable() {
		log.Printf("thumbnail: ffmpeg not available, skipping video %q", storageKey)
		return nil, "", nil
	}
	srcExt := filepath.Ext(storageKey)
	tmpPath, cleanup, err := s.writeToTempFile(ctx, storageKey, srcExt)
	if err != nil {
		return nil, "", err
	}
	defer cleanup()
	data, err := transform.ExtractVideoThumbnail(ctx, tmpPath, transform.VideoThumbnailParams{Timestamp: 1.0})
	if err != nil {
		return nil, "", err
	}
	return data, ".jpg", nil
}

func (s *Server) thumbnailFromPDF(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := transform.MagikFirstThumbnail(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	return data, mimeToExt(contentType), nil
}

func (s *Server) thumbnailFromAudio(ctx context.Context, rc io.ReadCloser, mimeType string) ([]byte, string, error) {
	data, contentType, err := transform.AudioWaveform(ctx, rc, mimeType)
	if err != nil {
		return nil, "", err
	}
	if len(data) == 0 {
		log.Printf("thumbnail: empty waveform for audio %q", mimeType)
		return nil, "", nil
	}
	return data, mimeToExt(contentType), nil
}
