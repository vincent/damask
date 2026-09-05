//go:build demo

package main

import (
	"context"
	"log/slog"
	"os"

	"database/sql"

	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/demo"
	"damask/server/internal/queue"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"
)

func initDemoSeeder(
	ctx context.Context,
	cfg *config.Config,
	sqlDB *sql.DB,
	stor storage.Storage,
	trf transform.Transformer,
	tmb transform.Thumbnailer,
	q queue.JobQueue,
	queries *dbgen.Queries,
) *demo.Seeder {
	if !cfg.Demo.DemoMode {
		return nil
	}
	vs := visualsimilarity.NewService(queries, sqlDB)
	seeder := demo.New(sqlDB, stor, cfg.Demo, trf, tmb, q, vs)
	if err := seeder.EnsureWorkspace(ctx); err != nil {
		slog.ErrorContext(ctx, "demo: ensure workspace", "error", err)
		os.Exit(1)
	}
	if err := seeder.SeedIfEmpty(ctx); err != nil {
		slog.WarnContext(ctx, "demo: initial seed failed (non-fatal)", "error", err)
	}
	seeder.StartResetLoop(ctx)
	slog.InfoContext(ctx, "demo: mode enabled", "reset_interval_hours", cfg.Demo.ResetIntervalHours)
	return seeder
}
