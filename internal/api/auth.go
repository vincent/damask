package api

import (
	"time"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/services"

	"github.com/gofiber/fiber/v3"
	"github.com/google/uuid"
	"golang.org/x/crypto/bcrypt"
)

const sessionDuration = 7 * 24 * time.Hour

// BcryptCost is the work factor used for password hashing.
// Tests override this to bcrypt.MinCost for speed.
var BcryptCost = bcrypt.DefaultCost

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), BcryptCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}

// UserResponse omits the password hash from JSON output.
type UserResponse struct {
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type AuthResponse struct {
	Token     string           `json:"token"`
	User      UserResponse     `json:"user"`
	Workspace *WorkspaceResponse `json:"workspace,omitempty"`
}

func userToResponse(u dbgen.User) UserResponse {
	return UserResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}

// handleRegister creates a new user account and a default workspace.
//
// @Summary Register a new user
// @Description Creates a new user account with name, email, and password. A default workspace is automatically created for the new user. The response includes a JWT auth token set as an <code>auth_token</code> httpOnly cookie and also returned in the response body.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body RegisterRequest true "Registration details"
// @Success 201 {object} AuthResponse
// @Failure 400 {object} ErrorResponse "Invalid request body"
// @Failure 409 {object} ErrorResponse "Email already in use"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /auth/register [post]
func (s *Server) handleRegister(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &RegisterRequest{})
	if !ok {
		return nil
	}

	hash, err := bcryptHash(req.Password)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not hash password")
	}

	userID := uuid.New().String()

	tx, err := s.sqlDB.BeginTx(c.RequestCtx(), nil)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not begin transaction")
	}
	defer tx.Rollback()

	qtx := s.db.WithTx(tx)

	user, err := qtx.CreateUser(c.RequestCtx(), dbgen.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		if isUniqueConstraintError(err) {
			return errRes(c, fiber.StatusConflict, "email already in use")
		}
		return errRes(c, fiber.StatusInternalServerError, "could not create user")
	}

	workspace, err := services.CreateWorkspaceForUser(c.RequestCtx(), qtx, req.Name+"'s Workspace", userID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create workspace")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	token, err := s.tokenMaker.CreateToken(userID, workspace.ID, sessionDuration)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	wr := workspaceToResponse(*workspace)
	return c.Status(fiber.StatusCreated).JSON(AuthResponse{Token: token, User: userToResponse(user), Workspace: &wr})
}

// handleLogin authenticates a user and returns a JWT token.
//
// @Summary Log in
// @Description Authenticates the user with email and password. Returns a JWT token in the response body and sets an <code>auth_token</code> httpOnly cookie (7-day expiry). The workspace embedded in the response is the user's first workspace — use <code>POST /api/v1/workspace/switch</code> to change it.
// @Tags Auth
// @Accept json
// @Produce json
// @Param body body LoginRequest true "Login credentials"
// @Success 200 {object} AuthResponse
// @Failure 401 {object} ErrorResponse "Invalid credentials"
// @Failure 422 {object} ValidationErrorResponse "Validation failed"
// @Router /auth/login [post]
func (s *Server) handleLogin(c fiber.Ctx) error {
	req, ok := decodeAndValidate(c, &LoginRequest{})
	if !ok {
		return nil
	}

	user, err := s.db.GetUserByEmail(c.RequestCtx(), req.Email)
	if err != nil {
		return errRes(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return errRes(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	memberships, err := s.db.ListWorkspacesByUserID(c.RequestCtx(), user.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace membership")
	}
	if len(memberships) == 0 {
		return errRes(c, fiber.StatusInternalServerError, "user has no workspace")
	}

	first := memberships[0]
	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), first.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace")
	}

	token, err := s.tokenMaker.CreateToken(user.ID, first.ID, sessionDuration)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	wr := workspaceToResponse(workspace)
	return c.JSON(AuthResponse{Token: token, User: userToResponse(user), Workspace: &wr})
}

// handleRefresh reissues an auth token for the currently authenticated user.
//
// @Summary Refresh auth token
// @Description Reissues a fresh JWT with a new 7-day expiry for the authenticated user and workspace. The new token is returned in the response body and also set as an updated <code>auth_token</code> cookie. Call this endpoint before the current token expires to maintain a session without re-login.
// @Tags Auth
// @Produce json
// @Security BearerAuth
// @Success 200 {object} map[string]string "token"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /auth/refresh [post]
func (s *Server) handleRefresh(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	token, err := s.tokenMaker.CreateToken(claims.UserID, claims.WorkspaceID, sessionDuration)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(fiber.Map{"token": token})
}

// handleLogout clears the auth cookie and ends the session.
//
// @Summary Log out
// @Description Clears the <code>auth_token</code> httpOnly cookie by setting it to an expired value. No server-side session state is maintained, so this is a client-side-only operation.
// @Tags Auth
// @Produce json
// @Success 200 {object} map[string]bool "ok"
// @Router /auth/logout [post]
func (s *Server) handleLogout(c fiber.Ctx) error {
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    "",
		HTTPOnly: true,
		Secure:   s.cfg.AppEnv != "development",
		SameSite: "Lax",
		MaxAge:   -1,
		Path:     "/",
	})
	return c.JSON(fiber.Map{"ok": true})
}

func (s *Server) setAuthCookie(c fiber.Ctx, token string) {
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    token,
		HTTPOnly: true,
		Secure:   s.cfg.AppEnv != "development",
		SameSite: "Lax",
		MaxAge:   int((7 * 24 * time.Hour).Seconds()),
		Path:     "/",
	})
}

// fiber:context-methods migrated
