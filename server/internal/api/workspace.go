package api

import (
	"database/sql"
	"errors"
	"time"

	"badam/server/internal/auth"
	dbgen "badam/server/internal/db/gen"
	"badam/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
)

type workspaceMeResponse struct {
	Workspace dbgen.Workspace `json:"workspace"`
	User      userResponse    `json:"user"`
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

	return c.JSON(workspaceMeResponse{Workspace: workspace, User: userToResponse(user), Role: member.Role})
}

type createWorkspaceRequest struct {
	Name string `json:"name"`
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

	var req createWorkspaceRequest
	if err := c.Bind().Body(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Name == "" {
		return errRes(c, fiber.StatusBadRequest, "name is required")
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

	return c.Status(fiber.StatusCreated).JSON(authResponse{Workspace: workspace})
}

type createInviteRequest struct {
	Email string `json:"email"`
	Role  string `json:"role"` // "editor" or "viewer"
}

type inviteResponse struct {
	ID          string `json:"id"`
	InviteToken string `json:"invite_token"`
	Email       string `json:"email"`
	Role        string `json:"role"`
}

func (s *Server) handleCreateInvite(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var req createInviteRequest
	if err := c.Bind().Body(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Email == "" {
		return errRes(c, fiber.StatusBadRequest, "email is required")
	}
	if req.Role == "" {
		req.Role = "editor"
	}
	if req.Role != "editor" && req.Role != "viewer" {
		return errRes(c, fiber.StatusBadRequest, "role must be editor or viewer")
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

	return c.Status(fiber.StatusCreated).JSON(inviteResponse{
		ID:          invite.ID,
		InviteToken: invite.Token,
		Email:       invite.Email,
		Role:        invite.Role,
	})
}

type acceptInviteRequest struct {
	Token    string `json:"token"`
	Name     string `json:"name"`
	Password string `json:"password"`
}

func (s *Server) handleAcceptInvite(c fiber.Ctx) error {
	var req acceptInviteRequest
	if err := c.Bind().Body(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Token == "" || req.Name == "" || req.Password == "" {
		return errRes(c, fiber.StatusBadRequest, "token, name and password are required")
	}
	if len(req.Password) < 8 {
		return errRes(c, fiber.StatusBadRequest, "password must be at least 8 characters")
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
		InvitedBy:   sql.NullString{String: invite.InvitedBy, Valid: true},
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
	return c.Status(fiber.StatusCreated).JSON(authResponse{Token: token, User: userToResponse(user)})
}

// fiber:context-methods migrated
