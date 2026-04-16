package ingress

import (
	"context"
	"encoding/json"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// Scheduler queries for due ingress sources every 60 seconds
// and enqueues ingest_poll jobs for each.
type Scheduler struct {
	db    *dbgen.Queries
	queue queue.JobQueue
}

// NewScheduler creates a Scheduler.
func NewScheduler(db *dbgen.Queries, qu queue.JobQueue) *Scheduler {
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
		slog.Error("ingress scheduler: list due sources", "error", err)
		return
	}

	for _, src := range sources {
		payload, _ := json.Marshal(PollJobPayload{
			SourceID:    src.ID,
			WorkspaceID: src.WorkspaceID,
		})
		if _, err := s.queue.Enqueue(ctx, src.WorkspaceID, queue.JobTypeIngestPoll, string(payload)); err != nil {
			slog.Error("ingress scheduler: enqueue poll", "source_id", src.ID, "error", err)
			continue
		}
		// Mark last_polled_at immediately to prevent double-scheduling.
		// error_count and last_error are untouched here; the poll worker updates them.
		if err := s.db.MarkIngressSourceScheduled(ctx, src.ID); err != nil {
			slog.Error("ingress scheduler: mark scheduled", "source_id", src.ID, "error", err)
		}
	}
}
