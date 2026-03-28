package api

import (
	"context"

	dbgen "creativo-dam/server/internal/db/gen"
	"creativo-dam/server/internal/auth"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
	"golang.org/x/crypto/bcrypt"
)

// Server holds shared dependencies injected at startup.
type Server struct {
	db         *dbgen.Queries
	tokenMaker *auth.Maker
}

// New creates a configured Fiber app with all routes registered.
func New(db *dbgen.Queries, tokenMaker *auth.Maker) *fiber.App {
	s := &Server{db: db, tokenMaker: tokenMaker}

	app := fiber.New(fiber.Config{
		ErrorHandler: defaultErrorHandler,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     "http://localhost:5173",
		AllowCredentials: true,
		AllowHeaders:     "Origin, Content-Type, Authorization",
		AllowMethods:     "GET, POST, PUT, DELETE, OPTIONS",
	}))

	// Health check (public)
	app.Get("/healthz", handleHealthz)

	// Auth routes (public)
	authGroup := app.Group("/auth")
	authGroup.Post("/register", s.handleRegister)
	authGroup.Post("/login", s.handleLogin)
	authGroup.Post("/logout", s.handleLogout)
	authGroup.Post("/refresh", auth.RequireAuth(tokenMaker), s.handleRefresh)

	// Protected API routes
	api := app.Group("/api/v1", auth.RequireAuth(tokenMaker))

	// Workspace
	api.Get("/workspace/me", s.handleWorkspaceMe)

	// Invites — owner only
	getRoleFn := func(workspaceID, userID string) (string, error) {
		return s.db.GetMemberRole(context.Background(), dbgen.GetMemberRoleParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
		})
	}
	api.Post("/workspace/invites", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleCreateInvite)
	api.Post("/workspace/invites/accept", s.handleAcceptInvite)

	return app
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}

func bcryptHash(password string) (string, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return "", err
	}
	return string(hash), nil
}
