// Package queue implements the in-process job queue backed by SQLite.
package queue

import (
	"context"
	"database/sql"
	"fmt"
	"log"
	"sync"
	"time"

	dbgen "damask/server/internal/db/gen"

	"github.com/google/uuid"
)

// HandlerFunc processes a job payload and returns an error on failure.
type HandlerFunc func(ctx context.Context, job dbgen.Job) error

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
		log.Printf("queue: requeue stalled: %v", err)
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
			log.Printf("queue: claim job: %v", err)
		}
		return
	}

	// For transcode jobs, enforce concurrency limit of 2.
	if job.Type == JobTypeVideoTranscode {
		q.transcodeSem <- struct{}{}
		defer func() { <-q.transcodeSem }()
	}

	defer func() {
		if r := recover(); r != nil {
			log.Printf("queue: job %s (%s) panicked: %v", job.ID, job.Type, r)
			errMsg := fmt.Sprintf("panic: %v", r)
			_ = q.db.FailJob(ctx, dbgen.FailJobParams{
				Error: &errMsg,
				ID:    job.ID,
			})
		}
	}()

	h, ok := q.handlers[job.Type]
	if !ok {
		log.Printf("queue: no handler for job type %q (id=%s)", job.Type, job.ID)
		errMsg := "no handler registered"
		_ = q.db.FailJob(ctx, dbgen.FailJobParams{
			Error: &errMsg,
			ID:    job.ID,
		})
		return
	}

	if err := h(ctx, job); err != nil {
		log.Printf("queue: job %s (%s) failed: %v", job.ID, job.Type, err)
		errMsg := err.Error()
		_ = q.db.FailJob(ctx, dbgen.FailJobParams{
			Error: &errMsg,
			ID:    job.ID,
		})
		return
	}

	if err := q.db.CompleteJob(ctx, job.ID); err != nil {
		log.Printf("queue: complete job %s: %v", job.ID, err)
	}

	// Signal other workers to check for more work.
	select {
	case q.notify <- struct{}{}:
	default:
	}
}

// Job type constants used throughout the application.
const (
	JobTypeImageThumbnail = "image_thumbnail"
	JobTypeImageResize    = "image_resize"
	JobTypeImageConvert   = "image_convert"
	JobTypeImageCrop      = "image_crop"
	JobTypeImageWatermark = "image_watermark"
	JobTypeImageBgRemove  = "image_bg_remove"
	JobTypeImageSmartCrop = "image_smartcrop"
	JobTypeVideoThumbnail = "video_thumbnail"
	JobTypeVideoTranscode = "video_transcode"
	JobTypeAudioWaveform  = "audio_waveform"
	JobTypePdfThumbnail   = "pdf_thumbnail"
	JobTypeIngestPoll          = "ingest_poll"
	JobTypeIngestFetch         = "ingest_fetch"
	JobTypePurgeDeletedFields  = "purge_deleted_fields"
)
