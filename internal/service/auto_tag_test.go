package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	dbpkg "damask/server/internal/db"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/service"
	"damask/server/internal/testutil/mockservice"

	"github.com/google/uuid"
)

const autoTagTestWorkspaceID = "ws_1"

func newAutoTagTestDB(t *testing.T) *dbgen.Queries {
	t.Helper()
	queries, sqlDB, err := dbpkg.Open(":memory:?_foreign_keys=ON")
	if err != nil {
		t.Fatalf("open db: %v", err)
	}
	t.Cleanup(func() { _ = sqlDB.Close() })
	if _, wsErr := queries.CreateWorkspace(context.Background(), dbgen.CreateWorkspaceParams{
		ID: autoTagTestWorkspaceID, Name: "ws",
	}); wsErr != nil {
		t.Fatalf("seed workspace: %v", wsErr)
	}
	return queries
}

func seedAutoTagAsset(t *testing.T, queries *dbgen.Queries, assetID string) {
	t.Helper()
	if _, err := queries.CreateAsset(context.Background(), dbgen.CreateAssetParams{
		ID:               assetID,
		WorkspaceID:      autoTagTestWorkspaceID,
		OriginalFilename: "photo.jpg",
		StorageKey:       "k/" + assetID,
		MimeType:         "image/jpeg",
	}); err != nil {
		t.Fatalf("seed asset: %v", err)
	}
}

func seedAutoTagSuggestion(t *testing.T, queries *dbgen.Queries, tagName string) string {
	t.Helper()
	id := uuid.NewString()
	if _, err := queries.CreateAutoTagSuggestion(context.Background(), dbgen.CreateAutoTagSuggestionParams{
		ID:          id,
		WorkspaceID: autoTagTestWorkspaceID,
		AssetID:     "ast_1",
		TagName:     tagName,
	}); err != nil {
		t.Fatalf("seed suggestion: %v", err)
	}
	return id
}

func TestAutoTagService_AcceptSuggestion_AssetMismatch_ReturnsNotFound(t *testing.T) {
	queries := newAutoTagTestDB(t)
	seedAutoTagAsset(t, queries, "ast_1")
	seedAutoTagAsset(t, queries, "ast_2")
	sugID := seedAutoTagSuggestion(t, queries, "hero")

	svc := service.NewAutoTagService(queries, nil, mockservice.NewTagService(), nil)

	_, err := svc.AcceptSuggestion(context.Background(), autoTagTestWorkspaceID, "ast_2", sugID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for mismatched asset id, got %v", err)
	}
}

func TestAutoTagService_DismissSuggestion_AssetMismatch_ReturnsNotFound(t *testing.T) {
	queries := newAutoTagTestDB(t)
	seedAutoTagAsset(t, queries, "ast_1")
	seedAutoTagAsset(t, queries, "ast_2")
	sugID := seedAutoTagSuggestion(t, queries, "hero")

	svc := service.NewAutoTagService(queries, nil, mockservice.NewTagService(), nil)

	err := svc.DismissSuggestion(context.Background(), autoTagTestWorkspaceID, "ast_2", sugID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for mismatched asset id, got %v", err)
	}
}

func TestAutoTagService_AcceptAll_ContinuesPastPerItemErrors(t *testing.T) {
	queries := newAutoTagTestDB(t)
	seedAutoTagAsset(t, queries, "ast_1")
	seedAutoTagSuggestion(t, queries, "ok-tag-1")
	seedAutoTagSuggestion(t, queries, "bad-tag")
	seedAutoTagSuggestion(t, queries, "ok-tag-2")

	tags := mockservice.NewTagService()
	tags.AddToAssetFn = func(_ context.Context, _, _, tagName string) (*service.TagDTO, error) {
		if tagName == "bad-tag" {
			return nil, errors.New("boom")
		}
		return &service.TagDTO{Name: tagName}, nil
	}
	svc := service.NewAutoTagService(queries, nil, tags, nil)

	accepted, err := svc.AcceptAll(context.Background(), autoTagTestWorkspaceID, "ast_1")
	if err != nil {
		t.Fatalf("expected partial success without error, got %v", err)
	}
	if accepted != 2 {
		t.Fatalf("expected 2 accepted, got %d", accepted)
	}

	remaining, err := queries.ListAutoTagSuggestions(context.Background(), dbgen.ListAutoTagSuggestionsParams{
		AssetID:     "ast_1",
		WorkspaceID: autoTagTestWorkspaceID,
	})
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].TagName != "bad-tag" {
		t.Fatalf("expected only bad-tag to remain, got %+v", remaining)
	}
}

func TestAutoTagService_AcceptAll_AllFail_ReturnsError(t *testing.T) {
	queries := newAutoTagTestDB(t)
	seedAutoTagAsset(t, queries, "ast_1")
	seedAutoTagSuggestion(t, queries, "bad-tag-1")
	seedAutoTagSuggestion(t, queries, "bad-tag-2")

	tags := mockservice.NewTagService()
	tags.AddToAssetFn = func(_ context.Context, _, _, _ string) (*service.TagDTO, error) {
		return nil, errors.New("boom")
	}
	svc := service.NewAutoTagService(queries, nil, tags, nil)

	accepted, err := svc.AcceptAll(context.Background(), autoTagTestWorkspaceID, "ast_1")
	if err == nil {
		t.Fatal("expected error when every suggestion fails")
	}
	if accepted != 0 {
		t.Fatalf("expected 0 accepted, got %d", accepted)
	}
}
