//go:build !demo

package main

import (
	"context"

	"damask/server/internal/api"
	"damask/server/internal/config"
	"damask/server/internal/storage"
	"database/sql"
)

func initDemoSeeder(_ context.Context, _ *config.Config, _ *sql.DB, _ storage.Storage) api.DemoSeeder {
	return nil
}
