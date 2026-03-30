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
	ID        string    `json:"id"`
	Email     string    `json:"email"`
	Name      string    `json:"name"`
	CreatedAt time.Time `json:"created_at"`
}

type authResponse struct {
	Token     string           `json:"token"`
	User      userResponse     `json:"user"`
	Workspace *dbgen.Workspace `json:"workspace,omitempty"`
}

func userToResponse(u dbgen.User) userResponse {
	return userResponse{
		ID:        u.ID,
		Email:     u.Email,
		Name:      u.Name,
		CreatedAt: u.CreatedAt,
	}
}

func (s *Server) handleRegister(c fiber.Ctx) error {
	var req registerRequest
	if err := c.Bind().Body(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Email == "" || req.Password == "" || req.Name == "" {
		return errRes(c, fiber.StatusBadRequest, "name, email and password are required")
	}
	if len(req.Password) < 8 {
		return errRes(c, fiber.StatusBadRequest, "password must be at least 8 characters")
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
	defer tx.Rollback() //nolint:errcheck

	qtx := s.db.WithTx(tx)

	user, err := qtx.CreateUser(c.RequestCtx(), dbgen.CreateUserParams{
		ID:           userID,
		Email:        req.Email,
		PasswordHash: hash,
		Name:         req.Name,
	})
	if err != nil {
		return errRes(c, fiber.StatusConflict, "email already in use")
	}

	workspace, err := services.CreateWorkspaceForUser(c.RequestCtx(), qtx, req.Name+"'s Workspace", userID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create workspace")
	}

	if err := tx.Commit(); err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not commit transaction")
	}

	token, err := s.tokenMaker.CreateToken(userID, workspace.ID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.Status(fiber.StatusCreated).JSON(authResponse{Token: token, User: userToResponse(user), Workspace: workspace})
}

// Login godoc
// @Summary Login to the application.
// @Description get auth token.
// @Tags Auth
// @Accept */*
// @Produce json
// @Param        email       query     string  false  "Email"
// @Param        password    query     string  false  "Password"
// // @Success 200 {object} AuthResponse
// @Router /login [post]
func (s *Server) handleLogin(c fiber.Ctx) error {
	var req loginRequest
	if err := c.Bind().Body(&req); err != nil {
		return errRes(c, fiber.StatusBadRequest, "invalid request body")
	}
	if req.Email == "" || req.Password == "" {
		return errRes(c, fiber.StatusBadRequest, "email and password are required")
	}

	user, err := s.db.GetUserByEmail(c.RequestCtx(), req.Email)
	if err != nil {
		return errRes(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	if err := bcrypt.CompareHashAndPassword([]byte(user.PasswordHash), []byte(req.Password)); err != nil {
		return errRes(c, fiber.StatusUnauthorized, "invalid credentials")
	}

	member, err := s.db.GetMemberByUserID(c.RequestCtx(), user.ID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace membership")
	}

	workspace, err := s.db.GetWorkspaceByID(c.RequestCtx(), member.WorkspaceID)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not load workspace")
	}

	token, err := s.tokenMaker.CreateToken(user.ID, member.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(authResponse{Token: token, User: userToResponse(user), Workspace: &workspace})
}

func (s *Server) handleRefresh(c fiber.Ctx) error {
	claims := auth.GetClaims(c)

	token, err := s.tokenMaker.CreateToken(claims.UserID, claims.WorkspaceID, 7*24*time.Hour)
	if err != nil {
		return errRes(c, fiber.StatusInternalServerError, "could not create token")
	}

	s.setAuthCookie(c, token)
	return c.JSON(fiber.Map{"token": token})
}

func (s *Server) handleLogout(c fiber.Ctx) error {
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

func (s *Server) setAuthCookie(c fiber.Ctx, token string) {
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

// fiber:context-methods migrated
