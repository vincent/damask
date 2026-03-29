package db

import (
	db "badam/server/internal/db/gen"
	"database/sql"
	"fmt"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database, runs migrations, and returns a Queries instance.
func Open(dbPath string) (*db.Queries, *sql.DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL`); err != nil {
		return nil, nil, fmt.Errorf("set WAL mode: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		return nil, nil, fmt.Errorf("enable foreign keys: %w", err)
	}
	if _, err := sqlDB.Exec(`PRAGMA busy_timeout=5000`); err != nil {
		return nil, nil, fmt.Errorf("set busy timeout: %w", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	// SQLite supports one writer at a time; use a single connection to
	// prevent SQLITE_BUSY races between concurrent goroutines (e.g. thumbnail worker).
	sqlDB.SetMaxOpenConns(1)

	return db.New(sqlDB), sqlDB, nil
}
