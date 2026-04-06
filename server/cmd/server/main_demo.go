//go:build demo

package main

import (
	"context"
	"log"

	"damask/server/internal/config"
	"damask/server/internal/demo"
	"damask/server/internal/storage"
	"database/sql"
)

func initDemoSeeder(ctx context.Context, cfg *config.Config, sqlDB *sql.DB, stor storage.Storage) *demo.Seeder {
	if !cfg.Demo.DemoMode {
		return nil
	}
	seeder := demo.New(sqlDB, stor, cfg.Demo)
	if err := seeder.EnsureWorkspace(ctx); err != nil {
		log.Fatalf("demo: ensure workspace: %v", err)
	}
	if err := seeder.SeedIfEmpty(ctx); err != nil {
		log.Printf("demo: initial seed failed (non-fatal): %v", err)
	}
	seeder.StartResetLoop(ctx)
	log.Printf("demo: mode enabled reset_interval=%dh", cfg.Demo.ResetIntervalHours)
	return seeder
}
