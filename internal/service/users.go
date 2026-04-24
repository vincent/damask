package service

import (
	"context"
	"encoding/json"
	"errors"
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

// OIDCUserDTO is returned by UpsertOIDCUser / UpsertCanvaUser / UnlinkProvider.
type OIDCUserDTO struct {
	ID           string
	Name         string
	Email        string
	AvatarURL    *string
	AuthMethods  string
	OIDCLinked   bool
	GoogleLinked bool
	CanvaLinked  bool
	WorkspaceID  string
}

// UpsertOIDCUserParams is the input for UserService.UpsertOIDCUser.
type UpsertOIDCUserParams struct {
	Issuer    string
	Sub       string
	Email     string
	Name      string
	AvatarURL string
	IsGoogle  bool
}

// UpsertCanvaUserParams is the input for UserService.UpsertCanvaUser.
type UpsertCanvaUserParams struct {
	CanvaID   string
	Email     string
	Name      string
	AvatarURL string
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

func (s *userService) GetProfile(ctx context.Context, userID string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
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

func (s *userService) UpsertOIDCUser(ctx context.Context, p UpsertOIDCUserParams) (*OIDCUserDTO, error) {
	var user repository.User
	var lookupErr error
	if p.IsGoogle {
		user, lookupErr = s.users.GetByGoogleID(ctx, p.Sub)
	} else {
		user, lookupErr = s.users.GetByOIDC(ctx, p.Issuer, p.Sub)
	}

	if lookupErr != nil && !errors.Is(lookupErr, apperr.ErrNotFound) {
		return nil, lookupErr
	}

	if lookupErr == nil {
		// Existing user — refresh avatar/auth_methods.
		updated := user
		updated.AuthMethods = addAuthMethod(user.AuthMethods, methodName(p.IsGoogle))
		updated.AvatarUrl = nilIfEmpty(p.AvatarURL)
		var err error
		if p.IsGoogle {
			updated.GoogleUserID = &p.Sub
			user, err = s.users.LinkGoogle(ctx, updated)
		} else {
			updated.OidcIssuer = &p.Issuer
			updated.OidcSub = &p.Sub
			user, err = s.users.LinkOIDC(ctx, updated)
		}
		if err != nil {
			return nil, err
		}
		return s.toOIDCUserDTO(ctx, user)
	}

	// Try to link by email.
	existing, err := s.users.GetByEmail(ctx, p.Email)
	if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		existing.AuthMethods = addAuthMethod(existing.AuthMethods, methodName(p.IsGoogle))
		existing.AvatarUrl = nilIfEmpty(p.AvatarURL)
		if p.IsGoogle {
			existing.GoogleUserID = &p.Sub
			existing, err = s.users.LinkGoogle(ctx, existing)
		} else {
			existing.OidcIssuer = &p.Issuer
			existing.OidcSub = &p.Sub
			existing, err = s.users.LinkOIDC(ctx, existing)
		}
		if err != nil {
			return nil, err
		}
		return s.toOIDCUserDTO(ctx, existing)
	}

	// New user — create with workspace in one transaction.
	userID := uuid.New().String()
	workspaceID := uuid.New().String()
	initMethods := `["` + methodName(p.IsGoogle) + `"]`

	newUser := repository.User{
		ID:          userID,
		Email:       p.Email,
		Name:        p.Name,
		AvatarUrl:   nilIfEmpty(p.AvatarURL),
		AuthMethods: initMethods,
	}
	if p.IsGoogle {
		newUser.GoogleUserID = &p.Sub
	} else {
		newUser.OidcIssuer = &p.Issuer
		newUser.OidcSub = &p.Sub
	}

	err = s.workspaces.RunRegistrationTx(ctx, func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
		var uErr error
		if p.IsGoogle {
			user, uErr = txUsers.CreateWithGoogle(ctx, newUser)
		} else {
			user, uErr = txUsers.CreateWithOIDC(ctx, newUser)
		}
		if uErr != nil {
			return uErr
		}
		ws, wErr := txWorkspaces.Create(ctx, repository.Workspace{ID: workspaceID, Name: p.Name + "'s Workspace"})
		if wErr != nil {
			return wErr
		}
		return txWorkspaces.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Role:        string(auth.Owner),
		})
	})
	if err != nil {
		return nil, err
	}
	dto, err := s.toOIDCUserDTO(ctx, user)
	if err != nil {
		return nil, err
	}
	dto.WorkspaceID = workspaceID
	return dto, nil
}

func (s *userService) UpsertCanvaUser(ctx context.Context, p UpsertCanvaUserParams) (*OIDCUserDTO, error) {
	user, err := s.users.GetByCanvaID(ctx, p.CanvaID)
	if err != nil && !errors.Is(err, apperr.ErrNotFound) {
		return nil, err
	}
	if err == nil {
		user.AuthMethods = addAuthMethod(user.AuthMethods, "canva")
		user.AvatarUrl = nilIfEmpty(p.AvatarURL)
		user.CanvaUserID = &p.CanvaID
		user, err = s.users.LinkCanva(ctx, user)
		if err != nil {
			return nil, err
		}
		return s.toOIDCUserDTO(ctx, user)
	}

	if p.Email != "" {
		existing, emailErr := s.users.GetByEmail(ctx, p.Email)
		if emailErr == nil {
			existing.AuthMethods = addAuthMethod(existing.AuthMethods, "canva")
			existing.AvatarUrl = nilIfEmpty(p.AvatarURL)
			existing.CanvaUserID = &p.CanvaID
			existing, emailErr = s.users.LinkCanva(ctx, existing)
			if emailErr != nil {
				return nil, emailErr
			}
			return s.toOIDCUserDTO(ctx, existing)
		}
	}

	// New user.
	name := p.Name
	if name == "" {
		name = "Canva User"
	}
	email := "canva+" + p.CanvaID + "@canva.local"
	if p.Email != "" {
		email = p.Email
	}
	userID := uuid.New().String()
	workspaceID := uuid.New().String()
	newUser := repository.User{
		ID:          userID,
		Email:       email,
		Name:        name,
		AvatarUrl:   nilIfEmpty(p.AvatarURL),
		AuthMethods: `["canva"]`,
		CanvaUserID: &p.CanvaID,
	}

	err = s.workspaces.RunRegistrationTx(ctx, func(ctx context.Context, txUsers repository.UserRepository, txWorkspaces repository.WorkspaceRepository) error {
		var uErr error
		user, uErr = txUsers.CreateWithCanva(ctx, newUser)
		if uErr != nil {
			return uErr
		}
		ws, wErr := txWorkspaces.Create(ctx, repository.Workspace{ID: workspaceID, Name: name + "'s Workspace"})
		if wErr != nil {
			return wErr
		}
		return txWorkspaces.CreateMember(ctx, repository.Member{
			WorkspaceID: ws.ID,
			UserID:      user.ID,
			Role:        string(auth.Owner),
		})
	})
	if err != nil {
		return nil, err
	}
	dto, err := s.toOIDCUserDTO(ctx, user)
	if err != nil {
		return nil, err
	}
	dto.WorkspaceID = workspaceID
	return dto, nil
}

func (s *userService) UnlinkProvider(ctx context.Context, userID, provider string) (*OIDCUserDTO, error) {
	user, err := s.users.GetByID(ctx, userID)
	if err != nil {
		return nil, err
	}
	if hasOnlyMethodStr(user.AuthMethods, provider) {
		return nil, fmt.Errorf("set a password before removing your last sign-in method: %w", apperr.ErrInvalidInput)
	}
	user.AuthMethods = removeAuthMethodStr(user.AuthMethods, provider)
	switch provider {
	case "google":
		user, err = s.users.UnlinkGoogle(ctx, user)
	case "oidc":
		user, err = s.users.UnlinkOIDC(ctx, user)
	case "canva":
		user, err = s.users.UnlinkCanva(ctx, user)
	}
	if err != nil {
		return nil, err
	}
	return s.toOIDCUserDTO(ctx, user)
}

func (s *userService) toOIDCUserDTO(ctx context.Context, user repository.User) (*OIDCUserDTO, error) {
	ids, err := s.users.ListWorkspaceIDs(ctx, user.ID)
	if err != nil {
		return nil, err
	}
	wsID := ""
	if len(ids) > 0 {
		wsID = ids[0]
	}
	return &OIDCUserDTO{
		ID:           user.ID,
		Name:         user.Name,
		Email:        user.Email,
		AvatarURL:    user.AvatarUrl,
		AuthMethods:  user.AuthMethods,
		OIDCLinked:   user.OidcSub != nil,
		GoogleLinked: user.GoogleUserID != nil,
		CanvaLinked:  user.CanvaUserID != nil,
		WorkspaceID:  wsID,
	}, nil
}

func methodName(isGoogle bool) string {
	if isGoogle {
		return "google"
	}
	return "oidc"
}

func nilIfEmpty(s string) *string {
	if s == "" {
		return nil
	}
	return &s
}

func addAuthMethod(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	for _, m := range methods {
		if m == method {
			return current
		}
	}
	methods = append(methods, method)
	b, _ := json.Marshal(methods)
	return string(b)
}

func removeAuthMethodStr(current, method string) string {
	var methods []string
	_ = json.Unmarshal([]byte(current), &methods)
	out := methods[:0]
	for _, m := range methods {
		if m != method {
			out = append(out, m)
		}
	}
	b, _ := json.Marshal(out)
	return string(b)
}

func hasOnlyMethodStr(authMethods, method string) bool {
	var methods []string
	_ = json.Unmarshal([]byte(authMethods), &methods)
	return len(methods) == 1 && methods[0] == method
}

func toUserDTO(u repository.User) *UserDTO {
	return &UserDTO{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}
