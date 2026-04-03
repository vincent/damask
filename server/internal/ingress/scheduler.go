package ingress

import (
	"context"
	"encoding/json"
	"log"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// Scheduler queries for due ingress sources every 60 seconds
// and enqueues ingest_poll jobs for each.
type Scheduler struct {
	db    *dbgen.Queries
	queue *queue.Queue
}

// NewScheduler creates a Scheduler.
func NewScheduler(db *dbgen.Queries, qu *queue.Queue) *Scheduler {
	return &Scheduler{db: db, queue: qu}
}

// Start launches the scheduler goroutine. Returns immediately.
// Exits when ctx is cancelled.
func (s *Scheduler) Start(ctx context.Context) {
	go func() {
		ticker := time.NewTicker(60 * time.Second)
		defer ticker.Stop()
		// Fire immediately on startup to pick up overdue sources
		s.tick(ctx)
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s.tick(ctx)
			}
		}
	}()
}

func (s *Scheduler) tick(ctx context.Context) {
	sources, err := s.db.ListDueIngressSources(ctx)
	if err != nil {
		log.Printf("ingress scheduler: list due sources: %v", err)
		return
	}

	for _, src := range sources {
		payload, _ := json.Marshal(PollJobPayload{
			SourceID:    src.ID,
			WorkspaceID: src.WorkspaceID,
		})
		if _, err := s.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestPoll, string(payload)); err != nil {
			log.Printf("ingress scheduler: enqueue poll for source %s: %v", src.ID, err)
			continue
		}
		// Mark as polled immediately to prevent double-scheduling
		// (last_error is cleared here; the actual poll worker updates it)
		if err := s.db.MarkIngressSourcePolled(ctx, dbgen.MarkIngressSourcePolledParams{
			ID:        src.ID,
			LastError: src.LastError, // preserve existing error until poll succeeds
		}); err != nil {
			log.Printf("ingress scheduler: mark polled for source %s: %v", src.ID, err)
		}
	}
}
