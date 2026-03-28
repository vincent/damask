package api

import (
	"context"
	"database/sql"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// Server holds shared dependencies injected at startup.
type Server struct {
	db         *dbgen.Queries
	sqlDB      *sql.DB
	tokenMaker *auth.Maker
	appEnv     string
}

// New creates a configured Fiber app with all routes registered.
func New(db *dbgen.Queries, sqlDB *sql.DB, tokenMaker *auth.Maker, appEnv string) *fiber.App {
	s := &Server{db: db, sqlDB: sqlDB, tokenMaker: tokenMaker, appEnv: appEnv}

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
	getRoleFn := func(ctx context.Context, workspaceID, userID string) (string, error) {
		member, err := s.db.GetMember(ctx, dbgen.GetMemberParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
		})
		if err != nil {
			return "", err
		}
		return member.Role, nil
	}
	api.Post("/workspace/invites", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleCreateInvite)

	// Invite acceptance is public — the caller has no account yet
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	return app
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}
