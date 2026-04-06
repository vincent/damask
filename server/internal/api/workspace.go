package api

import (
	"database/sql"
	"errors"
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type WorkspaceMeResponse struct {
	Workspace dbgen.Workspace `json:"workspace"`
	User      UserResponse    `json:"user"`
	Role      string          `json:"role"`
}

func (s *Server) handleWorkspaceMe(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "workspace not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace")
	}

	user, err := s.db.GetUserByID(c.RequestCtx(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "user not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}

	member, err := s.db.GetMember(c.RequestCtx(), dbgen.GetMemberParams{
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
	})
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	return c.JSON(WorkspaceMeResponse{Workspace: workspace, User: userToResponse(user), Role: member.Role})
}

func (s *Server) handleCreateWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	user, err := s.db.GetUserByID(c.RequestCtx(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return errRes(c, fiber.StatusNotFound, "user not found")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not load user")
	}

	req, ok := decodeAndValidate(c, &createWorkspaceRequest{})
	if !ok {
		return nil
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	workspace, err := services.CreateWorkspaceForUser(c.RequestCtx(), qtx, req.Name, user.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create workspace")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Workspace: workspace})
}

type InviteResponse struct {
	ID          string `json:"id"`
	InviteToken string `json:"invite_token"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

func (s *Server) handleCreateInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &createInviteRequest{})
	if !ok {
		return nil
	}

	invite, err := s.db.CreateInvite(c.RequestCtx(), dbgen.CreateInviteParams{
		ID:          uuid.New().String(),
		WorkspaceID: claims.WorkspaceID,
		Email:       req.Email,
		Token:       uuid.New().String(),
		Role:        req.Role,
		InvitedBy:   claims.UserID,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create invite")
	}

	return c.Status(fiber.StatusCreated).JSON(InviteResponse{
		ID:          invite.ID,
		InviteToken: invite.Token,
		Email:       invite.Email,
		Role:        invite.Role,
	})
}

func (s *Server) handleAcceptInvite(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &acceptInviteRequest{})
	if !ok {
		return nil
	}

	invite, err := s.db.GetInviteByToken(c.RequestCtx(), req.Token)
	if err != nil {
		return errRes(c, fiber.StatusNotFound, "invalid or expired invite token")
	}

	hash, err := bcryptHash(req.Password)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	userID := uuid.New().String()
	user, err := qtx.CreateUser(c.RequestCtx(), dbgen.CreateUserParams{
		ID:           userID,
		Email:        invite.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusConflict, "email already in use")
	}

	if err := qtx.CreateMember(c.RequestCtx(), dbgen.CreateMemberParams{
		WorkspaceID: invite.WorkspaceID,
		UserID:      userID,
		Role:        invite.Role,
		InvitedBy:   &invite.InvitedBy,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not add workspace member")
	}

	if err := qtx.AcceptInvite(c.RequestCtx(), invite.ID); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not mark invite as accepted")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	token, err := s.tokenMaker.CreateToken(userID, invite.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Token: token, User: userToResponse(user)})
}

type WorkspaceWithRoleResponse struct {
	ID        string `json:"id"`
	Name      string `json:"name"`
	Role      string `json:"role"`
	CreatedAt string `json:"created_at"`
	UpdatedAt string `json:"updated_at"`
}

func (s *Server) handleListWorkspaces(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	rows, err := s.db.ListWorkspacesByUserID(c.RequestCtx(), claims.UserID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not list workspaces")
	}

	result := make([]WorkspaceWithRoleResponse, len(rows))
	for i, r := range rows {
		result[i] = WorkspaceWithRoleResponse{
			ID:        r.ID,
			Name:      r.Name,
			Role:      r.Role,
			CreatedAt: r.CreatedAt.Format(time.RFC3339),
			UpdatedAt: r.UpdatedAt.Format(time.RFC3339),
		}
	}
	return c.JSON(result)
}

type SwitchWorkspaceResponse struct {
	Token     string          `json:"token"`
	Workspace dbgen.Workspace `json:"workspace"`
	Role      string          `json:"role"`
}

func (s *Server) handleSwitchWorkspace(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	req, ok := decodeAndValidate(c, &switchWorkspaceRequest{})
	if !ok {
		return nil
	}

	member, err := s.db.GetMember(c.RequestCtx(), dbgen.GetMemberParams{
		WorkspaceID: req.WorkspaceID,
		UserID:      claims.UserID,
	})
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), req.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusForbidden, "not a member of this workspace")
	}

	token, err := s.tokenMaker.CreateToken(claims.UserID, req.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(SwitchWorkspaceResponse{Token: token, Workspace: workspace, Role: member.Role})
}

func (s *Server) handleUpdateWorkspaceSettings(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	body, ok := decodeAndValidate(c, &updateWorkspaceSettingsRequest{})
	if !ok {
		return nil
	}

	if err := s.db.UpdateWorkspaceVersionRetention(c.RequestCtx(), dbgen.UpdateWorkspaceVersionRetentionParams{
		VersionRetentionCount: body.VersionRetentionCount,
		ID:                    claims.WorkspaceID,
	}); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not update settings")
	}

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), claims.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not reload workspace")
	}
	return c.JSON(workspace)
}

// fiber:context-methods migrated
