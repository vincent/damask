// Package queue implements the in-process job queue backed by SQLite.
package queue

import (
	"context"
	"database/sql"
	"errors"
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
)

const (
	transcodeSemaphoreLimit    = 2
	rebuildSemaphoreLimit      = 2
	customFFmpegSemaphoreLimit = 2
	tickerInterval             = 2 * time.Second
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
	queries  *dbgen.Queries
	workers  int
	handlers map[string]HandlerFunc

	notify chan struct{}
	done   chan struct{}
	wg     sync.WaitGroup

	// Semaphore limiting concurrent transcode jobs.
	transcodeSem chan struct{}
	// Semaphore limiting concurrent rebuild_variants jobs.
	rebuildSem chan struct{}
	// Semaphore limiting concurrent custom_ffmpeg jobs, separate from
	// transcodeSem so a slow/hanging user-supplied command can't starve
	// real video transcode/watermark jobs.
	customFFmpegSem chan struct{}
}

// New creates a new Queue. Call Start() to begin processing.
func New(queries *dbgen.Queries, workers int) *Queue {
	if workers <= 0 {
		workers = 4
	}
	return &Queue{
		queries:         queries,
		workers:         workers,
		handlers:        make(map[string]HandlerFunc),
		notify:          make(chan struct{}, workers),
		done:            make(chan struct{}),
		transcodeSem:    make(chan struct{}, transcodeSemaphoreLimit),
		rebuildSem:      make(chan struct{}, rebuildSemaphoreLimit),
		customFFmpegSem: make(chan struct{}, customFFmpegSemaphoreLimit),
	}
}

// Register adds a handler for the given job type.
func (q *Queue) Register(jobType string, h HandlerFunc) {
	q.handlers[jobType] = h
}

// Start re-queues any stalled jobs and launches worker goroutines.
func (q *Queue) Start(ctx context.Context) {
	// Re-queue jobs that were 'processing' when the server last crashed.
	if err := q.queries.RequeueStalledJobs(ctx); err != nil {
		slog.ErrorContext(ctx, "queue: requeue stalled", "error", err)
	}

	for range q.workers {
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
func (q *Queue) Enqueue(ctx context.Context, workspaceID, jobType, payload string) (job dbgen.Job, err error) {
	_, span := telemetry.StartSpan(ctx, "service.queue.enqueue",
		attribute.String("damask.workspace_id", workspaceID),
		attribute.String("damask.job.type", jobType),
	)
	defer telemetry.EndSpan(span, err)

	job, err = q.queries.CreateJob(ctx, dbgen.CreateJobParams{
		ID:          uuid.NewString(),
		WorkspaceID: workspaceID,
		Type:        jobType,
		Payload:     payload,
	})
	if err != nil {
		return dbgen.Job{}, err
	}

	span.SetAttributes(attribute.String("damask.job.id", job.ID))
	slog.DebugContext(ctx, "queue: created job", "job", job)

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

	ticker := time.NewTicker(tickerInterval)
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
	var err error
	ctx, span := telemetry.StartSpan(ctx, "service.queue.next")
	defer telemetry.EndSpan(span, err)

	job, err := q.queries.ClaimNextJob(ctx)
	if err != nil {
		if !errors.Is(err, sql.ErrNoRows) {
			span.SetStatus(codes.Error, err.Error())
			slog.ErrorContext(ctx, "queue: claim job", "error", err)
		}
		return
	}

	defer slog.DebugContext(
		ctx,
		"end background job",
		"job",
		"service.queue.job."+job.Type,
		"job_id",
		job.ID,
		"workspace_id",
		job.WorkspaceID,
		"attempt",
		job.Attempts,
	)

	span.SetAttributes(attribute.String("damask.job.id", job.ID))
	span.SetAttributes(attribute.String("damask.job.type", job.Type))
	span.SetAttributes(attribute.String("damask.job.workspace_id", job.WorkspaceID))

	// For transcode jobs, enforce concurrency limit of 2.
	if job.Type == JobTypeVideoTranscode || job.Type == JobTypeVideoWatermark {
		q.transcodeSem <- struct{}{}
		defer func() { <-q.transcodeSem }()
	}

	// custom_ffmpeg runs user-supplied commands and gets its own semaphore so
	// a slow/hanging one can't starve the real transcode/watermark jobs above.
	if job.Type == JobTypeCustomFFmpeg {
		q.customFFmpegSem <- struct{}{}
		defer func() { <-q.customFFmpegSem }()
	}

	// For rebuild_variants jobs, enforce concurrency limit of 2.
	if job.Type == JobTypeRebuildVariants {
		q.rebuildSem <- struct{}{}
		defer func() { <-q.rebuildSem }()
	}

	h, ok := q.handlers[job.Type]
	if !ok {
		slog.ErrorContext(ctx, "queue: no handler for job type", "job_type", job.Type, "job_id", job.ID)
		span.SetStatus(codes.Error, "queue: no handler registered")
		errMsg := "no handler registered"
		_ = q.queries.FailJob(ctx, dbgen.FailJobParams{
			Error: &errMsg,
			ID:    job.ID,
		})
		return
	}

	jobCtx := auth.WithActor(ctx, auth.Actor{Type: "system"})
	jobCtx, jobSpan := telemetry.StartBackgroundSpan(jobCtx, "service.queue.job."+job.Type,
		attribute.String("job.id", job.ID),
		attribute.String("job.type", job.Type),
		attribute.String("job.params", job.Payload),
		attribute.Int64("job.attempt", job.Attempts),
		attribute.String("damask.workspace_id", job.WorkspaceID),
	)

	slog.DebugContext(
		ctx,
		"start background job",
		"job",
		"service.queue.job."+job.Type,
		"job_id",
		job.ID,
		"workspace_id",
		job.WorkspaceID,
		"attempt",
		job.Attempts,
	)

	var jobFailed bool
	func() {
		defer func() {
			if r := recover(); r != nil {
				jobFailed = true
				panicErr := fmt.Errorf("panic: %v", r)
				telemetry.RecordError(jobSpan, panicErr)
				span.SetStatus(codes.Error, "queue: job panicked")
				slog.ErrorContext(
					ctx,
					"queue: job panicked",
					"job_id",
					job.ID,
					"job_type",
					job.Type,
					"panic",
					r,
					"stack",
					string(debug.Stack()),
				)
				errMsg := panicErr.Error()
				_ = q.queries.FailJob(ctx, dbgen.FailJobParams{
					Error: &errMsg,
					ID:    job.ID,
				})
			}
			jobSpan.End()
		}()

		if err = h(jobCtx, job); err != nil {
			jobFailed = true
			telemetry.RecordError(jobSpan, err)
			slog.ErrorContext(ctx, "queue: job failed", "job_id", job.ID, "job_type", job.Type, "error", err)
			errMsg := err.Error()
			_ = q.queries.FailJob(ctx, dbgen.FailJobParams{
				Error: &errMsg,
				ID:    job.ID,
			})
			return
		}
		jobSpan.SetStatus(codes.Ok, "")
	}()

	if jobFailed {
		return
	}

	span.SetStatus(codes.Ok, "")

	if err = q.queries.CompleteJob(ctx, job.ID); err != nil {
		slog.ErrorContext(ctx, "queue: complete job", "job_id", job.ID, "error", err)
	}

	// Signal other workers to check for more work.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Job type constants used throughout the application.
const (
	JobTypeVersionThumbnail            = "version_thumbnail"
	JobTypeVariantThumbnail            = "generate_variant_thumbnail"
	JobTypeOCRTextTrack                = "ocr_text_track"
	JobTypeAIImageDescriptionTextTrack = "ai_image_description_text_track"
	JobTypeExtractPDFTextTrack         = "document_pdf_extract_text_track"
	JobTypeExtractPlainTextTrack       = "document_plain_extract_text_track"
	JobTypeExtractDocumentTextTrack    = "document_office_extract_text_track"

	JobTypeVideoCaptureImage = "video_capture_image"
	JobTypeVideoTranscode    = "video_transcode"
	JobTypeVideoWatermark    = "video_watermark"
	JobTypeImageResize       = "image_resize"
	JobTypeImageConvert      = "image_convert"
	JobTypeImageCrop         = "image_crop"
	JobTypeImageWatermark    = "image_watermark"
	JobTypeImageBgRemove     = "image_bg_remove"
	JobTypeImageWithPrompt   = "image_with_prompt"
	JobTypeImageSmartCrop    = "image_smart_crop"
	JobTypeExtractAudio      = "video_extract"
	JobTypeTranscodeAudio    = "audio_transcode"
	JobTypeNormalizeAudio    = "audio_normalize"
	JobTypeCustomFFmpeg      = "custom_ffmpeg"

	JobTypeIngestPoll  = "ingest_poll"
	JobTypeIngestFetch = "ingest_fetch"

	JobTypeRebuildVariants = "rebuild_variants"
	JobTypeRunWorkflow     = "run_workflow"

	JobTypeExtractExif      = "extract_exif"
	JobTypeExtractMediaTags = "extract_media_tags"

	JobTypeStackMerge         = "stack_merge"
	JobTypeCreateVariantDraft = "create_variant_draft"

	JobTypeExportRun = "export_run"

	JobTypePurgeDeletedFields      = "purge_deleted_fields"
	JobTypeEnforceVersionRetention = "enforce_version_retention"
	JobTypePurgeVersionStorage     = "purge_version_storage"
	JobTypePurgeAuditLog           = "purge_event_log"
	JobTypePurgeScratchVariants    = "purge_scratch_variants"

	JobTypeVisualSimilarityBackfill = "visual_similarity_backfill"
)
