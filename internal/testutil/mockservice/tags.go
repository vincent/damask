package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockTagService is a no-op implementation of service.TagService.
type MockTagService struct {
	ListFn         func(ctx context.Context, workspaceID string) ([]*service.TagDTO, error)
	CreateFn       func(ctx context.Context, workspaceID string, p service.CreateTagParams) (*service.TagDTO, error)
	PatchFn        func(ctx context.Context, workspaceID, currentName string, p service.PatchTagParams) (*service.TagDTO, error)
	DeleteFn       func(ctx context.Context, workspaceID string, names []string) error
	BulkDeleteFn   func(ctx context.Context, workspaceID string, names []string) (service.BulkDeleteTagsResult, error)
	MergeFn        func(ctx context.Context, workspaceID string, sources []string, target string) (service.MergeTagsResult, error)
	TouchLastUsedFn func(ctx context.Context, workspaceID, name string) error
	ListForAssetFn func(ctx context.Context, assetID string) ([]*service.TagDTO, error)
	AddToAssetFn   func(ctx context.Context, workspaceID, assetID, tagName string) (*service.TagDTO, error)
	RemoveFromAssetFn func(ctx context.Context, workspaceID, assetID, tagName string) error
	UpsertForAssetFn  func(ctx context.Context, workspaceID, assetID, tagName string) error
}

func NewTagService() *MockTagService { return &MockTagService{} }

func (m *MockTagService) List(ctx context.Context, workspaceID string) ([]*service.TagDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockTagService) Create(ctx context.Context, workspaceID string, p service.CreateTagParams) (*service.TagDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, workspaceID, p)
	}
	return nil, nil
}

func (m *MockTagService) Patch(ctx context.Context, workspaceID, currentName string, p service.PatchTagParams) (*service.TagDTO, error) {
	if m.PatchFn != nil {
		return m.PatchFn(ctx, workspaceID, currentName, p)
	}
	return nil, nil
}

func (m *MockTagService) Delete(ctx context.Context, workspaceID string, names []string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, names)
	}
	return nil
}

func (m *MockTagService) BulkDelete(ctx context.Context, workspaceID string, names []string) (service.BulkDeleteTagsResult, error) {
	if m.BulkDeleteFn != nil {
		return m.BulkDeleteFn(ctx, workspaceID, names)
	}
	return service.BulkDeleteTagsResult{}, nil
}

func (m *MockTagService) Merge(ctx context.Context, workspaceID string, sources []string, target string) (service.MergeTagsResult, error) {
	if m.MergeFn != nil {
		return m.MergeFn(ctx, workspaceID, sources, target)
	}
	return service.MergeTagsResult{}, nil
}

func (m *MockTagService) TouchLastUsed(ctx context.Context, workspaceID, name string) error {
	if m.TouchLastUsedFn != nil {
		return m.TouchLastUsedFn(ctx, workspaceID, name)
	}
	return nil
}

func (m *MockTagService) ListForAsset(ctx context.Context, assetID string) ([]*service.TagDTO, error) {
	if m.ListForAssetFn != nil {
		return m.ListForAssetFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockTagService) AddToAsset(ctx context.Context, workspaceID, assetID, tagName string) (*service.TagDTO, error) {
	if m.AddToAssetFn != nil {
		return m.AddToAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil, nil
}

func (m *MockTagService) RemoveFromAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	if m.RemoveFromAssetFn != nil {
		return m.RemoveFromAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil
}

func (m *MockTagService) UpsertForAsset(ctx context.Context, workspaceID, assetID, tagName string) error {
	if m.UpsertForAssetFn != nil {
		return m.UpsertForAssetFn(ctx, workspaceID, assetID, tagName)
	}
	return nil
}
