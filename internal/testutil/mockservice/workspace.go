package mockservice

import (
	"context"

	"damask/server/internal/auth"
	"damask/server/internal/imagerouter"
	"damask/server/internal/service"
)

// MockWorkspaceService is a no-op implementation of service.WorkspaceService.
// By default GetMember returns a member with role "owner" so RequireRole middleware passes.
type MockWorkspaceService struct {
	GetFn                     func(ctx context.Context, workspaceID string) (*service.WorkspaceDTO, error)
	UpdateFn                  func(ctx context.Context, workspaceID string, p service.UpdateWorkspaceParams) (*service.WorkspaceDTO, error)
	MeFn                      func(ctx context.Context, workspaceID, userID string) (*service.WorkspaceMeDTO, error)
	ListForUserFn             func(ctx context.Context, userID string) ([]service.WorkspaceWithRoleDTO, error)
	CountAssetsFn             func(ctx context.Context, workspaceID string) (int64, error)
	ListImageRouterModelsFn   func(ctx context.Context, workspaceID string) ([]imagerouter.Model, imagerouter.KeyStatus, error)
	GetImageRouterKeyStatusFn func(ctx context.Context, workspaceID string) (imagerouter.KeyStatus, error)
	SetImageRouterKeyFn       func(ctx context.Context, workspaceID, plainKey string) error
	ClearImageRouterKeyFn     func(ctx context.Context, workspaceID string) error
	TestImageRouterKeyFn      func(ctx context.Context, workspaceID string) error
	GetMemberFn               func(ctx context.Context, workspaceID, userID string) (*service.MemberDTO, error)
	ListMembersFn             func(ctx context.Context, workspaceID string) ([]service.MemberDTO, error)
	RemoveMemberFn            func(ctx context.Context, workspaceID, callerID, targetUserID string) error
	UpdateMemberRoleFn        func(ctx context.Context, workspaceID, callerID, targetUserID string, role string) error
	CreateInviteFn            func(ctx context.Context, workspaceID, callerID string, p service.CreateInviteParams) (*service.InviteDTO, error)
	ListInvitesFn             func(ctx context.Context, workspaceID string) ([]service.InviteDTO, error)
	DeleteInviteFn            func(ctx context.Context, workspaceID, inviteID string) error
	AcceptInviteFn            func(ctx context.Context, p service.AcceptInviteParams) (*service.AcceptInviteResult, error)
}

func NewWorkspaceService() *MockWorkspaceService { return &MockWorkspaceService{} }

func (m *MockWorkspaceService) Get(ctx context.Context, workspaceID string) (*service.WorkspaceDTO, error) {
	if m.GetFn != nil {
		return m.GetFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockWorkspaceService) Update(ctx context.Context, workspaceID string, p service.UpdateWorkspaceParams) (*service.WorkspaceDTO, error) {
	if m.UpdateFn != nil {
		return m.UpdateFn(ctx, workspaceID, p)
	}
	return nil, nil
}

func (m *MockWorkspaceService) Me(ctx context.Context, workspaceID, userID string) (*service.WorkspaceMeDTO, error) {
	if m.MeFn != nil {
		return m.MeFn(ctx, workspaceID, userID)
	}
	return nil, nil
}

func (m *MockWorkspaceService) ListForUser(ctx context.Context, userID string) ([]service.WorkspaceWithRoleDTO, error) {
	if m.ListForUserFn != nil {
		return m.ListForUserFn(ctx, userID)
	}
	return nil, nil
}

func (m *MockWorkspaceService) CountAssets(ctx context.Context, workspaceID string) (int64, error) {
	if m.CountAssetsFn != nil {
		return m.CountAssetsFn(ctx, workspaceID)
	}
	return 0, nil
}

func (m *MockWorkspaceService) ListImageRouterModels(ctx context.Context, workspaceID string) ([]imagerouter.Model, imagerouter.KeyStatus, error) {
	if m.ListImageRouterModelsFn != nil {
		return m.ListImageRouterModelsFn(ctx, workspaceID)
	}
	return nil, imagerouter.KeyStatus{}, nil
}

func (m *MockWorkspaceService) GetImageRouterKeyStatus(ctx context.Context, workspaceID string) (imagerouter.KeyStatus, error) {
	if m.GetImageRouterKeyStatusFn != nil {
		return m.GetImageRouterKeyStatusFn(ctx, workspaceID)
	}
	return imagerouter.KeyStatus{}, nil
}

func (m *MockWorkspaceService) SetImageRouterKey(ctx context.Context, workspaceID, plainKey string) error {
	if m.SetImageRouterKeyFn != nil {
		return m.SetImageRouterKeyFn(ctx, workspaceID, plainKey)
	}
	return nil
}

func (m *MockWorkspaceService) ClearImageRouterKey(ctx context.Context, workspaceID string) error {
	if m.ClearImageRouterKeyFn != nil {
		return m.ClearImageRouterKeyFn(ctx, workspaceID)
	}
	return nil
}

func (m *MockWorkspaceService) TestImageRouterKey(ctx context.Context, workspaceID string) error {
	if m.TestImageRouterKeyFn != nil {
		return m.TestImageRouterKeyFn(ctx, workspaceID)
	}
	return nil
}

// GetMember returns an owner by default so that RequireRole middleware passes in tests.
// Override GetMemberFn to test non-owner or missing-member scenarios.
func (m *MockWorkspaceService) GetMember(ctx context.Context, workspaceID, userID string) (*service.MemberDTO, error) {
	if m.GetMemberFn != nil {
		return m.GetMemberFn(ctx, workspaceID, userID)
	}
	return &service.MemberDTO{
		UserID: userID,
		Role:   string(auth.Owner),
	}, nil
}

func (m *MockWorkspaceService) ListMembers(ctx context.Context, workspaceID string) ([]service.MemberDTO, error) {
	if m.ListMembersFn != nil {
		return m.ListMembersFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockWorkspaceService) RemoveMember(ctx context.Context, workspaceID, callerID, targetUserID string) error {
	if m.RemoveMemberFn != nil {
		return m.RemoveMemberFn(ctx, workspaceID, callerID, targetUserID)
	}
	return nil
}

func (m *MockWorkspaceService) UpdateMemberRole(ctx context.Context, workspaceID, callerID, targetUserID string, role string) error {
	if m.UpdateMemberRoleFn != nil {
		return m.UpdateMemberRoleFn(ctx, workspaceID, callerID, targetUserID, role)
	}
	return nil
}

func (m *MockWorkspaceService) CreateInvite(ctx context.Context, workspaceID, callerID string, p service.CreateInviteParams) (*service.InviteDTO, error) {
	if m.CreateInviteFn != nil {
		return m.CreateInviteFn(ctx, workspaceID, callerID, p)
	}
	return nil, nil
}

func (m *MockWorkspaceService) ListInvites(ctx context.Context, workspaceID string) ([]service.InviteDTO, error) {
	if m.ListInvitesFn != nil {
		return m.ListInvitesFn(ctx, workspaceID)
	}
	return nil, nil
}

func (m *MockWorkspaceService) DeleteInvite(ctx context.Context, workspaceID, inviteID string) error {
	if m.DeleteInviteFn != nil {
		return m.DeleteInviteFn(ctx, workspaceID, inviteID)
	}
	return nil
}

func (m *MockWorkspaceService) AcceptInvite(ctx context.Context, p service.AcceptInviteParams) (*service.AcceptInviteResult, error) {
	if m.AcceptInviteFn != nil {
		return m.AcceptInviteFn(ctx, p)
	}
	return nil, nil
}
