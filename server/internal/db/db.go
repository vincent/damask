package db

import (
	"database/sql"
	"fmt"

	dbgen "creativo-dam/server/internal/db/gen"

	_ "modernc.org/sqlite"
)

// Open opens the SQLite database, runs migrations, and returns a Queries instance.
func Open(dbPath string) (*dbgen.Queries, *sql.DB, error) {
	sqlDB, err := sql.Open("sqlite", dbPath)
	if err != nil {
		return nil, nil, fmt.Errorf("open db: %w", err)
	}

	// Enable WAL mode and foreign keys
	if _, err := sqlDB.Exec(`PRAGMA journal_mode=WAL; PRAGMA foreign_keys=ON;`); err != nil {
		return nil, nil, fmt.Errorf("configure db pragmas: %w", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		return nil, nil, fmt.Errorf("run migrations: %w", err)
	}

	return dbgen.New(sqlDB), sqlDB, nil
}
