package service

import (
	"context"
	"database/sql"
	"os"
	"path/filepath"
	"testing"

	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/mediatype"
	"damask/server/internal/transform"

	"github.com/google/uuid"
)

func newTestInjestorImpl(t *testing.T) (*injestorImpl, *sql.DB) {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	impl := &injestorImpl{
		db:    queries,
		sqlDB: sqlDB,
		media: mediatype.NewRegistry(transform.NewTransformer()),
	}
	return impl, sqlDB
}

// TestCreateInitialVersionWithNoUser verifies that when createInitialVersion
// is called without a userID, the created_by field is NULL in the database
// (representing a system action, e.g., ingress-created asset).
func TestCreateInitialVersionWithNoUser(t *testing.T) {
	impl, sqlDB := newTestInjestorImpl(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()

	workspaceID := uuid.NewString()
	_, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, workspaceID, "Test Workspace")
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	assetID := uuid.NewString()
	asset := dbgen.Asset{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		Size:             100,
		MimeType:         "text/plain",
		OriginalFilename: "test.txt",
	}
	_, err = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		asset.ID, asset.WorkspaceID, asset.OriginalFilename, "test-key", asset.MimeType, asset.Size,
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	versionID, err := impl.createInitialVersion(ctx, asset, tmpFile, "test-storage-key", "text/plain", mediatype.FileMeta{}, "")
	if err != nil {
		t.Fatalf("createInitialVersion: %v", err)
	}

	var createdBy *string
	if err := sqlDB.QueryRow(`SELECT created_by FROM asset_versions WHERE id = ?`, versionID).Scan(&createdBy); err != nil {
		t.Fatalf("query created_by: %v", err)
	}
	if createdBy != nil {
		t.Errorf("expected created_by to be NULL, got: %v", *createdBy)
	}
}

// TestCreateInitialVersionWithUser verifies that when createInitialVersion
// is called with a userID, it correctly stores that user's ID as created_by.
func TestCreateInitialVersionWithUser(t *testing.T) {
	impl, sqlDB := newTestInjestorImpl(t)
	t.Cleanup(func() { _ = sqlDB.Close() })

	ctx := context.Background()

	workspaceID := uuid.NewString()
	_, err := sqlDB.Exec(`INSERT INTO workspaces (id, name) VALUES (?, ?)`, workspaceID, "Test Workspace")
	if err != nil {
		t.Fatalf("insert workspace: %v", err)
	}

	userID := uuid.NewString()
	_, err = sqlDB.Exec(
		`INSERT INTO users (id, email, password_hash, name) VALUES (?, ?, ?, ?)`,
		userID, "user@example.com", "hash", "Test User",
	)
	if err != nil {
		t.Fatalf("insert user: %v", err)
	}

	assetID := uuid.NewString()
	asset := dbgen.Asset{
		ID:               assetID,
		WorkspaceID:      workspaceID,
		Size:             100,
		MimeType:         "text/plain",
		OriginalFilename: "test.txt",
	}
	_, err = sqlDB.Exec(
		`INSERT INTO assets (id, workspace_id, original_filename, storage_key, mime_type, size) VALUES (?, ?, ?, ?, ?, ?)`,
		asset.ID, asset.WorkspaceID, asset.OriginalFilename, "test-key", asset.MimeType, asset.Size,
	)
	if err != nil {
		t.Fatalf("insert asset: %v", err)
	}

	tmpFile := filepath.Join(t.TempDir(), "test.txt")
	if err := os.WriteFile(tmpFile, []byte("hello world"), 0644); err != nil {
		t.Fatalf("write temp file: %v", err)
	}

	versionID, err := impl.createInitialVersion(ctx, asset, tmpFile, "test-storage-key", "text/plain", mediatype.FileMeta{}, userID)
	if err != nil {
		t.Fatalf("createInitialVersion: %v", err)
	}

	var createdBy *string
	if err := sqlDB.QueryRow(`SELECT created_by FROM asset_versions WHERE id = ?`, versionID).Scan(&createdBy); err != nil {
		t.Fatalf("query created_by: %v", err)
	}
	if createdBy == nil {
		t.Error("expected created_by to not be NULL")
	} else if *createdBy != userID {
		t.Errorf("expected created_by to be %q, got %q", userID, *createdBy)
	}
}
