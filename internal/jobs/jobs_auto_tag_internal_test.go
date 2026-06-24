package jobs

import (
	"context"
	"testing"
)

// TestStoreAutoTagSuggestions_AllInsertsFail_ReturnsError exercises the case
// where every CreateAutoTagSuggestion insert fails (e.g. a stale/nonexistent
// asset_id violating the foreign key) — the job must surface an error rather
// than silently reporting success with zero suggestions persisted.
func TestStoreAutoTagSuggestions_AllInsertsFail_ReturnsError(t *testing.T) {
	_, _, js, _, _ := newMediaTagsJobTestEnv(t)

	payload := AutoTagPayload{WorkspaceID: "ws_test", AssetID: "nonexistent-asset"}
	err := js.storeAutoTagSuggestions(context.Background(), payload, []string{"hero", "blue"})
	if err == nil {
		t.Fatal("expected an error when every suggestion insert fails")
	}
}
