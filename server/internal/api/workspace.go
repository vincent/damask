package api

import (
	"database/sql"
	"errors"
	"time"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
)

type workspaceMeResponse struct {
	Workspace dbgen.Workspace `json:"workspace"`
	User      userResponse    `json:"user"`
	Role      string          `json:"role"`
}

func (s *Server) handleWorkspaceMe(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	workspace, err := s.db.GetWorkspaceByID(c.Context(), claims.WorkspaceID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "workspace not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not load workspace"})
	}

	user, err := s.db.GetUserByID(c.Context(), claims.UserID)
	if err != nil {
		if errors.Is(err, sql.ErrNoRows) {
			return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "user not found"})
		}
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not load user"})
	}

	member, err := s.db.GetMember(c.Context(), dbgen.GetMemberParams{
		WorkspaceID: claims.WorkspaceID,
		UserID:      claims.UserID,
	})
	if err != nil {
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": "not a member of this workspace"})
	}

	return c.JSON(workspaceMeResponse{Workspace: workspace, User: userToResponse(user), Role: member.Role})
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

func (s *Server) handleCreateInvite(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	var req createInviteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Email == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email is required"})
	}
	if req.Role == "" {
		req.Role = "editor"
	}
	if req.Role != "editor" && req.Role != "viewer" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "role must be editor or viewer"})
	}

	invite, err := s.db.CreateInvite(c.Context(), dbgen.CreateInviteParams{
		ID:          uuid.New().String(),
		WorkspaceID: claims.WorkspaceID,
		Email:       req.Email,
		Token:       uuid.New().String(),
		Role:        req.Role,
		InvitedBy:   claims.UserID,
		ExpiresAt:   time.Now().Add(7 * 24 * time.Hour),
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create invite"})
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

func (s *Server) handleAcceptInvite(c *fiber.Ctx) error {
	var req acceptInviteRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Token == "" || req.Name == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "token, name and password are required"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
	}

	invite, err := s.db.GetInviteByToken(c.Context(), req.Token)
	if err != nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "invalid or expired invite token"})
	}

	hash, err := bcryptHash(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not hash password"})
	}

	tx, err := s.sqlDB.BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not begin transaction"})
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	userID := uuid.New().String()
	user, err := qtx.CreateUser(c.Context(), dbgen.CreateUserParams{
		ID:           userID,
		WorkspaceID:  invite.WorkspaceID,
		Email:        invite.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already in use"})
	}

	if err := qtx.CreateMember(c.Context(), dbgen.CreateMemberParams{
		WorkspaceID: invite.WorkspaceID,
		UserID:      userID,
		Role:        invite.Role,
		InvitedBy:   sql.NullString{String: invite.InvitedBy, Valid: true},
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not add workspace member"})
	}

	if err := qtx.AcceptInvite(c.Context(), invite.ID); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not mark invite as accepted"})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not commit transaction"})
	}

	token, err := s.tokenMaker.CreateToken(userID, invite.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create token"})
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(authResponse{Token: token, User: userToResponse(user)})
}
