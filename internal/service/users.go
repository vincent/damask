package service

import (
	"context"
	"fmt"
	"time"

	"damask/server/internal/apperr"
	"damask/server/internal/auth"
	"damask/server/internal/repository"

	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// UserDTO is the output representation of a user.
type UserDTO struct {
	ID        string
	Email     string
	Name      string
	CreatedAt time.Time
}

// RegisterUserParams is the input for UserService.Register.
type RegisterUserParams struct {
	UserID        string
	Name          string
	Email         string
	PasswordHash  string
	WorkspaceName string
}

// RegisterUserResult is returned by UserService.Register.
type RegisterUserResult struct {
	User        *UserDTO
	WorkspaceID string
}

// LoginUserParams is the input for UserService.Login.
type LoginUserParams struct {
	Email         string
	PlainPassword string
}

// LoginUserResult is returned by UserService.Login.
type LoginUserResult struct {
	User        *UserDTO
	WorkspaceID string
}

type userService struct {
	users      repository.UserRepository
	workspaces repository.WorkspaceRepository
}

// NewUserService returns a UserService.
func NewUserService(
	users repository.UserRepository,
	workspaces repository.WorkspaceRepository,
) UserService {
	return &userService{users: users, workspaces: workspaces}
}

// Register creates a new user and a default workspace atomically.
// It uses WorkspaceRepository.RunRegistrationTx so that user row, workspace row, and
// member row all land in a single DB transaction without leaking *sql.DB into the service.
func (s *userService) Register(ctx context.Context, p RegisterUserParams) (*RegisterUserResult, error) {
	workspaceID := uuid.New().String()
	var result RegisterUserResult

	err := s.workspaces.RunRegistrationTx(ctx, func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
		u, err := txUsers.Create(ctx, repository.User{
			ID:           p.UserID,
			Email:        p.Email,
			PasswordHash: p.PasswordHash,
			Name:         p.Name,
		})
		if err != nil {
			return fmt.Errorf("email already in use: %w", apperr.ErrConflict)
		}

		ws, err := txWorkspaces.Create(ctx, repository.Workspace{
			ID:   workspaceID,
			Name: p.WorkspaceName,
		})
		if err != nil {
			return fmt.Errorf("could not create workspace: %w", err)
		}

		if err := txWorkspaces.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      u.ID,
			Role:        string(auth.Owner),
		}); err != nil {
			return fmt.Errorf("could not create membership: %w", err)
		}

		result = RegisterUserResult{User: toUserDTO(u), WorkspaceID: ws.ID}
		return nil
	})
	if err != nil {
		return nil, err
	}
	return &result, nil
}

func (s *userService) Login(ctx context.Context, p LoginUserParams) (*LoginUserResult, error) {
	user, err := s.users.GetByEmail(ctx, p.Email)
	if err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(p.PlainPassword)); err != nil {
		return nil, fmt.Errorf("invalid credentials: %w", apperr.ErrForbidden)
	}

	workspaces, err := s.workspaces.ListByUserID(ctx, user.ID)
	if err != nil || len(workspaces) == 0 {
		return nil, fmt.Errorf("user has no workspace: %w", apperr.ErrInvalidInput)
	}

	return &LoginUserResult{
		User:        toUserDTO(user),
		WorkspaceID: workspaces[0].ID,
	}, nil
}

func (s *userService) GetByID(ctx context.Context, userID string) (*UserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return toUserDTO(user), nil
}

// CreateWorkspace creates a new workspace owned by userID in a transaction.
func (s *userService) CreateWorkspace(ctx context.Context, userID, name string) (*WorkspaceDTO, error) {
	workspaceID := uuid.New().String()

	err := s.workspaces.RunInTx(ctx, func(txWS repository.WorkspaceRepository) error {
		ws, err := txWS.Create(ctx, repository.Workspace{
			ID:   workspaceID,
			Name: name,
		})
		if err != nil {
			return err
		}
		return txWS.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      userID,
			Role:        string(auth.Owner),
		})
	})
	if err != nil {
		return nil, err
	}

	ws, err := s.workspaces.GetByID(ctx, workspaceID)
	if err != nil {
		return nil, err
	}
	return toWorkspaceDTO(ws), nil
}

func toUserDTO(u repository.User) *UserDTO {
	return &UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}
