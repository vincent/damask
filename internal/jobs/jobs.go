package jobs

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"io"
	"os"
	"time"

	"damask/server/internal/assetio"
	"damask/server/internal/audit"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/imagerouter"
	"damask/server/internal/ingress"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"
)

const schedulerCronInterval = 24 * time.Hour

// exportService is the subset of service.ExportService used by job handlers and the scheduler.
// Defined here to avoid an import cycle (service imports jobs for payload types and enqueue helpers).
type exportService interface {
	ExecuteRun(ctx context.Context, workspaceID, configID, runID string) error
	ListDueConfigs(ctx context.Context) ([]repository.ExportConfig, error)
	CreateRun(ctx context.Context, run repository.ExportRun) (repository.ExportRun, error)
	SetConfigLastRun(ctx context.Context, configID string, p repository.ExportRunResult) error
}

type exifService interface {
	ExtractForAsset(ctx context.Context, workspaceID, assetID, userID string) error
}

type fieldPurgeService interface {
	PurgeExpiredFields(ctx context.Context) (int, error)
}

// textTrackService is the subset of service.TextTrackService used by job handlers.
// Defined here to avoid an import cycle (service imports jobs for queue payload types).
type textTrackService interface {
	RunOCR(ctx context.Context, workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat string) error
}

// JobServer holds shared dependencies injected at startup.
type JobServer struct {
	queries        *dbgen.Queries
	sqlDB          *sql.DB
	storage        storage.Storage
	queue          queue.JobQueue
	mailer         mail.Mailer
	hub            events.EventHub
	cfg            *config.Config
	audit          *audit.EventWriter
	handlers       map[string]queue.HandlerFunc
	trf            transform.Transformer
	tmb            transform.Thumbnailer
	injestor       assetio.Injestor
	imgKeyResolver imagerouter.KeyResolver
	workflowExec   *workflow.Executor
	exportSvc      exportService
	exifSvc        exifService
	fieldSvc       fieldPurgeService
	textTrackSvc   textTrackService
	storageSvc     ingress.StorageLimitChecker
}

func NewJobServer(
	queries *dbgen.Queries,
	sqlDB *sql.DB,
	stor storage.Storage,
	hub events.EventHub,
	q queue.JobQueue,
	mailer mail.Mailer,
	trf transform.Transformer,
	tmb transform.Thumbnailer,
	cfg *config.Config,
	injestor assetio.Injestor,
	imgKeyResolver imagerouter.KeyResolver,
	workflowExec *workflow.Executor,
	exportSvc exportService,
	exifSvc exifService,
	fieldSvc fieldPurgeService,
	textTrackSvc textTrackService,
	storageSvc ingress.StorageLimitChecker,
) *JobServer {
	if imgKeyResolver == nil {
		panic("jobs: NewJobServer requires a non-nil imagerouter key resolver")
	}
	if workflowExec == nil {
		panic("jobs: NewJobServer requires a non-nil workflow executor")
	}
	return &JobServer{
		audit:          audit.New(sqlDB),
		cfg:            cfg,
		queries:        queries,
		exportSvc:      exportSvc,
		exifSvc:        exifSvc,
		fieldSvc:       fieldSvc,
		textTrackSvc:   textTrackSvc,
		handlers:       make(map[string]queue.HandlerFunc),
		hub:            hub,
		imgKeyResolver: imgKeyResolver,
		injestor:       injestor,
		mailer:         mailer,
		queue:          q,
		sqlDB:          sqlDB,
		storage:        stor,
		storageSvc:     storageSvc,
		tmb:            tmb,
		trf:            trf,
		workflowExec:   workflowExec,
	}
}

// EnqueueForTest inserts a job directly into the queue. Intended for use in tests only.
func (s *JobServer) EnqueueForTest(ctx context.Context, workspaceID, jobType, payload string) (string, error) {
	job, err := s.queue.Enqueue(ctx, workspaceID, jobType, payload)
	if err != nil {
		return "", err
	}
	return job.ID, nil
}

// DrainForTest registers all handlers and synchronously processes every pending
// job. Intended for use in tests only — not safe for concurrent use.
func (s *JobServer) DrainForTest(ctx context.Context) {
	s.RegisterJobHandlers()
	for {
		job, err := s.queries.ClaimNextJob(ctx)
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
	ingressWorker := ingress.NewWorker(s.queries, s.sqlDB, s.storage, s.queue, s.cfg, s.audit, s.mailer, s.injestor, s.storageSvc)
	reg(queue.JobTypeIngestPoll, ingressWorker.HandlePoll)
	reg(queue.JobTypeIngestFetch, ingressWorker.HandleFetch)

	reg(queue.JobTypeVersionThumbnail, s.jobVersionThumbnail)
	reg(queue.JobTypeVariantThumbnail, s.jobVariantThumbnail)

	// Variant jobs — registered from the central registry.
	variantHandler := s.wrapVariantJob(s.jobVariant)
	for jobType := range s.variantRegistry() {
		reg(jobType, variantHandler)
	}
	reg(queue.JobTypeOCRTextTrack, s.wrapSimpleJob(s.jobOCRTextTrack))

	// Rebuild jobs — system-triggered on version upload.
	reg(queue.JobTypeRebuildVariants, s.jobRebuildVariants)
	reg(queue.JobTypeRunWorkflow, s.jobRunWorkflow)

	// EXIF extraction.
	reg(queue.JobTypeExtractExif, s.jobExtractExif)
	reg(queue.JobTypeExtractMediaTags, s.jobExtractMediaTags)

	// Stack merge jobs.
	reg(queue.JobTypeStackMerge, s.jobStackMerge)

	// Variant draft jobs.
	reg(queue.JobTypeCreateVariantDraft, s.jobCreateVariantDraft)

	// Export jobs.
	reg(queue.JobTypeExportRun, s.jobExportRun)

	// Maintenance jobs.
	reg(queue.JobTypePurgeDeletedFields, s.jobPurgeDeletedFields)
	reg(queue.JobTypeEnforceVersionRetention, s.jobEnforceVersionRetention)
	reg(queue.JobTypePurgeVersionStorage, s.jobPurgeVersionStorage)
	reg(queue.JobTypePurgeAuditLog, s.jobPurgeAuditLog)
	reg(queue.JobTypePurgeScratchVariants, s.jobPurgeScratchVariants)
}

// ---- OS helpers ----

func writeToTempFile(_ context.Context, src io.Reader, ext string) (string, func(), error) {
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

// NextDaily returns the next occurrence of hour:min UTC on or after now.
func NextDaily(hour, minute int) time.Time {
	now := time.Now().UTC()
	next := time.Date(now.Year(), now.Month(), now.Day(), hour, minute, 0, 0, time.UTC)
	if !next.After(now) {
		next = next.Add(schedulerCronInterval)
	}
	return next
}

func (s *JobServer) wrapSimpleJob(fn func(context.Context, string) error) queue.HandlerFunc {
	return func(ctx context.Context, job dbgen.Job) error {
		return fn(ctx, job.Payload)
	}
}

func (s *JobServer) jobRunWorkflow(ctx context.Context, job dbgen.Job) error {
	var payload workflow.RunWorkflowPayload
	if err := json.Unmarshal([]byte(job.Payload), &payload); err != nil {
		return fmt.Errorf("parse workflow payload: %w", err)
	}
	return s.workflowExec.Run(ctx, payload.RunID)
}
