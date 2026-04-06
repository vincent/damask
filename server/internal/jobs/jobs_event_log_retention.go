package jobs

import (
	"context"
	"fmt"
	"log"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// jobPurgeAuditLog runs the event log retention policy for all workspaces.
// For each workspace it:
//  1. Deletes asset_downloaded events older than download_log_retention_days.
//  2. Deletes all other events older than event_log_retention_days.
//  3. Deletes project events older than event_log_retention_days.
func (s *JobServer) jobPurgeAuditLog(ctx context.Context, job dbgen.Job) error {
	workspaces, err := s.db.ListWorkspacesForEventRetention(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		if err := s.purgeAuditLogForWorkspace(ctx, ws); err != nil {
			log.Printf("audit-log retention: workspace %s: %v", ws.ID, err)
		}
	}
	return nil
}

func (s *JobServer) purgeAuditLogForWorkspace(ctx context.Context, ws dbgen.Workspace) error {
	now := time.Now().UTC()

	// Purge download events on their shorter retention cycle.
	if ws.DownloadLogRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -int(ws.DownloadLogRetentionDays)).Format("2006-01-02 15:04:05")
		if err := s.db.DeleteDownloadEventsOlderThan(ctx, dbgen.DeleteDownloadEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			log.Printf("audit-log retention: workspace %s: purge downloads: %v", ws.ID, err)
		} else {
			log.Printf("audit-log retention: workspace %s: purged download events before %s", ws.ID, cutoff)
		}
	}

	// Purge all asset events beyond the general retention window.
	if ws.AuditLogRetentionDays > 0 {
		cutoff := now.AddDate(0, 0, -int(ws.AuditLogRetentionDays)).Format("2006-01-02 15:04:05")
		if err := s.db.DeleteAssetEventsOlderThan(ctx, dbgen.DeleteAssetEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			log.Printf("audit-log retention: workspace %s: purge asset events: %v", ws.ID, err)
		} else {
			log.Printf("audit-log retention: workspace %s: purged asset events before %s", ws.ID, cutoff)
		}
		if err := s.db.DeleteProjectEventsOlderThan(ctx, dbgen.DeleteProjectEventsOlderThanParams{
			WorkspaceID: ws.ID,
			Cutoff:      cutoff,
		}); err != nil {
			log.Printf("audit-log retention: workspace %s: purge project events: %v", ws.ID, err)
		} else {
			log.Printf("audit-log retention: workspace %s: purged project events before %s", ws.ID, cutoff)
		}
	}

	return nil
}

// AuditLogRetentionScheduler fires the purge_event_log job nightly at 04:00 UTC.
type AuditLogRetentionScheduler struct {
	queue queue.JobQueue
}

func NewAuditLogRetentionScheduler(q queue.JobQueue) *AuditLogRetentionScheduler {
	return &AuditLogRetentionScheduler{queue: q}
}

func (r *AuditLogRetentionScheduler) Start(ctx context.Context) {
	go func() {
		for {
			next := NextDaily(4, 0)
			select {
			case <-ctx.Done():
				return
			case <-time.After(time.Until(next)):
				if _, err := r.queue.Enqueue(ctx, "system", queue.JobTypePurgeAuditLog, "{}"); err != nil {
					log.Printf("audit-log retention scheduler: enqueue: %v", err)
				} else {
					log.Printf("audit-log retention scheduler: enqueued purge_event_log")
				}
			}
		}
	}()
}
