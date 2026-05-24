package service

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"log/slog"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/export"
	"damask/server/internal/ingress"
	"damask/server/internal/queue"
	"damask/server/internal/repository"
	reposqlc "damask/server/internal/repository/sqlc"

	"github.com/google/uuid"
)

type exportService struct {
	db         *dbgen.Queries
	appSecret  string
	q          queue.JobQueue
	configRepo repository.ExportConfigRepository
	runRepo    repository.ExportRunRepository
}

// NewExportService creates a production ExportService with sqlc-backed repos.
func NewExportService(db *dbgen.Queries, sqlDB *sql.DB, appSecret string, q queue.JobQueue) ExportService {
	return &exportService{
		db:         db,
		appSecret:  appSecret,
		q:          q,
		configRepo: reposqlc.NewExportConfigRepo(db, sqlDB),
		runRepo:    reposqlc.NewExportRunRepo(db, sqlDB),
	}
}

// NewExportServiceWithRepos creates an ExportService with explicit repos (for tests).
func NewExportServiceWithRepos(
	db *dbgen.Queries,
	appSecret string,
	q queue.JobQueue,
	configRepo repository.ExportConfigRepository,
	runRepo repository.ExportRunRepository,
) ExportService {
	return &exportService{
		db:         db,
		appSecret:  appSecret,
		q:          q,
		configRepo: configRepo,
		runRepo:    runRepo,
	}
}

// stampWorkspaceID injects workspace_id into a dest_config JSON blob so that
// destinations (e.g. gdrive) can look up the OAuth connection at job time,
// when the request context no longer carries the workspace.
func stampWorkspaceID(cfg json.RawMessage, workspaceID string) (json.RawMessage, error) {
	var m map[string]any
	if err := json.Unmarshal(cfg, &m); err != nil {
		return cfg, fmt.Errorf("dest_config: unmarshal: %w", err)
	}
	m["workspace_id"] = workspaceID
	out, err := json.Marshal(m)
	if err != nil {
		return cfg, fmt.Errorf("dest_config: re-marshal: %w", err)
	}
	return out, nil
}

func (s *exportService) Create(ctx context.Context, workspaceID, userID string, p CreateExportConfigParams) (*ExportConfigDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	stamped, err := stampWorkspaceID(p.DestConfig, workspaceID)
	if err != nil {
		return nil, fmt.Errorf("export: stamp workspace: %w", err)
	}
	enc, err := ingress.EncryptConfig(s.appSecret, stamped)
	if err != nil {
		return nil, fmt.Errorf("export: encrypt dest config: %w", err)
	}
	cfg := repository.ExportConfig{
		ID:              uuid.NewString(),
		WorkspaceID:     workspaceID,
		ProjectID:       p.ProjectID,
		CreatedBy:       userID,
		Label:           p.Label,
		DestType:        p.DestType,
		DestConfigEnc:   enc,
		Versions:        p.Versions,
		IncludeVariants: p.IncludeVariants,
		ScheduleType:    p.ScheduleType,
		QuietMinutes:    p.QuietMinutes,
		Enabled:         true,
		CreatedAt:       time.Now(),
		UpdatedAt:       time.Now(),
	}
	created, err := s.configRepo.Create(ctx, cfg)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(created), nil
}

func (s *exportService) Get(ctx context.Context, workspaceID, id string) (*ExportConfigDTO, error) {
	cfg, err := s.configRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(cfg), nil
}

func (s *exportService) List(ctx context.Context, workspaceID string) ([]*ExportConfigDTO, error) {
	cfgs, err := s.configRepo.List(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]*ExportConfigDTO, len(cfgs))
	for i, c := range cfgs {
		out[i] = exportConfigToDTO(c)
	}
	return out, nil
}

func (s *exportService) ListByProject(ctx context.Context, workspaceID, projectID string) ([]*ExportConfigDTO, error) {
	cfgs, err := s.configRepo.ListByProject(ctx, workspaceID, projectID)
	if err != nil {
		return nil, err
	}
	out := make([]*ExportConfigDTO, len(cfgs))
	for i, c := range cfgs {
		out[i] = exportConfigToDTO(c)
	}
	return out, nil
}

func (s *exportService) Update(ctx context.Context, workspaceID, id string, p UpdateExportConfigParams) (*ExportConfigDTO, error) {
	if err := p.Validate(); err != nil {
		return nil, err
	}
	existing, err := s.configRepo.Get(ctx, workspaceID, id)
	if err != nil {
		return nil, err
	}
	enc := existing.DestConfigEnc
	if len(p.DestConfig) > 0 {
		stamped, stampErr := stampWorkspaceID(p.DestConfig, workspaceID)
		if stampErr != nil {
			return nil, fmt.Errorf("export: stamp workspace: %w", stampErr)
		}
		enc, err = ingress.EncryptConfig(s.appSecret, stamped)
		if err != nil {
			return nil, fmt.Errorf("export: encrypt dest config: %w", err)
		}
	}
	existing.Label = p.Label
	existing.DestType = p.DestType
	existing.DestConfigEnc = enc
	existing.Versions = p.Versions
	existing.IncludeVariants = p.IncludeVariants
	existing.ScheduleType = p.ScheduleType
	existing.QuietMinutes = p.QuietMinutes
	existing.Enabled = p.Enabled
	existing.UpdatedAt = time.Now()

	updated, err := s.configRepo.Update(ctx, existing)
	if err != nil {
		return nil, err
	}
	return exportConfigToDTO(updated), nil
}

func (s *exportService) Delete(ctx context.Context, workspaceID, id string) error {
	return s.configRepo.Delete(ctx, workspaceID, id)
}

func (s *exportService) ValidateDestination(ctx context.Context, workspaceID, configID string) error {
	cfg, err := s.configRepo.Get(ctx, workspaceID, configID)
	if err != nil {
		return err
	}
	configJSON, err := ingress.DecryptConfig(s.appSecret, cfg.DestConfigEnc)
	if err != nil {
		slog.ErrorContext(ctx, "export: decrypt dest config", "error", err)
		return fmt.Errorf("export: decrypt dest config: %w", err)
	}
	dest, err := export.NewDestination(cfg.DestType, configJSON)
	if err != nil {
		slog.ErrorContext(ctx, "export: decrypt new dest", "error", err)
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	return dest.Validate(ctx)
}

func (s *exportService) ValidateDestinationConfig(ctx context.Context, workspaceID, destType string, destConfig json.RawMessage) error {
	stamped, err := stampWorkspaceID(destConfig, workspaceID)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	dest, err := export.NewDestination(destType, stamped)
	if err != nil {
		return fmt.Errorf("%w: %s", apperr.ErrInvalidInput, err.Error())
	}
	return dest.Validate(ctx)
}

func (s *exportService) TriggerManual(ctx context.Context, workspaceID, userID, configID string) (*ExportRunDTO, error) {
	cfg, err := s.configRepo.Get(ctx, workspaceID, configID)
	if err != nil {
		return nil, err
	}
	run := repository.ExportRun{
		ID:             uuid.NewString(),
		ExportConfigID: cfg.ID,
		WorkspaceID:    workspaceID,
		TriggeredBy:    &userID,
		Status:         "pending",
		CreatedAt:      time.Now(),
	}
	created, err := s.runRepo.Create(ctx, run)
	if err != nil {
		return nil, err
	}
	payload := fmt.Sprintf(`{"export_config_id":%q,"export_run_id":%q}`, cfg.ID, created.ID)
	if _, err := s.q.Enqueue(ctx, workspaceID, queue.JobTypeExportRun, payload); err != nil {
		return nil, fmt.Errorf("export: enqueue job: %w", err)
	}
	return exportRunToDTO(created), nil
}

func (s *exportService) GetRun(ctx context.Context, workspaceID, runID string) (*ExportRunDTO, error) {
	run, err := s.runRepo.Get(ctx, workspaceID, runID)
	if err != nil {
		return nil, err
	}
	return exportRunToDTO(run), nil
}

func (s *exportService) ListRuns(ctx context.Context, workspaceID, configID string, limit, offset int) ([]*ExportRunDTO, error) {
	if _, err := s.configRepo.Get(ctx, workspaceID, configID); err != nil {
		return nil, err
	}
	runs, err := s.runRepo.List(ctx, configID, limit, offset)
	if err != nil {
		return nil, err
	}
	out := make([]*ExportRunDTO, len(runs))
	for i, r := range runs {
		out[i] = exportRunToDTO(r)
	}
	return out, nil
}

func exportConfigToDTO(c repository.ExportConfig) *ExportConfigDTO {
	return &ExportConfigDTO{
		ID:              c.ID,
		WorkspaceID:     c.WorkspaceID,
		ProjectID:       c.ProjectID,
		Label:           c.Label,
		DestType:        c.DestType,
		Versions:        c.Versions,
		IncludeVariants: c.IncludeVariants,
		ScheduleType:    c.ScheduleType,
		QuietMinutes:    c.QuietMinutes,
		Enabled:         c.Enabled,
		LastRunAt:       c.LastRunAt,
		LastRunStatus:   c.LastRunStatus,
		LastError:       c.LastError,
		CreatedAt:       c.CreatedAt,
		UpdatedAt:       c.UpdatedAt,
	}
}

func exportRunToDTO(r repository.ExportRun) *ExportRunDTO {
	return &ExportRunDTO{
		ID:             r.ID,
		ExportConfigID: r.ExportConfigID,
		TriggeredBy:    r.TriggeredBy,
		Status:         r.Status,
		AssetsTotal:    r.AssetsTotal,
		AssetsExported: r.AssetsExported,
		AssetsSkipped:  r.AssetsSkipped,
		BytesWritten:   r.BytesWritten,
		Error:          r.Error,
		StartedAt:      r.StartedAt,
		CompletedAt:    r.CompletedAt,
		CreatedAt:      r.CreatedAt,
	}
}
