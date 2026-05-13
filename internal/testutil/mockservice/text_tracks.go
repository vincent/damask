package mockservice

import (
	"context"

	"damask/server/internal/service"
)

type MockTextTrackService struct {
	ListFn   func(ctx context.Context, workspaceID, assetID string) ([]service.TextTrackDTO, error)
	GetFn    func(ctx context.Context, workspaceID, trackID string) (service.TextTrackDTO, error)
	CreateFn func(ctx context.Context, p service.CreateTextTrackParams) (service.TextTrackDTO, error)
	DeleteFn func(ctx context.Context, workspaceID, trackID string) error
}

func NewTextTrackService() *MockTextTrackService { return &MockTextTrackService{} }

func (m *MockTextTrackService) List(ctx context.Context, workspaceID, assetID string) ([]service.TextTrackDTO, error) {
	if m.ListFn != nil {
		return m.ListFn(ctx, workspaceID, assetID)
	}
	return nil, nil
}

func (m *MockTextTrackService) Get(ctx context.Context, workspaceID, trackID string) (service.TextTrackDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID, trackID)
	}
	return service.TextTrackDTO{}, nil
}

func (m *MockTextTrackService) Create(ctx context.Context, p service.CreateTextTrackParams) (service.TextTrackDTO, error) {
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
