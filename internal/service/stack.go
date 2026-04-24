package service

import (
	"archive/zip"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"strings"

	"damask/server/internal/apperr"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	"damask/server/internal/storage"
)

// ExportZipParams is the input for StackService.ExportZip.
type ExportZipParams struct {
	AssetIDs []string
	Filename string // without extension; defaults to "stack-export"
}

// MergeParams is the input for StackService.EnqueueMerge.
type MergeParams struct {
	AssetIDs       []string
	OutputType     string // "gif" | "pdf"
	OutputFilename string
	GifFrameMs     int
}

func (p MergeParams) Validate() error {
	if len(p.AssetIDs) == 0 {
		return fmt.Errorf("asset_ids must not be empty: %w", apperr.ErrInvalidInput)
	}
	if p.OutputType != "gif" && p.OutputType != "pdf" {
		return fmt.Errorf("output_type must be gif or pdf: %w", apperr.ErrInvalidInput)
	}
	return nil
}

// stackMergePayload is the JSON payload stored in the jobs queue row.
type stackMergePayload struct {
	WorkspaceID    string   `json:"workspace_id"`
	CreatedBy      string   `json:"created_by"`
	AssetIDs       []string `json:"asset_ids"`
	OutputType     string   `json:"output_type"`
	OutputFilename string   `json:"output_filename"`
	GifFrameMs     int      `json:"gif_frame_ms"`
}

type stackService struct {
	assets   repository.AssetRepository
	versions repository.VersionRepository
	storage  storage.Storage
	q        queue.JobQueue
}

// NewStackService returns a StackService.
func NewStackService(assets repository.AssetRepository, versions repository.VersionRepository, stor storage.Storage, q queue.JobQueue) StackService {
	return &stackService{assets: assets, versions: versions, storage: stor, q: q}
}

// ExportZip streams a ZIP archive of the given assets into w.
// All asset IDs must belong to workspaceID; if any are missing the call returns ErrForbidden.
func (s *stackService) ExportZip(ctx context.Context, workspaceID string, p ExportZipParams, w io.Writer) error {
	if len(p.AssetIDs) == 0 {
		return fmt.Errorf("asset_ids must not be empty: %w", apperr.ErrInvalidInput)
	}

	filename := sanitiseStackFilename(p.Filename)
	if filename == "" {
		filename = "stack-export"
	}
	_ = filename // used by caller for Content-Disposition

	// Verify all IDs belong to this workspace.
	for _, id := range p.AssetIDs {
		if _, err := s.assets.GetByID(ctx, workspaceID, id); err != nil {
			return fmt.Errorf("asset %q not found in workspace: %w", id, apperr.ErrForbidden)
		}
	}

	type entry struct {
		name       string
		storageKey string
	}
	var entries []entry
	var missingNames []string
	usedNames := map[string]int{}

	for _, id := range p.AssetIDs {
		asset, err := s.assets.GetByID(ctx, workspaceID, id)
		if err != nil {
			return err
		}

		version, err := s.versions.GetCurrentByAsset(ctx, asset.ID)
		if err != nil {
			missingNames = append(missingNames, asset.OriginalFilename)
			continue
		}

		base := asset.OriginalFilename
		usedNames[base]++
		name := base
		if usedNames[base] > 1 {
			ext := ""
			stem := base
			if dot := strings.LastIndex(base, "."); dot >= 0 {
				stem = base[:dot]
				ext = base[dot:]
			}
			name = fmt.Sprintf("%s_%d%s", stem, usedNames[base], ext)
		}
		entries = append(entries, entry{name: name, storageKey: version.StorageKey})
	}

	zw := zip.NewWriter(w)
	missing := missingNames

	for _, e := range entries {
		rc, err := s.storage.Get(e.storageKey)
		if err != nil {
			missing = append(missing, e.name)
			continue
		}
		fw, err := zw.Create(e.name)
		if err != nil {
			_ = rc.Close()
			missing = append(missing, e.name)
			continue
		}
		if _, err := io.Copy(fw, rc); err != nil {
			slog.WarnContext(ctx, "zip copy error", "name", e.name, "err", err)
		}
		_ = rc.Close()
	}

	if len(missing) > 0 {
		fw, err := zw.Create("_missing_files.txt")
		if err == nil {
			for _, n := range missing {
				_, _ = fmt.Fprintln(fw, n)
			}
		}
	}

	return zw.Close()
}

// EnqueueMerge enqueues a stack_merge job and returns the job ID.
// All asset IDs must belong to workspaceID.
func (s *stackService) EnqueueMerge(ctx context.Context, workspaceID, userID string, p MergeParams) (string, error) {
	if err := p.Validate(); err != nil {
		return "", err
	}

	for _, id := range p.AssetIDs {
		if _, err := s.assets.GetByID(ctx, workspaceID, id); err != nil {
			return "", fmt.Errorf("asset %q not found in workspace: %w", id, apperr.ErrForbidden)
		}
	}

	payload := stackMergePayload{
		WorkspaceID:    workspaceID,
		CreatedBy:      userID,
		AssetIDs:       p.AssetIDs,
		OutputType:     p.OutputType,
		OutputFilename: p.OutputFilename,
		GifFrameMs:     p.GifFrameMs,
	}
	payloadJSON, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("could not encode payload: %w", err)
	}

	job, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeStackMerge, string(payloadJSON))
	if err != nil {
		return "", fmt.Errorf("could not enqueue merge job: %w", apperr.ErrConflict)
	}
	return job.ID, nil
}

func sanitiseStackFilename(s string) string {
	var b strings.Builder
	for _, r := range s {
		if r == '/' || r == '\\' || r < 0x20 || r == 0x7f {
			continue
		}
		b.WriteRune(r)
	}
	return strings.TrimSpace(b.String())
}
