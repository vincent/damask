package reposqlc

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/apperr"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/repository"
)

// ---- ExportConfig repo ----

type exportConfigRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewExportConfigRepo creates a new sqlc-backed ExportConfigRepository.
func NewExportConfigRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.ExportConfigRepository {
	return &exportConfigRepo{q: q, sqlDB: sqlDB}
}

func (r *exportConfigRepo) Create(ctx context.Context, p repository.ExportConfig) (repository.ExportConfig, error) {
	var quietMinutes *int64
	if p.QuietMinutes != nil {
		v := int64(*p.QuietMinutes)
		quietMinutes = &v
	}
	row, err := r.q.CreateExportConfig(ctx, dbgen.CreateExportConfigParams{
		ID:              p.ID,
		WorkspaceID:     p.WorkspaceID,
		ProjectID:       p.ProjectID,
		CreatedBy:       p.CreatedBy,
		Label:           p.Label,
		DestType:        p.DestType,
		DestConfig:      p.DestConfigEnc,
		Versions:        p.Versions,
		IncludeVariants: boolToInt64(p.IncludeVariants),
		ScheduleType:    p.ScheduleType,
		QuietMinutes:    quietMinutes,
		Enabled:         boolToInt64(p.Enabled),
	})
	if err != nil {
		return repository.ExportConfig{}, err
	}
	return mapExportConfig(row), nil
}

func (r *exportConfigRepo) Get(ctx context.Context, workspaceID, id string) (repository.ExportConfig, error) {
	row, err := r.q.GetExportConfig(ctx, dbgen.GetExportConfigParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ExportConfig{}, apperr.ErrNotFound
		}
		return repository.ExportConfig{}, err
	}
	return mapExportConfig(row), nil
}

func (r *exportConfigRepo) List(ctx context.Context, workspaceID string) ([]repository.ExportConfig, error) {
	rows, err := r.q.ListExportConfigs(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	out := make([]repository.ExportConfig, len(rows))
	for i, row := range rows {
		out[i] = mapExportConfig(row)
	}
	return out, nil
}

func (r *exportConfigRepo) ListByProject(
	ctx context.Context,
	workspaceID, projectID string,
) ([]repository.ExportConfig, error) {
	rows, err := r.q.ListExportConfigsByProject(ctx, dbgen.ListExportConfigsByProjectParams{
		ProjectID:   projectID,
		WorkspaceID: workspaceID,
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.ExportConfig, len(rows))
	for i, row := range rows {
		out[i] = mapExportConfig(row)
	}
	return out, nil
}

func (r *exportConfigRepo) Update(ctx context.Context, p repository.ExportConfig) (repository.ExportConfig, error) {
	var quietMinutes *int64
	if p.QuietMinutes != nil {
		v := int64(*p.QuietMinutes)
		quietMinutes = &v
	}
	row, err := r.q.UpdateExportConfig(ctx, dbgen.UpdateExportConfigParams{
		ID:              p.ID,
		WorkspaceID:     p.WorkspaceID,
		Label:           p.Label,
		DestType:        p.DestType,
		DestConfig:      p.DestConfigEnc,
		Versions:        p.Versions,
		IncludeVariants: boolToInt64(p.IncludeVariants),
		ScheduleType:    p.ScheduleType,
		QuietMinutes:    quietMinutes,
		Enabled:         boolToInt64(p.Enabled),
	})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ExportConfig{}, apperr.ErrNotFound
		}
		return repository.ExportConfig{}, err
	}
	return mapExportConfig(row), nil
}

func (r *exportConfigRepo) Delete(ctx context.Context, workspaceID, id string) error {
	return r.q.DeleteExportConfig(ctx, dbgen.DeleteExportConfigParams{ID: id, WorkspaceID: workspaceID})
}

func (r *exportConfigRepo) SetLastRun(ctx context.Context, id string, p repository.ExportRunResult) error {
	return r.q.SetExportConfigLastRun(ctx, dbgen.SetExportConfigLastRunParams{
		ID:            id,
		LastRunAt:     &p.LastRunAt,
		LastRunStatus: &p.LastRunStatus,
		LastError:     p.LastError,
	})
}

func (r *exportConfigRepo) ListDue(ctx context.Context) ([]repository.ExportConfig, error) {
	rows, err := r.q.ListDueExportConfigs(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]repository.ExportConfig, len(rows))
	for i, row := range rows {
		out[i] = mapExportConfig(row)
	}
	return out, nil
}

// ---- ExportRun repo ----

type exportRunRepo struct {
	q     *dbgen.Queries
	sqlDB *sql.DB
}

// NewExportRunRepo creates a new sqlc-backed ExportRunRepository.
func NewExportRunRepo(q *dbgen.Queries, sqlDB *sql.DB) repository.ExportRunRepository {
	return &exportRunRepo{q: q, sqlDB: sqlDB}
}

func (r *exportRunRepo) Create(ctx context.Context, p repository.ExportRun) (repository.ExportRun, error) {
	row, err := r.q.CreateExportRun(ctx, dbgen.CreateExportRunParams{
		ID:             p.ID,
		ExportConfigID: p.ExportConfigID,
		WorkspaceID:    p.WorkspaceID,
		TriggeredBy:    p.TriggeredBy,
	})
	if err != nil {
		return repository.ExportRun{}, err
	}
	return mapExportRun(row), nil
}

func (r *exportRunRepo) Get(ctx context.Context, workspaceID, id string) (repository.ExportRun, error) {
	row, err := r.q.GetExportRun(ctx, dbgen.GetExportRunParams{ID: id, WorkspaceID: workspaceID})
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return repository.ExportRun{}, apperr.ErrNotFound
		}
		return repository.ExportRun{}, err
	}
	return mapExportRun(row), nil
}

func (r *exportRunRepo) List(ctx context.Context, configID string, limit, offset int) ([]repository.ExportRun, error) {
	rows, err := r.q.ListExportRuns(ctx, dbgen.ListExportRunsParams{
		ExportConfigID: configID,
		Limit:          int64(limit),
		Offset:         int64(offset),
	})
	if err != nil {
		return nil, err
	}
	out := make([]repository.ExportRun, len(rows))
	for i, row := range rows {
		out[i] = mapExportRun(row)
	}
	return out, nil
}

func (r *exportRunRepo) Start(ctx context.Context, id string) error {
	return r.q.StartExportRun(ctx, id)
}

func (r *exportRunRepo) UpdateProgress(ctx context.Context, id string, p repository.ExportProgress) error {
	return r.q.UpdateExportRunProgress(ctx, dbgen.UpdateExportRunProgressParams{
		ID:             id,
		AssetsExported: int64(p.AssetsExported),
		AssetsSkipped:  int64(p.AssetsSkipped),
		BytesWritten:   p.BytesWritten,
	})
}

func (r *exportRunRepo) Finish(ctx context.Context, id string, p repository.ExportFinish) error {
	return r.q.FinishExportRun(ctx, dbgen.FinishExportRunParams{
		ID:             id,
		Status:         p.Status,
		AssetsTotal:    int64(p.AssetsTotal),
		AssetsExported: int64(p.AssetsExported),
		AssetsSkipped:  int64(p.AssetsSkipped),
		BytesWritten:   p.BytesWritten,
		Error:          p.Error,
	})
}

// ---- mappers ----

func mapExportConfig(row dbgen.ExportConfig) repository.ExportConfig {
	var quietMinutes *int
	if row.QuietMinutes != nil {
		v := int(*row.QuietMinutes)
		quietMinutes = &v
	}
	return repository.ExportConfig{
		ID:              row.ID,
		WorkspaceID:     row.WorkspaceID,
		ProjectID:       row.ProjectID,
		CreatedBy:       row.CreatedBy,
		Label:           row.Label,
		DestType:        row.DestType,
		DestConfigEnc:   row.DestConfig,
		Versions:        row.Versions,
		IncludeVariants: row.IncludeVariants == 1,
		ScheduleType:    row.ScheduleType,
		QuietMinutes:    quietMinutes,
		Enabled:         row.Enabled == 1,
		LastRunAt:       row.LastRunAt,
		LastRunStatus:   row.LastRunStatus,
		LastError:       row.LastError,
		CreatedAt:       row.CreatedAt,
		UpdatedAt:       row.UpdatedAt,
	}
}

func mapExportRun(row dbgen.ExportRun) repository.ExportRun {
	return repository.ExportRun{
		ID:             row.ID,
		ExportConfigID: row.ExportConfigID,
		WorkspaceID:    row.WorkspaceID,
		TriggeredBy:    row.TriggeredBy,
		Status:         row.Status,
		AssetsTotal:    int(row.AssetsTotal),
		AssetsExported: int(row.AssetsExported),
		AssetsSkipped:  int(row.AssetsSkipped),
		BytesWritten:   row.BytesWritten,
		Error:          row.Error,
		StartedAt:      row.StartedAt,
		CompletedAt:    row.CompletedAt,
		CreatedAt:      row.CreatedAt,
	}
}

// Ensure time is used (for SetLastRun).
var _ = time.Now
