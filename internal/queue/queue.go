// Package queue implements the in-process job queue backed by SQLite.
package queue

import (
	"context"
	"database/sql"
	"fmt"
	"log/slog"
	"runtime/debug"
	"sync"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/telemetry"

	"github.com/google/uuid"
	"go.opentelemetry.io/otel/attribute"
	"go.opentelemetry.io/otel/codes"
	"go.opentelemetry.io/otel/trace"
)

// HandlerFunc processes a job payload and returns an error on failure.
type HandlerFunc func(ctx context.Context, job dbgen.Job) error

// JobQueue is the interface implemented by queue backends.
// The SQLite-backed Queue satisfies this interface; future backends (e.g. asynq) can too.
type JobQueue interface {
	Register(jobType string, handler HandlerFunc)
	Enqueue(ctx context.Context, workspaceID, jobType, payload string) (dbgen.Job, error)
	Start(ctx context.Context)
	Stop()
}

// Queue is an in-process job queue backed by SQLite with a configurable worker pool.
type Queue struct {
	db       *dbgen.Queries
	workers  int
	handlers map[string]HandlerFunc

	notify chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup

	// Semaphore limiting concurrent transcode jobs to 2.
	transcodeSem chan struct{}
	// Semaphore limiting concurrent rebuild_variants jobs to 2.
	rebuildSem chan struct{}
}

// New creates a new Queue. Call Start() to begin processing.
func New(db *dbgen.Queries, workers int) *Queue {
	if workers <= 0 {
		workers = 4
	}
	return &Queue{
		db:           db,
		workers:      workers,
		handlers:     make(map[string]HandlerFunc),
		notify:       make(chan struct{}, workers),
		done:         make(chan struct{}),
		transcodeSem: make(chan struct{}, 2),
		rebuildSem:   make(chan struct{}, 2),
	}
}

// Register adds a handler for the given job type.
func (q *Queue) Register(jobType string, h HandlerFunc) {
	q.handlers[jobType] = h
}

// Start re-queues any stalled jobs and launches worker goroutines.
func (q *Queue) Start(ctx context.Context) {
	// Re-queue jobs that were 'processing' when the server last crashed.
	if err := q.db.RequeueStalledJobs(ctx); err != nil {
		slog.Error("queue: requeue stalled", "error", err)
	}

	for i := 0; i < q.workers; i++ {
		q.wg.Add(1)
		go q.worker(ctx)
	}
}

// Stop gracefully shuts down all workers.
func (q *Queue) Stop() {
	close(q.done)
	q.wg.Wait()
}

// Enqueue persists a new job and notifies an idle worker.
func (q *Queue) Enqueue(ctx context.Context, workspaceID, jobType, payload string) (dbgen.Job, error) {
	job, err := q.db.CreateJob(ctx, dbgen.CreateJobParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Type:        jobType,
		Payload:     payload,
	})
	if err != nil {
		return dbgen.Job{}, err
	}
	slog.Debug("queue: created job", "job", job)

	// Best-effort wake up a worker; non-blocking.
	select {
	case q.notify <- struct{}{}:
	default:
	}

	return job, nil
}

// worker loops, claiming and processing one job at a time.
func (q *Queue) worker(ctx context.Context) {
	defer q.wg.Done()

	ticker := time.NewTicker(2 * time.Second)
	defer ticker.Stop()

	for {
		select {
		case <-q.done:
			return
		case <-ctx.Done():
			return
		case <-q.notify:
			q.processNext(ctx)
		case <-ticker.C:
			q.processNext(ctx)
		}
	}
}

// processNext claims the oldest pending job and runs its handler.
func (q *Queue) processNext(ctx context.Context) {
	job, err := q.db.ClaimNextJob(ctx)
	if err != nil {
		if err != sql.ErrNoRows {
			slog.Error("queue: claim job", "error", err)
		}
		return
	}

	// For transcode jobs, enforce concurrency limit of 2.
	if job.Type == JobTypeVideoTranscode {
		q.transcodeSem <- struct{}{}
		defer func() { <-q.transcodeSem }()
	}

	// For rebuild_variants jobs, enforce concurrency limit of 2.
	if job.Type == JobTypeRebuildVariants {
		q.rebuildSem <- struct{}{}
		defer func() { <-q.rebuildSem }()
	}

	defer func() {
		if r := recover(); r != nil {
			slog.Error("queue: job panicked", "job_id", job.ID, "job_type", job.Type, "panic", r, "stack", string(debug.Stack()))
			errMsg := fmt.Sprintf("panic: %v", r)
			_ = q.db.FailJob(ctx, dbgen.FailJobParams{
				Error: &errMsg,
				ID:    job.ID,
			})
		}
	}()

	h, ok := q.handlers[job.Type]
	if !ok {
		slog.Error("queue: no handler for job type", "job_type", job.Type, "job_id", job.ID)
		errMsg := "no handler registered"
		_ = q.db.FailJob(ctx, dbgen.FailJobParams{
			Error: &errMsg,
			ID:    job.ID,
		})
		return
	}

	jobCtx := auth.WithActor(ctx, auth.Actor{Type: "system"})
	jobCtx, span := telemetry.Tracer("damask/internal/queue").Start(jobCtx, "job."+job.Type,
		trace.WithAttributes(
			attribute.String("job.id", job.ID),
			attribute.String("job.type", job.Type),
			attribute.Int64("job.attempt", job.Attempts),
		),
	)
	defer span.End()
	if err := h(jobCtx, job); err != nil {
		span.RecordError(err)
		span.SetStatus(codes.Error, err.Error())
		slog.Error("queue: job failed", "job_id", job.ID, "job_type", job.Type, "error", err)
		errMsg := err.Error()
		_ = q.db.FailJob(ctx, dbgen.FailJobParams{
			Error: &errMsg,
			ID:    job.ID,
		})
		return
	}
	span.SetStatus(codes.Ok, "")

	if err := q.db.CompleteJob(ctx, job.ID); err != nil {
		slog.Error("queue: complete job", "job_id", job.ID, "error", err)
	}

	// Signal other workers to check for more work.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Job type constants used throughout the application.
const (
	JobTypeVersionThumbnail = "version_thumbnail"
	JobTypeVariantThumbnail = "generate_variant_thumbnail"

	// Variant jobs — user-triggered, each creates a variants row.
	JobTypeVideoCaptureImage = "video_capture_image"
	JobTypeVideoTranscode    = "video_transcode"
	JobTypeImageResize       = "image_resize"
	JobTypeImageConvert      = "image_convert"
	JobTypeImageCrop         = "image_crop"
	JobTypeImageWatermark    = "image_watermark"
	JobTypeImageBgRemove     = "image_bg_remove"
	JobTypeImageSmartCrop    = "image_smartcrop"

	// Ingress jobs.
	JobTypeIngestPoll  = "ingest_poll"
	JobTypeIngestFetch = "ingest_fetch"

	// Rebuild jobs — system-triggered on new version upload.
	JobTypeRebuildVariants = "rebuild_variants"

	// EXIF extraction.
	JobTypeExtractExif = "extract_exif"

	// Stack merge jobs.
	JobTypeStackMerge = "stack_merge"

	// Maintenance jobs.
	JobTypePurgeDeletedFields      = "purge_deleted_fields"
	JobTypeEnforceVersionRetention = "enforce_version_retention"
	JobTypePurgeVersionStorage     = "purge_version_storage"
	JobTypePurgeAuditLog           = "purge_event_log"
)
