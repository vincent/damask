package mockservice

import (
	"context"

	"damask/server/internal/service"
)

// MockUserService is a no-op implementation of service.UserService.
type MockUserService struct {
	RegisterFn        func(ctx context.Context, p service.RegisterUserParams) (*service.RegisterUserResult, error)
	LoginFn           func(ctx context.Context, p service.LoginUserParams) (*service.LoginUserResult, error)
	GetByIDFn         func(ctx context.Context, userID string) (*service.UserDTO, error)
	CreateWorkspaceFn func(ctx context.Context, userID, name string) (*service.WorkspaceDTO, error)
	GetProfileFn      func(ctx context.Context, userID string) (*service.OIDCUserDTO, error)
	UpsertOIDCUserFn  func(ctx context.Context, p service.UpsertOIDCUserParams) (*service.OIDCUserDTO, error)
	UpsertCanvaUserFn func(ctx context.Context, p service.UpsertCanvaUserParams) (*service.OIDCUserDTO, error)
	UnlinkProviderFn  func(ctx context.Context, userID, provider string) (*service.OIDCUserDTO, error)
}

func NewUserService() *MockUserService { return &MockUserService{} }

func (m *MockUserService) Register(ctx context.Context, p service.RegisterUserParams) (*service.RegisterUserResult, error) {
	if m.RegisterFn != nil {
		return m.RegisterFn(ctx, p)
	}
	return nil, nil
}

func (m *MockUserService) Login(ctx context.Context, p service.LoginUserParams) (*service.LoginUserResult, error) {
	if m.LoginFn != nil {
		return m.LoginFn(ctx, p)
	}
	return nil, nil
}

func (m *MockUserService) GetByID(ctx context.Context, userID string) (*service.UserDTO, error) {
	if m.GetByIDFn != nil {
		return m.GetByIDFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockUserService) CreateWorkspace(ctx context.Context, userID, name string) (*service.WorkspaceDTO, error) {
	if m.CreateWorkspaceFn != nil {
		return m.CreateWorkspaceFn(ctx, userID, name)
	}
	return nil, nil
}

func (m *MockUserService) GetProfile(ctx context.Context, userID string) (*service.OIDCUserDTO, error) {
	if m.GetProfileFn != nil {
		return m.GetProfileFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockUserService) UpsertOIDCUser(ctx context.Context, p service.UpsertOIDCUserParams) (*service.OIDCUserDTO, error) {
	if m.UpsertOIDCUserFn != nil {
		return m.UpsertOIDCUserFn(ctx, p)
	}
	return nil, nil
}

func (m *MockUserService) UpsertCanvaUser(ctx context.Context, p service.UpsertCanvaUserParams) (*service.OIDCUserDTO, error) {
	if m.UpsertCanvaUserFn != nil {
		return m.UpsertCanvaUserFn(ctx, p)
	}
	return nil, nil
}

func (m *MockUserService) UnlinkProvider(ctx context.Context, userID, provider string) (*service.OIDCUserDTO, error) {
	if m.UnlinkProviderFn != nil {
		return m.UnlinkProviderFn(ctx, userID, provider)
	}
	return nil, nil
}
