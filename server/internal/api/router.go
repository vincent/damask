package api

import (
	"context"
	"database/sql"

	"creativo-dam/server/internal/auth"
	dbgen "creativo-dam/server/internal/db/gen"
	"creativo-dam/server/internal/storage"

	"github.com/gofiber/fiber/v2"
	"github.com/gofiber/fiber/v2/middleware/cors"
)

// Server holds shared dependencies injected at startup.
type Server struct {
	db         *dbgen.Queries
	sqlDB      *sql.DB
	tokenMaker *auth.Maker
	storage    storage.Storage
	appEnv     string
}

// New creates a configured Fiber app with all routes registered.
func New(db *dbgen.Queries, sqlDB *sql.DB, tokenMaker *auth.Maker, stor storage.Storage, appEnv string) *fiber.App {
	s := &Server{db: db, sqlDB: sqlDB, tokenMaker: tokenMaker, storage: stor, appEnv: appEnv}

	app := fiber.New(fiber.Config{
		ErrorHandler: defaultErrorHandler,
		BodyLimit:    100 * 1024 * 1024, // 100 MB
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

	// Invites — owner only
	api.Post("/workspace/invites", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleCreateInvite)

	// Invite acceptance is public — the caller has no account yet
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	// Assets
	api.Post("/assets", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUploadAsset)
	api.Get("/assets", s.handleListAssets)
	api.Get("/assets/:id", s.handleGetAsset)
	api.Get("/assets/:id/file", s.handleGetAssetFile)
	api.Get("/assets/:id/thumb", s.handleGetAssetThumb)
	api.Delete("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteAsset)

	return app
}

func defaultErrorHandler(c *fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}
