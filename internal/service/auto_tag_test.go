package service_test

import (
	"context"
	"errors"
	"testing"

	"damask/server/internal/apperr"
	"damask/server/internal/repository"
	"damask/server/internal/repository/memory"
	"damask/server/internal/service"
	"damask/server/internal/testutil/mockservice"

	"github.com/google/uuid"
)

const autoTagTestWorkspaceID = "ws_1"

// newAutoTagEnv wires an AutoTagService against memory repos with tags stubbed by a mock.
func newAutoTagEnv(t *testing.T, tags service.TagService) (
	service.AutoTagService,
	*memory.AssetRepo,
	*memory.AutoTagSuggestionMemoryRepo,
) {
	t.Helper()
	assets := memory.NewAssetRepo()
	workspaces := memory.NewRealWorkspaceRepo()
	suggestions := memory.NewAutoTagSuggestionRepo()
	workspaces.Seed(repository.Workspace{ID: autoTagTestWorkspaceID, Name: "ws"})
	svc := service.NewAutoTagService(assets, workspaces, suggestions, nil, tags, nil)
	return svc, assets, suggestions
}

func seedAutoTagAsset(assets *memory.AssetRepo, assetID string) {
	assets.Seed(repository.Asset{
		ID: assetID, WorkspaceID: autoTagTestWorkspaceID,
		OriginalFilename: "photo.jpg", StorageKey: "k/" + assetID, MimeType: "image/jpeg",
	})
}

//nolint:unparam // assetID kept general; every current test case happens to use the same asset.
func seedAutoTagSuggestion(suggestions *memory.AutoTagSuggestionMemoryRepo, assetID, tagName string) string {
	id := uuid.NewString()
	suggestions.Seed(repository.AutoTagSuggestion{
		ID: id, WorkspaceID: autoTagTestWorkspaceID, AssetID: assetID, TagName: tagName,
	})
	return id
}

func TestAutoTagService_AcceptSuggestion_AssetMismatch_ReturnsNotFound(t *testing.T) {
	svc, assets, suggestions := newAutoTagEnv(t, mockservice.NewTagService())
	seedAutoTagAsset(assets, "ast_1")
	seedAutoTagAsset(assets, "ast_2")
	sugID := seedAutoTagSuggestion(suggestions, "ast_1", "hero")

	_, err := svc.AcceptSuggestion(context.Background(), autoTagTestWorkspaceID, "ast_2", sugID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for mismatched asset id, got %v", err)
	}
}

func TestAutoTagService_DismissSuggestion_AssetMismatch_ReturnsNotFound(t *testing.T) {
	svc, assets, suggestions := newAutoTagEnv(t, mockservice.NewTagService())
	seedAutoTagAsset(assets, "ast_1")
	seedAutoTagAsset(assets, "ast_2")
	sugID := seedAutoTagSuggestion(suggestions, "ast_1", "hero")

	err := svc.DismissSuggestion(context.Background(), autoTagTestWorkspaceID, "ast_2", sugID)
	if !errors.Is(err, apperr.ErrNotFound) {
		t.Fatalf("expected ErrNotFound for mismatched asset id, got %v", err)
	}
}

func TestAutoTagService_AcceptAll_ContinuesPastPerItemErrors(t *testing.T) {
	tags := mockservice.NewTagService()
	tags.AddToAssetFn = func(_ context.Context, _, _, tagName string) (*service.TagDTO, error) {
		if tagName == "bad-tag" {
			return nil, errors.New("boom")
		}
		return &service.TagDTO{Name: tagName}, nil
	}
	svc, assets, suggestions := newAutoTagEnv(t, tags)
	seedAutoTagAsset(assets, "ast_1")
	seedAutoTagSuggestion(suggestions, "ast_1", "ok-tag-1")
	seedAutoTagSuggestion(suggestions, "ast_1", "bad-tag")
	seedAutoTagSuggestion(suggestions, "ast_1", "ok-tag-2")

	accepted, err := svc.AcceptAll(context.Background(), autoTagTestWorkspaceID, "ast_1")
	if err != nil {
		t.Fatalf("expected partial success without error, got %v", err)
	}
	if accepted != 2 {
		t.Fatalf("expected 2 accepted, got %d", accepted)
	}

	remaining, err := suggestions.List(context.Background(), autoTagTestWorkspaceID, "ast_1")
	if err != nil {
		t.Fatalf("list remaining: %v", err)
	}
	if len(remaining) != 1 || remaining[0].TagName != "bad-tag" {
		t.Fatalf("expected only bad-tag to remain, got %+v", remaining)
	}
}

func TestAutoTagService_AcceptAll_AllFail_ReturnsError(t *testing.T) {
	tags := mockservice.NewTagService()
	tags.AddToAssetFn = func(_ context.Context, _, _, _ string) (*service.TagDTO, error) {
		return nil, errors.New("boom")
	}
	svc, assets, suggestions := newAutoTagEnv(t, tags)
	seedAutoTagAsset(assets, "ast_1")
	seedAutoTagSuggestion(suggestions, "ast_1", "bad-tag-1")
	seedAutoTagSuggestion(suggestions, "ast_1", "bad-tag-2")

	accepted, err := svc.AcceptAll(context.Background(), autoTagTestWorkspaceID, "ast_1")
	if err == nil {
		t.Fatal("expected error when every suggestion fails")
	}
	if accepted != 0 {
		t.Fatalf("expected 0 accepted, got %d", accepted)
	}
}

// -- Cross-workspace negative test --
// AutoTagSuggestionRepository must not leak suggestions across workspaces.

func TestAutoTagSuggestionRepository_CrossWorkspaceIsolation(t *testing.T) {
	ctx := context.Background()
	repo := memory.NewAutoTagSuggestionRepo()

	wsA, wsB := "ws-a", "ws-b"
	sug, err := repo.Create(ctx, repository.CreateAutoTagSuggestionParams{
		ID: "sug-1", WorkspaceID: wsA, AssetID: "asset-1", TagName: "hero",
	})
	if err != nil {
		t.Fatalf("create: %v", err)
	}

	if out, listErr := repo.List(ctx, wsB, "asset-1"); listErr != nil || len(out) != 0 {
		t.Errorf("List cross-workspace: expected empty, got %v, err=%v", out, listErr)
	}
	if _, getErr := repo.Get(ctx, wsB, sug.ID); !errors.Is(getErr, apperr.ErrNotFound) {
		t.Errorf("Get cross-workspace: expected ErrNotFound, got %v", getErr)
	}
	if delErr := repo.Delete(ctx, wsB, sug.ID); !errors.Is(delErr, apperr.ErrNotFound) {
		t.Errorf("Delete cross-workspace: expected ErrNotFound, got %v", delErr)
	}
	if delByAssetErr := repo.DeleteByAsset(ctx, wsB, "asset-1"); delByAssetErr != nil {
		t.Errorf("DeleteByAsset cross-workspace: unexpected error %v", delByAssetErr)
	}
	// Sanity: DeleteByAsset on the wrong workspace must not have removed it.
	if _, getErr := repo.Get(ctx, wsA, sug.ID); getErr != nil {
		t.Errorf("Get same-workspace after cross-workspace DeleteByAsset: unexpected error %v", getErr)
	}
}
