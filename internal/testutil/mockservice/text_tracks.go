package mockservice

import (
	"context"

	"damask/server/internal/service"
)

type MockTextTrackService struct {
	ListFn               func(ctx context.Context, workspaceID, assetID string) ([]service.TextTrackDTO, error)
	GetFn                func(ctx context.Context, workspaceID, trackID string) (service.TextTrackDTO, error)
	CreateFn             func(ctx context.Context, p service.CreateTextTrackParams) (service.TextTrackDTO, error)
	DeleteFn             func(ctx context.Context, workspaceID, trackID string) error
	RunOCRFn             func(ctx context.Context, workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat string) error
	RunExtractPDFFn      func(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractPlainFn    func(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error
	RunExtractDocumentFn func(ctx context.Context, workspaceID, assetID, trackID, storageKey, mimeType string) error
}

func NewTextTrackService() *MockTextTrackService { return &MockTextTrackService{} }

func (m *MockTextTrackService) List(ctx context.Context, workspaceID, assetID string) ([]service.TextTrackDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, assetID)
	}
	return nil, nil //nolint:nilnil // mock
}

func (m *MockTextTrackService) Get(ctx context.Context, workspaceID, trackID string) (service.TextTrackDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, trackID)
	}
	return service.TextTrackDTO{}, nil
}

func (m *MockTextTrackService) Create(
	ctx context.Context,
	p service.CreateTextTrackParams,
) (service.TextTrackDTO, error) {
	if m.CreateFn != nil {
		return m.CreateFn(ctx, p)
	}
	return service.TextTrackDTO{}, nil
}

func (m *MockTextTrackService) Delete(ctx context.Context, workspaceID, trackID string) error {
	if m.DeleteFn != nil {
		return m.DeleteFn(ctx, workspaceID, trackID)
	}
	return nil
}

func (m *MockTextTrackService) RunOCR(ctx context.Context, workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat string) error {
	if m.RunOCRFn != nil {
		return m.RunOCRFn(ctx, workspaceID, assetID, trackID, assetVersionID, storageKey, mimeType, lang, outputFormat)
	}
	return nil
}

func (m *MockTextTrackService) RunExtractPDF(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error {
	if m.RunExtractPDFFn != nil {
		return m.RunExtractPDFFn(ctx, workspaceID, assetID, trackID, storageKey)
	}
	return nil
}

func (m *MockTextTrackService) RunExtractPlain(ctx context.Context, workspaceID, assetID, trackID, storageKey string) error {
	if m.RunExtractPlainFn != nil {
		return m.RunExtractPlainFn(ctx, workspaceID, assetID, trackID, storageKey)
	}
	return nil
}

func (m *MockTextTrackService) RunExtractDocument(ctx context.Context, workspaceID, assetID, trackID, storageKey, mimeType string) error {
	if m.RunExtractDocumentFn != nil {
		return m.RunExtractDocumentFn(ctx, workspaceID, assetID, trackID, storageKey, mimeType)
	}
	return nil
}
