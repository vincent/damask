package api

import (
	"context"
	"log"
	"time"

	"damask/server/internal/queue"
)

// RetentionScheduler fires the enforce_version_retention job once per night
// at approximately 02:00 UTC.
type RetentionScheduler struct {
	queue *queue.Queue
}

// NewRetentionScheduler creates a RetentionScheduler.
func NewRetentionScheduler(q *queue.Queue) *RetentionScheduler {
	return &RetentionScheduler{queue: q}
}

// Start launches the scheduler goroutine. It exits when ctx is cancelled.
func (r *RetentionScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := nextRunAt(time.Now().UTC())
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := r.queue.Enqueue(ctx, "system", queue.JobTypeEnforceVersionRetention, "{}"); err != nil {
					log.Printf("retention scheduler: enqueue: %v", err)
				} else {
					log.Printf("retention scheduler: enqueued enforce_version_retention")
				}
			}
		}
	}()
}

// nextRunAt returns the next 02:00 UTC after t.
func nextRunAt(t time.Time) time.Time {
	next := time.Date(t.Year(), t.Month(), t.Day(), 2, 0, 0, 0, time.UTC)
	if !next.After(t) {
		next = next.Add(24 * time.Hour)
	}
	return next
}
