package api

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
)

// purgeVersionStoragePayload carries the data needed by the purge job.
type purgeVersionStoragePayload struct {
	VersionID   string `json:"version_id"`
	WorkspaceID string `json:"workspace_id"`
}

// jobEnforceVersionRetention runs the retention policy for all workspaces
// that have version_retention_count > 0.
func (s *Server) jobEnforceVersionRetention(ctx context.Context, job dbgen.Job) error {
	workspaces, err := s.db.ListWorkspacesWithRetention(ctx)
	if err != nil {
		return fmt.Errorf("list workspaces: %w", err)
	}

	for _, ws := range workspaces {
		if ws.VersionRetentionCount <= 0 {
			continue
		}
		if err := s.enforceRetentionForWorkspace(ctx, ws); err != nil {
			log.Printf("retention: workspace %s: %v", ws.ID, err)
		}
	}
	return nil
}

func (s *Server) enforceRetentionForWorkspace(ctx context.Context, ws dbgen.Workspace) error {
	assetIDs, err := s.db.ListAssetsWithVersions(ctx, ws.ID)
	if err != nil {
		return fmt.Errorf("list assets: %w", err)
	}

	keep := ws.VersionRetentionCount

	for _, assetID := range assetIDs {
		count, err := s.db.CountActiveVersions(ctx, assetID)
		if err != nil {
			log.Printf("retention: count versions for %s: %v", assetID, err)
			continue
		}
		if count <= keep {
			continue
		}

		// The query returns non-current versions ordered by version_num DESC,
		// skipping the `keep` most recent, so what comes back should be deleted.
		beyond, err := s.db.ListVersionsBeyondRetention(ctx, dbgen.ListVersionsBeyondRetentionParams{
			AssetID: assetID,
			Offset:  keep, // skip the `keep` most recent non-current versions
		})
		if err != nil {
			log.Printf("retention: list beyond for %s: %v", assetID, err)
			continue
		}

		for _, v := range beyond {
			// Safety: never soft-delete the current version.
			if v.IsCurrent == 1 {
				continue
			}
			if err := s.db.SoftDeleteVersion(ctx, v.ID); err != nil {
				log.Printf("retention: soft-delete version %s: %v", v.ID, err)
				continue
			}
			// Enqueue physical purge after 7-day grace period.
			payload, _ := json.Marshal(purgeVersionStoragePayload{
				VersionID:   v.ID,
				WorkspaceID: ws.ID,
			})
			if _, err := s.queue.Enqueue(ctx, ws.ID, queue.JobTypePurgeVersionStorage, string(payload)); err != nil {
				log.Printf("retention: enqueue purge for version %s: %v", v.ID, err)
			}
		}
	}
	return nil
}

// jobPurgeVersionStorage physically removes a soft-deleted version's files
// from storage after the 7-day grace period, then hard-deletes the row.
func (s *Server) jobPurgeVersionStorage(ctx context.Context, job dbgen.Job) error {
	var p purgeVersionStoragePayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("parse payload: %w", err)
	}

	ver, err := s.db.GetVersionByIDUnchecked(ctx, p.VersionID)
	if err != nil {
		// Already hard-deleted; nothing to do.
		return nil
	}

	// Safety guard: never purge a current version.
	if ver.IsCurrent == 1 {
		log.Printf("purge-version-storage: version %s is current — skipping", p.VersionID)
		return nil
	}

	// Enforce 7-day grace period.
	if ver.DeletedAt == nil {
		log.Printf("purge-version-storage: version %s has no deleted_at — skipping", p.VersionID)
		return nil
	}
	deletedAt, err := parseSQLiteTime(*ver.DeletedAt)
	if err != nil {
		return fmt.Errorf("parse deleted_at: %w", err)
	}
	if time.Since(deletedAt) < 7*24*time.Hour {
		// Grace period not yet elapsed — re-enqueue so the job is retried rather
		// than silently dropped. It will check the elapsed time again on next run.
		log.Printf("purge-version-storage: version %s grace period not elapsed — re-enqueuing", p.VersionID)
		payload, _ := json.Marshal(p)
		if _, err := s.queue.Enqueue(ctx, p.WorkspaceID, queue.JobTypePurgeVersionStorage, string(payload)); err != nil {
			log.Printf("purge-version-storage: re-enqueue version %s: %v", p.VersionID, err)
		}
		return nil
	}

	// Delete storage files.
	if err := s.storage.Delete(ver.StorageKey); err != nil {
		log.Printf("purge-version-storage: delete storage %s: %v", ver.StorageKey, err)
	}
	if ver.ThumbnailKey != nil {
		if err := s.storage.Delete(*ver.ThumbnailKey); err != nil {
			log.Printf("purge-version-storage: delete thumb %s: %v", *ver.ThumbnailKey, err)
		}
	}

	// Hard-delete the row.
	return s.db.HardDeleteVersion(ctx, p.VersionID)
}
