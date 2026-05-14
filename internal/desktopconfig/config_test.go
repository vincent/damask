package desktopconfig_test

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"damask/server/internal/desktopconfig"
)

func TestConfigDir_ReturnsNonEmpty(t *testing.T) {
	tmp := t.TempDir()
	sub := filepath.Join(tmp, "sub", "damask")
	dir, err := desktopconfig.ConfigDir(sub)
	if err != nil {
		t.Fatalf("ConfigDir: %v", err)
	}
	if dir == "" {
		t.Fatal("expected non-empty dir")
	}
	if _, err := os.Stat(dir); err != nil {
		t.Fatalf("dir not created: %v", err)
	}
}

func TestWrite_RoundTrip(t *testing.T) {
	tmp := t.TempDir()
	in := map[string]string{"PORT": "14000", "BASE_URL": "http://localhost:14000"}
	if err := desktopconfig.Write(tmp, in); err != nil {
		t.Fatalf("Write: %v", err)
	}
	got, err := desktopconfig.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	for k, want := range in {
		if got[k] != want {
			t.Errorf("key %q: got %q want %q", k, got[k], want)
		}
	}
}

func TestWrite_MergesExisting(t *testing.T) {
	tmp := t.TempDir()
	// Write initial
	if err := desktopconfig.Write(tmp, map[string]string{"A": "1", "B": "2"}); err != nil {
		t.Fatalf("Write initial: %v", err)
	}
	// Write only "B" update — "A" must survive.
	if err := desktopconfig.Write(tmp, map[string]string{"B": "99"}); err != nil {
		t.Fatalf("Write update: %v", err)
	}
	got, err := desktopconfig.Load(tmp)
	if err != nil {
		t.Fatalf("Load: %v", err)
	}
	if got["A"] != "1" {
		t.Errorf("A: got %q want 1", got["A"])
	}
	if got["B"] != "99" {
		t.Errorf("B: got %q want 99", got["B"])
	}
}

func TestWrite_Atomic(t *testing.T) {
	tmp := t.TempDir()
	if err := desktopconfig.Write(tmp, map[string]string{"X": "y"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	entries, _ := os.ReadDir(tmp)
	for _, e := range entries {
		if strings.HasSuffix(e.Name(), ".tmp") {
			t.Errorf("leftover .tmp file: %s", e.Name())
		}
	}
}

func TestBackupAndWipe_CreatesBackup(t *testing.T) {
	tmp := t.TempDir()
	if err := desktopconfig.Write(tmp, map[string]string{"KEY": "val"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := desktopconfig.BackupAndWipe(tmp, 5); err != nil {
		t.Fatalf("BackupAndWipe: %v", err)
	}
	// Original gone.
	if _, err := os.Stat(filepath.Join(tmp, "damask.env")); !os.IsNotExist(err) {
		t.Error("original config should be gone")
	}
	// Backup exists.
	entries, _ := os.ReadDir(tmp)
	var bakFound bool
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			bakFound = true
		}
	}
	if !bakFound {
		t.Error("expected backup file")
	}
}

func TestBackupAndWipe_PrunesOldest(t *testing.T) {
	tmp := t.TempDir()
	// Create 6 fake backup files with different timestamps.
	for i := range 6 {
		name := filepath.Join(tmp, "damask.env.bak.2026-01-0"+string(rune('1'+i))+"T00-00-00Z")
		_ = os.WriteFile(name, []byte("K=V"), 0o600)
	}
	// Write a real config to wipe.
	if err := desktopconfig.Write(tmp, map[string]string{"K": "V"}); err != nil {
		t.Fatalf("Write: %v", err)
	}
	if err := desktopconfig.BackupAndWipe(tmp, 5); err != nil {
		t.Fatalf("BackupAndWipe: %v", err)
	}
	entries, _ := os.ReadDir(tmp)
	var count int
	for _, e := range entries {
		if strings.Contains(e.Name(), ".bak.") {
			count++
		}
	}
	if count > 5 {
		t.Errorf("expected ≤5 backup files, got %d", count)
	}
}

func TestLatestBackup_ReturnsNewest(t *testing.T) {
	tmp := t.TempDir()
	_ = os.WriteFile(filepath.Join(tmp, "damask.env.bak.2026-01-01T00-00-00Z"), []byte("KEY=old\n"), 0o600)
	_ = os.WriteFile(filepath.Join(tmp, "damask.env.bak.2026-06-01T00-00-00Z"), []byte("KEY=new\n"), 0o600)

	m, err := desktopconfig.LatestBackup(tmp)
	if err != nil {
		t.Fatalf("LatestBackup: %v", err)
	}
	if m["KEY"] != "new" {
		t.Errorf("got %q want new", m["KEY"])
	}
}

func TestLatestBackup_NoneExist_ReturnsEmpty(t *testing.T) {
	tmp := t.TempDir()
	m, err := desktopconfig.LatestBackup(tmp)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(m) != 0 {
		t.Errorf("expected empty map, got %v", m)
	}
}
