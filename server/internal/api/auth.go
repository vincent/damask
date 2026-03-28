package api

import (
	"database/sql"
	"time"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/gofiber/fiber/v2"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

// bcryptCost is the work factor used for password hashing.
// Tests override this to bcrypt.MinCost for speed.
var bcryptCost = bcrypt.DefaultCost

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

type registerRequest struct {
	Name     string `json:"name"`
	Email    string `json:"email"`
	Password string `json:"password"`
}

type loginRequest struct {
	Email    string `json:"email"`
	Password string `json:"password"`
}

// userResponse omits the password hash from JSON output.
type userResponse struct {
	ID          string    `json:"id"`
	WorkspaceID string    `json:"workspace_id"`
	Email       string    `json:"email"`
	Name        string    `json:"name"`
	CreatedAt   time.Time `json:"created_at"`
}

type authResponse struct {
	Token     string           `json:"token"`
	User      userResponse     `json:"user"`
	Workspace *dbgen.Workspace `json:"workspace,omitempty"`
}

func userToResponse(u dbgen.User) userResponse {
	return userResponse{
		ID:          u.ID,
		WorkspaceID: u.WorkspaceID,
		Email:       u.Email,
		Name:        u.Name,
		CreatedAt:   u.CreatedAt,
	}
}

func (s *Server) handleRegister(c *fiber.Ctx) error {
	var req registerRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "name, email and password are required"})
	}
	if len(req.Password) < 8 {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "password must be at least 8 characters"})
	}

	hash, err := bcryptHash(req.Password)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not hash password"})
	}

	workspaceID := uuid.New().String()
	userID := uuid.New().String()

	tx, err := s.sqlDB.BeginTx(c.Context(), nil)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not begin transaction"})
	}
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	workspace, err := qtx.CreateWorkspace(c.Context(), dbgen.CreateWorkspaceParams{
		ID:   workspaceID,
		Name: req.Name + "'s Workspace",
	})
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create workspace"})
	}

	user, err := qtx.CreateUser(c.Context(), dbgen.CreateUserParams{
		ID:           userID,
		WorkspaceID:  workspaceID,
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": "email already in use"})
	}

	if err := qtx.CreateMember(c.Context(), dbgen.CreateMemberParams{
		WorkspaceID: workspaceID,
		UserID:      userID,
		Role:        "owner",
		InvitedBy:   sql.NullString{}, // owner has no inviter
	}); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not assign workspace owner"})
	}

	if err := tx.Commit(); err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not commit transaction"})
	}

	token, err := s.tokenMaker.CreateToken(userID, workspaceID, 7*24*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create token"})
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(authResponse{Token: token, User: userToResponse(user), Workspace: &workspace})
}

func (s *Server) handleLogin(c *fiber.Ctx) error {
	var req loginRequest
	if err := c.BodyParser(&req); err != nil {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request body"})
	}
	if req.Email == "" || req.Password == "" {
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "email and password are required"})
	}

	user, err := s.db.GetUserByEmail(c.Context(), req.Email)
	if err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "invalid credentials"})
	}

	workspace, err := s.db.GetWorkspaceByID(c.Context(), user.WorkspaceID)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not load workspace"})
	}

	token, err := s.tokenMaker.CreateToken(user.ID, user.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create token"})
	}

	s.setAuthCookie(c, token)
	return c.JSON(authResponse{Token: token, User: userToResponse(user), Workspace: &workspace})
}

func (s *Server) handleRefresh(c *fiber.Ctx) error {
	claims := auth.GetClaims(c)

	token, err := s.tokenMaker.CreateToken(claims.UserID, claims.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "could not create token"})
	}

	s.setAuthCookie(c, token)
	return c.JSON(fiber.Map{"token": token})
}

func (s *Server) handleLogout(c *fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   s.appEnv != "development",
		SameSite: "Lax",
		MaxAge:   -1,
		Path:     "/",
	})
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) setAuthCookie(c *fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    token,
		HTTPOnly: true,
		Secure:   s.appEnv != "development",
		SameSite: "Lax",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		Path:     "/",
	})
}
