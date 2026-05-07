package jobs

import (
	"context"
	"database/sql"
	"fmt"
	"io"
	"mime"
	"os"
	"time"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/ingress"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
)

// JobServer holds shared dependencies injected at startup.
type JobServer struct {
	db       *dbgen.Queries
	sqlDB    *sql.DB
	storage  storage.Storage
	queue    queue.JobQueue
	mailer   mail.Mailer
	hub      events.EventHub
	cfg      *config.Config
	audit    *audit.EventWriter
	handlers map[string]queue.HandlerFunc
	trf      transform.Transformer
	tmb      transform.Thumbnailer
	injestor assetio.Injestor
}

func NewJobServer(
	db *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	hub events.EventHub,
	q queue.JobQueue,
	mailer mail.Mailer,
	trf transform.Transformer,
	tmb transform.Thumbnailer,
	cfg *config.Config,
	injestor assetio.Injestor,
) *JobServer {
	return &JobServer{
		db:       db,
		sqlDB:    sqlDB,
		storage:  stor,
		queue:    q,
		mailer:   mailer,
		hub:      hub,
		cfg:      cfg,
		trf:      trf,
		tmb:      tmb,
		audit:    audit.New(sqlDB),
		handlers: make(map[string]queue.HandlerFunc),
		injestor: injestor,
	}
}

// DrainForTest registers all handlers and synchronously processes every pending
// job. Intended for use in tests only — not safe for concurrent use.
func (s *JobServer) DrainForTest(ctx context.Context) {
	s.RegisterJobHandlers()
	for {
		job, err := s.db.ClaimNextJob(ctx)
		if err != nil {
			return // queue empty
		}
		h, ok := s.handlers[job.Type]
		if !ok {
			continue
		}
		_ = h(ctx, job)
	}
}

// RegisterJobHandlers wires all job handlers into the queue.
// It does not start any scheduler goroutines — call StartSchedulers for that.
func (s *JobServer) RegisterJobHandlers() {
	reg := func(jobType string, h queue.HandlerFunc) {
		s.queue.Register(jobType, h)
		s.handlers[jobType] = h
	}

	// Register ingress job handlers
	ingressWorker := ingress.NewWorker(s.db, s.sqlDB, s.storage, s.queue, s.cfg, s.audit, s.mailer, s.injestor)
	reg(queue.JobTypeIngestPoll, ingressWorker.HandlePoll)
	reg(queue.JobTypeIngestFetch, ingressWorker.HandleFetch)

	reg(queue.JobTypeVersionThumbnail, s.jobVersionThumbnail)
	reg(queue.JobTypeVariantThumbnail, s.jobVariantThumbnail)

	// Variant jobs — user-triggered, each creates a variants row.
	reg(queue.JobTypeVideoCaptureImage, s.wrapVariantJob(s.jobVideoCaptureImage))
	reg(queue.JobTypeVideoTranscode, s.wrapVariantJob(s.jobVideoTranscode))
	reg(queue.JobTypeVideoWatermark, s.wrapVariantJob(s.jobVideoWatermark))
	reg(queue.JobTypeImageResize, s.wrapVariantJob(s.jobImageTransform))
	reg(queue.JobTypeImageConvert, s.wrapVariantJob(s.jobImageTransform))
	reg(queue.JobTypeImageCrop, s.wrapVariantJob(s.jobImageTransform))
	reg(queue.JobTypeImageWatermark, s.wrapVariantJob(s.jobImageTransform))
	reg(queue.JobTypeImageSmartCrop, s.wrapVariantJob(s.jobImageTransform))
	reg(queue.JobTypeImageBgRemove, s.wrapVariantJob(s.jobImageBgRemove))
	reg(queue.JobTypeImageWithPrompt, s.wrapVariantJob(s.jobImageWithPrompt))
	reg(queue.JobTypeExtractAudio, s.wrapVariantJob(s.jobAudioTransform))
	reg(queue.JobTypeTranscodeAudio, s.wrapVariantJob(s.jobAudioTransform))
	reg(queue.JobTypeNormalizeAudio, s.wrapVariantJob(s.jobAudioTransform))

	// Rebuild jobs — system-triggered on version upload.
	reg(queue.JobTypeRebuildVariants, s.jobRebuildVariants)

	// EXIF extraction.
	reg(queue.JobTypeExtractExif, s.jobExtractExif)

	// Stack merge jobs.
	reg(queue.JobTypeStackMerge, s.jobStackMerge)

	// Maintenance jobs.
	reg(queue.JobTypePurgeDeletedFields, s.jobPurgeDeletedFields)
	reg(queue.JobTypeEnforceVersionRetention, s.jobEnforceVersionRetention)
	reg(queue.JobTypePurgeVersionStorage, s.jobPurgeVersionStorage)
	reg(queue.JobTypePurgeAuditLog, s.jobPurgeAuditLog)
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
