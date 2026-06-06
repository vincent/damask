// Package db manages the SQLite database connection and migrations.
package db

import (
	"context"
	"database/sql"
	"fmt"
	"path/filepath"

	db "damask/server/internal/db/gen"

	"github.com/uptrace/opentelemetry-go-extra/otelsql"
	semconv "go.opentelemetry.io/otel/semconv/v1.26.0"
	_ "modernc.org/sqlite" //nolint:nolintlint // to register the sqlite driver
)

// Open opens the SQLite database, runs migrations, and returns a Queries instance.
func Open(dbPath string) (*db.Queries, *sql.DB, error) {
	sqlDB, err := otelsql.Open("sqlite", dbPath,
		otelsql.WithAttributes(semconv.DBSystemSqlite),
		otelsql.WithDBName(filepath.Base(dbPath)),
	)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	ctx := context.Background()

	if _, err = sqlDB.ExecContext(ctx, `PRAGMA journal_mode=WAL`); err != nil {
		return nil, nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err = sqlDB.ExecContext(ctx, `PRAGMA foreign_keys=ON`); err != nil {
		return nil, nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err = sqlDB.ExecContext(ctx, `PRAGMA busy_timeout=5000`); err != nil {
		return nil, nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if err = RunMigrations(sqlDB); err != nil {
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	// SQLite supports one writer at a time; use a single connection to
	// prevent SQLITE_BUSY races between concurrent goroutines (e.g. thumbnail worker).
	sqlDB.SetMaxOpenConns(1)

	return db.New(sqlDB), sqlDB, nil
}
