//go:build demo

package demo_test

import (
	"context"
	"database/sql"
	"io"
	"testing"

	"damask/server/internal/config"
	dbpkg "damask/server/internal/db"
	"damask/server/internal/demo"
	"damask/server/internal/storage"
	"damask/server/internal/transform"
	"damask/server/internal/visualsimilarity"
)

func newTestSeeder(t *testing.T) (*demo.Seeder, *sql.DB, storage.Storage) {
	t.Helper()

	database, err := dbpkg.Open(":memory:")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { database.Close() })

	stor, err := storage.NewAferoMemoryStorage()
	if err != nil {
		t.Fatalf("storage: %v", err)
	}

	trf := transform.NewTransformer()
	tmb := transform.NewThumbnailer(trf)

	cfg := config.DemoConfig{
		DemoMode:      true,
		UserEmail:     "demo@damask.studio",
		WorkspaceName: "Demo Agency",
	}

	vs := visualsimilarity.NewService(database.WQ, database.Writer)
	seeder := demo.New(database.Writer, stor, cfg, trf, tmb, nil, vs)
	if err := seeder.EnsureWorkspace(context.Background()); err != nil {
		t.Fatalf("ensure workspace: %v", err)
	}
	return seeder, database.Writer, stor
}

func TestSeed_CreatesAssetsFromManifest(t *testing.T) {
	seeder, db, _ := newTestSeeder(t)
	ctx := context.Background()

	if err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	workspaceID, ok := seeder.GetWorkspaceID(ctx)
	if !ok {
		t.Fatal("expected demo workspace to exist after seed")
	}

	assetCount, storageUsed, err := seeder.GetUsage(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get usage: %v", err)
	}
	if assetCount == 0 {
		t.Fatal("expected at least one asset after seed")
	}
	if storageUsed == 0 {
		t.Fatal("expected non-zero storage usage after seed")
	}

	// Every manifest-declared tag must actually exist and be attached to at
	// least one asset (RA-4): spot-check "hero" and "video".
	for _, tag := range []string{"hero", "video", "brand"} {
		var count int
		err := db.QueryRowContext(ctx, `
			SELECT COUNT(*) FROM asset_tags at
			JOIN tags t ON t.id = at.tag_id
			WHERE t.workspace_id = ? AND t.name = ?
		`, workspaceID, tag).Scan(&count)
		if err != nil {
			t.Fatalf("count tag %q: %v", tag, err)
		}
		if count == 0 {
			t.Errorf("expected at least one asset tagged %q", tag)
		}
	}
}

func TestSeed_VersionedRealPhotoProducesDistinctVersions(t *testing.T) {
	seeder, db, stor := newTestSeeder(t)
	ctx := context.Background()

	if err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	var assetID string
	err := db.QueryRowContext(ctx, `SELECT id FROM assets WHERE original_filename = ?`, "hero-shot-beach.jpg").
		Scan(&assetID)
	if err != nil {
		t.Fatalf("find hero-shot-beach.jpg: %v", err)
	}

	rows, err := db.QueryContext(ctx, `
		SELECT version_num, content_hash, thumbnail_key FROM asset_versions
		WHERE asset_id = ? ORDER BY version_num
	`, assetID)
	if err != nil {
		t.Fatalf("query versions: %v", err)
	}
	defer rows.Close()

	type version struct {
		num          int
		contentHash  string
		thumbnailKey sql.NullString
	}
	var versions []version
	for rows.Next() {
		var v version
		if err := rows.Scan(&v.num, &v.contentHash, &v.thumbnailKey); err != nil {
			t.Fatalf("scan version: %v", err)
		}
		versions = append(versions, v)
	}
	if len(versions) != 2 {
		t.Fatalf("expected 2 versions for hero-shot-beach.jpg, got %d", len(versions))
	}
	if versions[0].contentHash == versions[1].contentHash {
		t.Fatal("expected version 1 (draft) and version 2 (final) to have different content hashes")
	}

	// Thumbnails should also differ, since they're generated from the
	// (visually distinct) crop/grayscale draft vs. the real final photo.
	hashThumb := func(key string) string {
		t.Helper()
		rc, err := stor.Get(ctx, key)
		if err != nil {
			t.Fatalf("get thumbnail %q: %v", key, err)
		}
		defer rc.Close()
		data, err := io.ReadAll(rc)
		if err != nil {
			t.Fatalf("read thumbnail %q: %v", key, err)
		}
		return string(data)
	}
	if !versions[0].thumbnailKey.Valid || !versions[1].thumbnailKey.Valid {
		t.Fatal("expected both versions to have a thumbnail")
	}
	if hashThumb(versions[0].thumbnailKey.String) == hashThumb(versions[1].thumbnailKey.String) {
		t.Fatal("expected draft and final thumbnails to have different bytes")
	}
}
