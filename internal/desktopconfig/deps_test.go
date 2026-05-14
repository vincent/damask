package desktopconfig_test

import (
	"testing"

	"damask/server/internal/desktopconfig"
)

func TestCheck_ReturnsAllDeps(t *testing.T) {
	all := desktopconfig.AllDeps()
	got := desktopconfig.Check()
	if len(got) != len(all) {
		t.Errorf("Check() returned %d results, want %d", len(got), len(all))
	}
}

func TestCheck_NeverPanics(t *testing.T) {
	// Run Check with an empty PATH — must not panic, all Found must be false.
	t.Setenv("PATH", "")
	defer func() {
		if r := recover(); r != nil {
			t.Errorf("Check() panicked: %v", r)
		}
	}()
	statuses := desktopconfig.Check()
	for _, s := range statuses {
		if s.Found {
			t.Errorf("dep %q should not be found with empty PATH", s.Binary)
		}
	}
}
