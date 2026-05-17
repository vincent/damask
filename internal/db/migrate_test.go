package db

import (
	"database/sql"
	"testing"

	_ "modernc.org/sqlite"
)

func TestRunMigrations_FieldDefinitionsOrphanedCreatedBy(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, stmt := range []string{
		`PRAGMA foreign_keys=OFF`,
		`CREATE TABLE schema_migrations (version uint64, dirty bool)`,
		`INSERT INTO schema_migrations (version, dirty) VALUES (33, 0)`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL, password_hash TEXT NOT NULL, name TEXT NOT NULL)`,
		`CREATE TABLE field_definitions (
			id TEXT PRIMARY KEY,
			workspace_id TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			created_by TEXT NOT NULL REFERENCES users(id),
			scope TEXT NOT NULL CHECK(scope IN ('asset', 'project')),
			name TEXT NOT NULL,
			key TEXT NOT NULL,
			field_type TEXT NOT NULL CHECK(field_type IN ('text','number','date','boolean','select','url')),
			options TEXT,
			required INTEGER NOT NULL DEFAULT 0,
			position INTEGER NOT NULL DEFAULT 0,
			inherit_from_project INTEGER NOT NULL DEFAULT 0,
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			deleted_at TEXT,
			UNIQUE(workspace_id, scope, key)
		)`,
		`CREATE INDEX idx_field_defs_workspace ON field_definitions(workspace_id, scope)`,
		`CREATE INDEX idx_field_defs_active ON field_definitions(workspace_id, deleted_at)`,
		`CREATE TABLE assets (id TEXT PRIMARY KEY)`,
		`CREATE TABLE asset_field_values (
			id TEXT PRIMARY KEY,
			asset_id TEXT NOT NULL REFERENCES assets(id) ON DELETE CASCADE,
			field_id TEXT NOT NULL REFERENCES field_definitions(id),
			value_text TEXT,
			value_number REAL,
			value_date TEXT,
			value_boolean INTEGER,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(asset_id, field_id)
		)`,
		`CREATE INDEX idx_afv_asset ON asset_field_values(asset_id)`,
		`CREATE INDEX idx_afv_field ON asset_field_values(field_id)`,
		`CREATE INDEX idx_afv_text ON asset_field_values(field_id, value_text)`,
		`CREATE INDEX idx_afv_number ON asset_field_values(field_id, value_number)`,
		`CREATE INDEX idx_afv_date ON asset_field_values(field_id, value_date)`,
		`CREATE INDEX idx_afv_boolean ON asset_field_values(field_id, value_boolean)`,
		`CREATE TABLE projects (id TEXT PRIMARY KEY)`,
		`CREATE TABLE project_field_values (
			id TEXT PRIMARY KEY,
			project_id TEXT NOT NULL REFERENCES projects(id) ON DELETE CASCADE,
			field_id TEXT NOT NULL REFERENCES field_definitions(id),
			value_text TEXT,
			value_number REAL,
			value_date TEXT,
			value_boolean INTEGER,
			created_by TEXT NOT NULL REFERENCES users(id),
			created_at TEXT NOT NULL DEFAULT (datetime('now')),
			updated_at TEXT NOT NULL DEFAULT (datetime('now')),
			UNIQUE(project_id, field_id)
		)`,
		`CREATE INDEX idx_pfv_project ON project_field_values(project_id)`,
		`CREATE INDEX idx_pfv_field ON project_field_values(field_id)`,
		`CREATE INDEX idx_pfv_text ON project_field_values(field_id, value_text)`,
		`CREATE INDEX idx_pfv_number ON project_field_values(field_id, value_number)`,
		`CREATE INDEX idx_pfv_date ON project_field_values(field_id, value_date)`,
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Workspace')`,
		`INSERT INTO users (id, email, password_hash, name) VALUES ('user1', 'u@example.com', 'hash', 'User')`,
		`INSERT INTO field_definitions (id, workspace_id, created_by, scope, name, key, field_type)
		 VALUES ('fd1', 'ws1', 'missing-user', 'asset', 'Camera maker', '_exif_make', 'text')`,
		`INSERT INTO assets (id) VALUES ('asset1')`,
		`INSERT INTO asset_field_values (id, asset_id, field_id, value_text, created_by)
		 VALUES ('afv1', 'asset1', 'fd1', 'Canon', 'user1')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("seed legacy schema: %v\nstmt: %s", err, stmt)
		}
	}
	if _, err := sqlDB.Exec(`PRAGMA foreign_keys=ON`); err != nil {
		t.Fatalf("enable foreign keys: %v", err)
	}

	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	var source string
	var createdBy sql.NullString
	if err := sqlDB.QueryRow(`SELECT source, created_by FROM field_definitions WHERE id = 'fd1'`).Scan(&source, &createdBy); err != nil {
		t.Fatalf("read migrated row: %v", err)
	}
	if source != "exif" {
		t.Fatalf("source = %q, want exif", source)
	}
	if createdBy.Valid {
		t.Fatalf("created_by = %q, want NULL", createdBy.String)
	}

	var afvCount int
	if err := sqlDB.QueryRow(`SELECT COUNT(*) FROM asset_field_values WHERE field_id = 'fd1'`).Scan(&afvCount); err != nil {
		t.Fatalf("read asset_field_values: %v", err)
	}
	if afvCount != 1 {
		t.Fatalf("asset_field_values count = %d, want 1", afvCount)
	}
}

func TestRunMigrations_DisableExistingWorkflows(t *testing.T) {
	sqlDB, err := sql.Open("sqlite", ":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	for _, stmt := range []string{
		`CREATE TABLE schema_migrations (version uint64, dirty bool)`,
		`INSERT INTO schema_migrations (version, dirty) VALUES (38, 0)`,
		`CREATE TABLE workspaces (id TEXT PRIMARY KEY, name TEXT NOT NULL)`,
		`CREATE TABLE users (id TEXT PRIMARY KEY, email TEXT NOT NULL, password_hash TEXT NOT NULL, name TEXT NOT NULL)`,
		`CREATE TABLE workflows (
			id                      TEXT PRIMARY KEY,
			workspace_id            TEXT NOT NULL REFERENCES workspaces(id) ON DELETE CASCADE,
			name                    TEXT NOT NULL,
			description             TEXT NOT NULL DEFAULT '',
			enabled                 INTEGER NOT NULL DEFAULT 1,
			trigger_type            TEXT NOT NULL,
			graph                   TEXT NOT NULL,
			notify_on_failure_email TEXT NOT NULL DEFAULT '',
			last_run_at             DATETIME,
			created_by              TEXT NOT NULL REFERENCES users(id),
			created_at              DATETIME NOT NULL DEFAULT (datetime('now')),
			updated_at              DATETIME NOT NULL DEFAULT (datetime('now'))
		)`,
		`INSERT INTO workspaces (id, name) VALUES ('ws1', 'Workspace 1')`,
		`INSERT INTO workspaces (id, name) VALUES ('ws2', 'Workspace 2')`,
		`INSERT INTO users (id, email, password_hash, name) VALUES ('user1', 'u1@example.com', 'hash', 'User 1')`,
		`INSERT INTO users (id, email, password_hash, name) VALUES ('user2', 'u2@example.com', 'hash', 'User 2')`,
		`INSERT INTO workflows (id, workspace_id, name, enabled, trigger_type, graph, created_by)
		 VALUES ('wf1', 'ws1', 'Enabled one', 1, 'trigger.manual', '{"nodes":[],"edges":[]}', 'user1')`,
		`INSERT INTO workflows (id, workspace_id, name, enabled, trigger_type, graph, created_by)
		 VALUES ('wf2', 'ws1', 'Already disabled', 0, 'trigger.manual', '{"nodes":[],"edges":[]}', 'user1')`,
		`INSERT INTO workflows (id, workspace_id, name, enabled, trigger_type, graph, created_by)
		 VALUES ('wf3', 'ws2', 'Enabled two', 1, 'trigger.asset_created', '{"nodes":[],"edges":[]}', 'user2')`,
	} {
		if _, err := sqlDB.Exec(stmt); err != nil {
			t.Fatalf("seed workflow schema: %v\nstmt: %s", err, stmt)
		}
	}

	if err := RunMigrations(sqlDB); err != nil {
		t.Fatalf("RunMigrations() error = %v", err)
	}

	rows, err := sqlDB.Query(`SELECT id, enabled FROM workflows ORDER BY id`)
	if err != nil {
		t.Fatalf("query workflows: %v", err)
	}
	defer rows.Close()

	got := map[string]int{}
	for rows.Next() {
		var id string
		var enabled int
		if err := rows.Scan(&id, &enabled); err != nil {
			t.Fatalf("scan workflow: %v", err)
		}
		got[id] = enabled
	}
	if err := rows.Err(); err != nil {
		t.Fatalf("iterate workflows: %v", err)
	}

	for _, id := range []string{"wf1", "wf2", "wf3"} {
		if got[id] != 0 {
			t.Fatalf("workflow %s enabled = %d, want 0", id, got[id])
		}
	}
}
