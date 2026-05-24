package jobs

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/export"
	"damask/server/internal/ingress"
	"damask/server/internal/repository"
)

type exportRunPayload struct {
	ExportConfigID string `json:"export_config_id"`
	ExportRunID    string `json:"export_run_id"`
}

func (s *JobServer) jobExportRun(ctx context.Context, job dbgen.Job) error {
	var p exportRunPayload
	if err := json.Unmarshal([]byte(job.Payload), &p); err != nil {
		return fmt.Errorf("export job: parse payload: %w", err)
	}

	configRepo := s.exportConfigs
	runRepo := s.exportRuns

	cfg, err := configRepo.Get(ctx, job.WorkspaceID, p.ExportConfigID)
	if err != nil {
		return fmt.Errorf("export job: load config %s: %w", p.ExportConfigID, err)
	}

	configJSON, err := ingress.DecryptConfig(s.cfg.AppSecret, cfg.DestConfigEnc)
	if err != nil {
		return fmt.Errorf("export job: decrypt config: %w", err)
	}

	dest, err := export.NewDestination(cfg.DestType, configJSON)
	if err != nil {
		return fmt.Errorf("export job: build destination: %w", err)
	}

	run, err := runRepo.Get(ctx, job.WorkspaceID, p.ExportRunID)
	if err != nil {
		return fmt.Errorf("export job: load run %s: %w", p.ExportRunID, err)
	}

	if err := runRepo.Start(ctx, run.ID); err != nil {
		slog.WarnContext(ctx, "export job: mark run started", "error", err)
	}

	project, err := s.db.GetProjectByID(ctx, dbgen.GetProjectByIDParams{
		WorkspaceID: cfg.WorkspaceID,
		ID:          cfg.ProjectID,
	})
	if err != nil {
		_ = finishRunFailed(ctx, runRepo, configRepo, run.ID, cfg.ID, fmt.Sprintf("load project: %s", err))
		return fmt.Errorf("export job: load project: %w", err)
	}

	result, buildErr := export.Build(ctx, export.BuildParams{
		Config:  cfg,
		Run:     run,
		Dest:    dest,
		Storage: s.storage,
		Queries: s.db,
		SQLite:  s.sqlDB,
		Project: project,
		OnProgress: func(prog export.BuildProgress) {
			_ = runRepo.UpdateProgress(ctx, run.ID, repository.ExportProgress{
				AssetsExported: prog.AssetsExported,
				AssetsSkipped:  prog.AssetsSkipped,
				BytesWritten:   prog.BytesWritten,
			})
		},
	})

	if buildErr != nil {
		_ = finishRunFailed(ctx, runRepo, configRepo, run.ID, cfg.ID, buildErr.Error())
		return fmt.Errorf("export job: build: %w", buildErr)
	}

	configStatus := "ok"
	if result.AssetsExported > 0 && result.AssetsSkipped > 0 {
		configStatus = "partial"
	}
	status := "done"

	finish := repository.ExportFinish{
		Status:         status,
		AssetsTotal:    result.AssetsTotal,
		AssetsExported: result.AssetsExported,
		AssetsSkipped:  result.AssetsSkipped,
		BytesWritten:   result.BytesWritten,
	}
	if err := runRepo.Finish(ctx, run.ID, finish); err != nil {
		slog.WarnContext(ctx, "export job: finish run", "error", err)
	}

	now := time.Now()
	if err := configRepo.SetLastRun(ctx, cfg.ID, repository.ExportRunResult{
		LastRunAt:     now,
		LastRunStatus: configStatus,
	}); err != nil {
		slog.WarnContext(ctx, "export job: set last run", "error", err)
	}

	return nil
}

func finishRunFailed(ctx context.Context, runRepo repository.ExportRunRepository, configRepo repository.ExportConfigRepository, runID, configID, errMsg string) error {
	finish := repository.ExportFinish{
		Status: "failed",
		Error:  &errMsg,
	}
	_ = runRepo.Finish(ctx, runID, finish)
	now := time.Now()
	_ = configRepo.SetLastRun(ctx, configID, repository.ExportRunResult{
		LastRunAt:     now,
		LastRunStatus: "failed",
		LastError:     &errMsg,
	})
	return nil
}

