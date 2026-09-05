//go:build demo

package demo_test

import (
	"context"
	"testing"
)

func TestWipe_ClearsContentButKeepsWorkspaceAndUser(t *testing.T) {
	seeder, db, stor := newTestSeeder(t)
	ctx := context.Background()

	if err := seeder.Seed(ctx); err != nil {
		t.Fatalf("seed: %v", err)
	}

	workspaceID, ok := seeder.GetWorkspaceID(ctx)
	if !ok {
		t.Fatal("expected demo workspace to exist after seed")
	}
	userID, _, err := seeder.GetDemoUser(ctx)
	if err != nil {
		t.Fatalf("get demo user before wipe: %v", err)
	}

	assetCountBefore, _, err := seeder.GetUsage(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get usage before wipe: %v", err)
	}
	if assetCountBefore == 0 {
		t.Fatal("expected assets to exist before wipe")
	}

	if err := seeder.Wipe(ctx); err != nil {
		t.Fatalf("wipe: %v", err)
	}

	// Workspace and user rows survive a wipe so existing JWTs stay valid.
	if _, ok := seeder.GetWorkspaceID(ctx); !ok {
		t.Fatal("expected demo workspace to still exist after wipe")
	}
	var stillExists string
	if err := db.QueryRowContext(ctx, `SELECT id FROM users WHERE id = ?`, userID).Scan(&stillExists); err != nil {
		t.Fatalf("expected demo user to survive wipe: %v", err)
	}

	assetCountAfter, storageUsedAfter, err := seeder.GetUsage(ctx, workspaceID)
	if err != nil {
		t.Fatalf("get usage after wipe: %v", err)
	}
	if assetCountAfter != 0 {
		t.Fatalf("expected 0 assets after wipe, got %d", assetCountAfter)
	}
	if storageUsedAfter != 0 {
		t.Fatalf("expected 0 storage usage after wipe, got %d", storageUsedAfter)
	}

	var projectCount int
	if err := db.QueryRowContext(ctx, `SELECT COUNT(*) FROM projects WHERE workspace_id = ?`, workspaceID).
		Scan(&projectCount); err != nil {
		t.Fatalf("count projects: %v", err)
	}
	if projectCount != 0 {
		t.Fatalf("expected 0 projects after wipe, got %d", projectCount)
	}

	// The whole demo/{workspaceID} storage prefix should be gone, including
	// the larger real photo/video files (not just the old tiny stubs).
	keys, err := stor.List(ctx, "demo/"+workspaceID)
	if err != nil {
		t.Fatalf("list storage after wipe: %v", err)
	}
	if len(keys) != 0 {
		t.Fatalf("expected no storage keys under demo/%s after wipe, got %d", workspaceID, len(keys))
	}
}
