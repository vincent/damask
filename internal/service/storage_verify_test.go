package service

import (
	"context"
	"log/slog"
	"strings"
	"testing"

	dbpkg "damask/server/internal/db"
)

func TestVerifySizeColumns(t *testing.T) {
	ctx := context.Background()
	_, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })

	// Insert a workspace, user, project, and asset_version with size=0.
	wsID := "ws-verify"
	if _, err := sqlDB.ExecContext(ctx, `INSERT INTO workspaces (id, name) VALUES (?, ?)`, wsID, "test"); err != nil {
		t.Fatalf("insert workspace: %v", err)
	}
	userID := "user-verify"
	if _, err := sqlDB.ExecContext(
		ctx,
		`INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, '', 'Test')`,
		userID,
		"v@test.com",
	); err != nil {
		t.Fatalf("insert user: %v", err)
	}
	assetID := "asset-verify"
	if _, err := sqlDB.ExecContext(
		ctx,
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, 'f.jpg', 'k', 'image/jpeg', 0)`,
		assetID,
		wsID,
	); err != nil {
		t.Fatalf("insert asset: %v", err)
	}
	if _, err := sqlDB.ExecContext(
		ctx,
		`INSERT INTO asset_versions (id, workspace_id, asset_id, storage_key, content_hash, version_num, mime_type, size, created_by)
		 VALUES ('ver1', ?, ?, 'k', 'abc', 1, 'image/jpeg', 0, ?)`,
		wsID,
		assetID,
		userID,
	); err != nil {
		t.Fatalf("insert asset_version: %v", err)
	}

	// Capture log output
	var buf strings.Builder
	handler := slog.NewTextHandler(&buf, &slog.HandlerOptions{Level: slog.LevelWarn})
	logger := slog.New(handler)

	VerifySizeColumns(ctx, sqlDB, logger)

	if !strings.Contains(buf.String(), "asset_versions with size=0") {
		t.Errorf("expected warning about asset_versions, got: %s", buf.String())
	}
}
