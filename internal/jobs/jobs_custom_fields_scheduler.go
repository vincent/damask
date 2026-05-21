package jobs

import (
	"context"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

const (
	schedulerCronHour   = 3
	schedulerCronMinute = 0
)

// FieldCleanupScheduler enqueues a purge_deleted_fields job once per day at 03:00 UTC.
type FieldCleanupScheduler struct {
	db    *dbgen.Queries
	queue queue.JobQueue
}

// NewFieldCleanupScheduler creates a FieldCleanupScheduler.
func NewFieldCleanupScheduler(db *dbgen.Queries, q queue.JobQueue) *FieldCleanupScheduler {
	return &FieldCleanupScheduler{db: db, queue: q}
}

// Start launches the scheduler goroutine. Returns immediately; exits when ctx is cancelled.
func (s *FieldCleanupScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := NextDaily(schedulerCronHour, schedulerCronMinute)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := s.queue.Enqueue(ctx, "system", queue.JobTypePurgeDeletedFields, "{}"); err != nil {
					slog.ErrorContext(ctx, "field cleanup scheduler: enqueue purge", "error", err)
				}
			}
		}
	}()
}

// jobPurgeDeletedFields hard-deletes field_definitions (and their values) that
// have been soft-deleted for more than 30 days.
func (s *JobServer) jobPurgeDeletedFields(ctx context.Context, _ dbgen.Job) error {
	ids, err := s.db.HardDeleteExpiredFieldDefinitions(ctx)
	if err != nil {
		return err
	}

	if len(ids) == 0 {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback() //nolint:errcheck // Rollback is best-effort after commit or earlier query errors.
	qtx := s.db.WithTx(tx)

	for _, id := range ids {
		if err := qtx.DeleteAssetFieldValuesByField(ctx, id); err != nil {
			return err
		}
		if err := qtx.DeleteProjectFieldValuesByField(ctx, id); err != nil {
			return err
		}
		if err := qtx.HardDeleteFieldDefinition(ctx, id); err != nil {
			return err
		}
	}

	if err := tx.Commit(); err != nil {
		return err
	}

	slog.InfoContext(ctx, "field cleanup: purged expired field definitions", "count", len(ids))
	return nil
}
