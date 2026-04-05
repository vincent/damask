package api

import (
	"context"
	"fmt"
	"io"
	"mime"
	"os"
	"time"

	"damask/server/internal/queue"
)

// RegisterJobHandlers wires transform job handlers into the queue.
func (s *Server) RegisterJobHandlers() {
	// Thumbnail — 2 unified handlers (one per context).
	s.queue.Register(queue.JobTypeAssetThumbnail, s.jobAssetThumbnail)
	s.queue.Register(queue.JobTypeVersionThumbnail, s.jobVersionThumbnail)

	// Variant jobs — user-triggered, each creates a variants row.
	s.queue.Register(queue.JobTypeVideoCaptureImage, s.jobVideoCaptureImage)
	s.queue.Register(queue.JobTypeVideoTranscode, s.jobVideoTranscode)
	s.queue.Register(queue.JobTypeImageResize, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageConvert, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageCrop, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageWatermark, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageSmartCrop, s.jobImageTransform)
	s.queue.Register(queue.JobTypeImageBgRemove, s.jobImageBgRemove)

	// Maintenance jobs.
	s.queue.Register(queue.JobTypePurgeDeletedFields, s.jobPurgeDeletedFields)
	s.queue.Register(queue.JobTypeEnforceVersionRetention, s.jobEnforceVersionRetention)
	s.queue.Register(queue.JobTypePurgeVersionStorage, s.jobPurgeVersionStorage)
}

// ---- OS helpers ----

func (s *Server) writeToTempFile(ctx context.Context, storageKey, ext string) (string, func(), error) {
	rc, err := s.storage.Get(storageKey)
	if err != nil {
		return "", nil, err
	}
	defer rc.Close()

	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	if _, copyErr := io.Copy(f, rc); copyErr != nil {
		_ = f.Close()
		_ = os.Remove(f.Name())
		return "", nil, fmt.Errorf("copy to temp: %w", copyErr)
	}
	err = f.Close()
	if err != nil {
		return "", nil, fmt.Errorf("close temp: %w", err)
	}
	return f.Name(), func() { _ = os.Remove(f.Name()) }, nil
}

// ---- Helpers ----

func mimeToExt(ct string) string {
	ms, err := mime.ExtensionsByType(ct)
	if err == nil && len(ms) > 0 {
		return ms[0]
	}
	return "application/octet-stream"
}

// nextDaily returns the next occurrence of hour:min UTC on or after now.
func nextDaily(hour, min int) time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
