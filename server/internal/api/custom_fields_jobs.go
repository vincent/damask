package api

import (
	"context"
	"log"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// FieldCleanupScheduler enqueues a purge_deleted_fields job once per day at 03:00 UTC.
type FieldCleanupScheduler struct {
	db    *dbgen.Queries
	queue *queue.Queue
}

// NewFieldCleanupScheduler creates a FieldCleanupScheduler.
func NewFieldCleanupScheduler(db *dbgen.Queries, q *queue.Queue) *FieldCleanupScheduler {
	return &FieldCleanupScheduler{db: db, queue: q}
}

// Start launches the scheduler goroutine. Returns immediately; exits when ctx is cancelled.
func (s *FieldCleanupScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := nextDaily(3, 0)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := s.queue.Enqueue(ctx, "system", queue.JobTypePurgeDeletedFields, "{}"); err != nil {
					log.Printf("field cleanup scheduler: enqueue purge: %v", err)
				}
			}
		}
	}()
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

// jobPurgeDeletedFields hard-deletes field_definitions (and their values) that
// have been soft-deleted for more than 30 days.
func (s *Server) jobPurgeDeletedFields(ctx context.Context, job dbgen.Job) error {
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
	defer tx.Rollback() //nolint:errcheck
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

	log.Printf("field cleanup: purged %d expired field definitions", len(ids))
	return nil
}
