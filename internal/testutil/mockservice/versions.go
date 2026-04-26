package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockVersionService is a no-op implementation of service.VersionService.
type MockVersionService struct {
	ListFn                 func(ctx context.Context, assetID string) ([]*service.VersionDTO, error)
	ListWithVariantCountFn func(ctx context.Context, assetID string) ([]*service.VersionWithCountDTO, error)
	GetFn                  func(ctx context.Context, workspaceID, id string) (*service.VersionDTO, error)
	GetCurrentByAssetFn    func(ctx context.Context, assetID string) (*service.VersionDTO, error)
	GetByHashFn            func(ctx context.Context, assetID, contentHash string) (*service.VersionDTO, error)
	NextVersionNumFn       func(ctx context.Context, assetID string) (int64, error)
	CreateFn               func(ctx context.Context, v *service.VersionDTO) (*service.VersionDTO, error)
	SetCurrentFn           func(ctx context.Context, assetID, versionID string) error
	SetAssetThumbnailFn    func(ctx context.Context, assetID string, key *string) error
	DeleteFn               func(ctx context.Context, workspaceID, assetID, versionID string) error
}

func NewVersionService() *MockVersionService { return &MockVersionService{} }

func (m *MockVersionService) List(ctx context.Context, assetID string) ([]*service.VersionDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockVersionService) ListWithVariantCount(ctx context.Context, assetID string) ([]*service.VersionWithCountDTO, error) {
	if m.ListWithVariantCountFn != nil {
		return m.ListWithVariantCountFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockVersionService) Get(ctx context.Context, workspaceID, id string) (*service.VersionDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil
}

func (m *MockVersionService) GetCurrentByAsset(ctx context.Context, assetID string) (*service.VersionDTO, error) {
	if m.GetCurrentByAssetFn != nil {
		return m.GetCurrentByAssetFn(ctx, assetID)
	}
	return nil, nil
}

func (m *MockVersionService) GetByHash(ctx context.Context, assetID, contentHash string) (*service.VersionDTO, error) {
	if m.GetByHashFn != nil {
		return m.GetByHashFn(ctx, assetID, contentHash)
	}
	return nil, nil
}

func (m *MockVersionService) NextVersionNum(ctx context.Context, assetID string) (int64, error) {
	if m.NextVersionNumFn != nil {
		return m.NextVersionNumFn(ctx, assetID)
	}
	return 0, nil
}

func (m *MockVersionService) Create(ctx context.Context, v *service.VersionDTO) (*service.VersionDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, v)
	}
	return nil, nil
}

func (m *MockVersionService) SetCurrent(ctx context.Context, assetID, versionID string) error {
	if m.SetCurrentFn != nil {
		return m.SetCurrentFn(ctx, assetID, versionID)
	}
	return nil
}

func (m *MockVersionService) SetAssetThumbnail(ctx context.Context, assetID string, key *string) error {
	if m.SetAssetThumbnailFn != nil {
		return m.SetAssetThumbnailFn(ctx, assetID, key)
	}
	return nil
}

func (m *MockVersionService) Delete(ctx context.Context, workspaceID, assetID, versionID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, assetID, versionID)
	}
	return nil
}

func (m *MockVersionService) WriteVersionUploaded(_ context.Context, _, _ string, _ *service.VersionDTO, _ string) {
}

func (m *MockVersionService) WriteVersionRestored(_ context.Context, _, _ string, _, _ int64) {
}

// MockVariantService is a no-op implementation of service.VariantService.
type MockVariantService struct {
	ListFn   func(ctx context.Context, workspaceID, assetID string) ([]*service.VariantDTO, error)
	GetFn    func(ctx context.Context, workspaceID, id string) (*service.VariantDTO, error)
	CreateFn func(ctx context.Context, p service.CreateVariantParams) (*service.VariantDTO, error)
	DeleteFn func(ctx context.Context, workspaceID, assetID, variantID string) error
}

func NewVariantService() *MockVariantService { return &MockVariantService{} }

func (m *MockVariantService) List(ctx context.Context, workspaceID, assetID string) ([]*service.VariantDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockVariantService) Get(ctx context.Context, workspaceID, id string) (*service.VariantDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, id)
	}
	return nil, nil
}

func (m *MockVariantService) Create(ctx context.Context, p service.CreateVariantParams) (*service.VariantDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, p)
	}
	return nil, nil
}

func (m *MockVariantService) Delete(ctx context.Context, workspaceID, assetID, variantID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, assetID, variantID)
	}
	return nil
}

func (m *MockVariantService) WriteVariantQueued(_ context.Context, _, _, _ string) {}

func (m *MockVariantService) WriteVariantDownloadedAsync(_, _, _, _ string) {}
