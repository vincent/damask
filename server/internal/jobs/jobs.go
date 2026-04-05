package jobs

import (
	"context"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/ingress"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"database/sql"
	"fmt"
	"io"
	"log"
	"mime"
	"os"
	"time"
)

// JobServer holds shared dependencies injected at startup.
type JobServer struct {
	db         *dbgen.Queries
	sqlDB      *sql.DB
	tokenMaker *auth.Maker
	storage    storage.Storage
	queue      *queue.Queue
	hub        events.EventHub
	cfg        *config.Config
}

func NewJobServer(
	db *dbgen.Queries,
	sqlDB *sql.DB,
	tokenMaker *auth.Maker,
	stor storage.Storage,
	hub events.EventHub,
	q *queue.Queue,
	cfg *config.Config,
) *JobServer {
	return &JobServer{
		db:         db,
		sqlDB:      sqlDB,
		tokenMaker: tokenMaker,
		storage:    stor,
		queue:      q,
		hub:        hub,
		cfg:        cfg,
	}
}

// RegisterJobHandlers wires transform job handlers into the queue.
func (s *JobServer) RegisterJobHandlers() {

	// Register ingress job handlers
	ingressWorker := ingress.NewWorker(s.db, s.storage, s.queue, s.cfg)
	s.queue.Register(queue.JobTypeIngestPoll, ingressWorker.HandlePoll)
	s.queue.Register(queue.JobTypeIngestFetch, ingressWorker.HandleFetch)

	// Start ingress scheduler (disabled in tests via ENABLE_SCHEDULER=false)
	if s.cfg.EnableScheduler {
		scheduler := ingress.NewScheduler(s.db, s.queue)
		scheduler.Start(context.Background())
		log.Printf("ingress scheduler started")

		fieldCleanup := NewFieldCleanupScheduler(s.db, s.queue)
		fieldCleanup.Start(context.Background())
		log.Printf("field cleanup scheduler started")

		retentionSched := NewRetentionScheduler(s.queue)
		retentionSched.Start(context.Background())
		log.Printf("retention scheduler started")
	}

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

func writeToTempFile(ctx context.Context, src io.Reader, ext string) (string, func(), error) {
	f, err := os.CreateTemp("", "damask-*"+ext)
	if err != nil {
		return "", nil, fmt.Errorf("create temp: %w", err)
	}
	if _, copyErr := io.Copy(f, src); copyErr != nil {
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

// ParseSQLiteTime parses a timestamp string as produced by SQLite's datetime('now').
// It accepts both the SQLite default format ("2006-01-02 15:04:05") and RFC3339.
func ParseSQLiteTime(s string) (time.Time, error) {
	if t, err := time.Parse("2006-01-02 15:04:05", s); err == nil {
		return t, nil
	}
	if t, err := time.Parse(time.RFC3339, s); err == nil {
		return t, nil
	}
	return time.Time{}, fmt.Errorf("unrecognised time format: %q", s)
}

func MimeToExt(ct string) string {
	ms, err := mime.ExtensionsByType(ct)
	if err == nil && len(ms) > 0 {
		return ms[0]
	}
	return "application/octet-stream"
}

// NextDaily returns the next occurrence of hour:min UTC on or after now.
func NextDaily(hour, min int) time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, min, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
