package api

import (
	"context"
	"database/sql"

	"damask/server/internal/auth"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/queue"
	"damask/server/internal/storage"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	_ "damask/server/docs"
)

// Server holds shared dependencies injected at startup.
type Server struct {
	db             *dbgen.Queries
	sqlDB          *sql.DB
	tokenMaker     *auth.Maker
	storage        storage.Storage
	queue          *queue.Queue
	previewCache   *lruPreviewCache
	removeBgAPIKey string
	appEnv         string
	baseUrl        string
}

// New creates a configured Fiber app with all routes registered.
// @title Damask Swagger API
// @version 1.0
// @description This is a Damask server.
// @termsOfService http://swagger.io/terms/

// @contact.name API Support
// @contact.url http://www.swagger.io/support
// @contact.email support@getdamask.com

// @license.name Apache 2.0
// @license.url http://www.apache.org/licenses/LICENSE-2.0.html

// @BasePath /
// @schemes http
func New(
	db *dbgen.Queries,
	sqlDB *sql.DB,
	tokenMaker *auth.Maker,
	stor storage.Storage,
	q *queue.Queue,
	removeBgAPIKey,
	appEnv string,
	baseUrl string,
) *fiber.App {
	s := &Server{
		db:             db,
		sqlDB:          sqlDB,
		tokenMaker:     tokenMaker,
		storage:        stor,
		queue:          q,
		previewCache:   newLRUPreviewCache(100),
		removeBgAPIKey: removeBgAPIKey,
		appEnv:         appEnv,
		baseUrl:        baseUrl,
	}
	s.RegisterJobHandlers()

	app := fiber.New(fiber.Config{
		ErrorHandler: defaultErrorHandler,
		BodyLimit:    100 * 1024 * 1024, // 100 MB,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173"},
		AllowCredentials: true,
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
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
	api.Post("/workspace", s.handleCreateWorkspace)
	api.Get("/workspaces", s.handleListWorkspaces)
	api.Post("/workspace/switch", s.handleSwitchWorkspace)

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

	// Projects
	api.Post("/projects", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateProject)
	api.Get("/projects", s.handleListProjects)
	api.Get("/projects/:id", s.handleGetProject)
	api.Put("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateProject)
	api.Delete("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteProject)

	// Tags
	api.Get("/tags", s.handleListTags)

	// Folders
	api.Post("/projects/:id/folders", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateFolder)
	api.Get("/projects/:id/folders", s.handleGetFolders)
	api.Put("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateFolder)
	api.Delete("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteFolder)

	// Assets — bulk routes must be registered before /:id to avoid conflict
	api.Post("/assets/bulk/tag", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkTag)
	api.Post("/assets/bulk/project", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkProject)
	api.Delete("/assets/bulk", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleBulkDelete)

	api.Post("/assets", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUploadAsset)
	api.Get("/assets", s.handleListAssets)
	api.Patch("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateAssetFolder)
	api.Get("/assets/:id", s.handleGetAsset)
	api.Get("/assets/:id/file", s.handleGetAssetFile)
	api.Get("/assets/:id/thumb", s.handleGetAssetThumb)
	api.Delete("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteAsset)

	// Asset tags
	api.Get("/assets/:id/tags", s.handleGetAssetTags)
	api.Post("/assets/:id/tags", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleAddTagToAsset)
	api.Delete("/assets/:id/tags/:name", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleRemoveTagFromAsset)

	// Variants
	api.Get("/assets/:id/variants", s.handleListVariants)
	api.Post("/assets/:id/variants", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateVariant)
	api.Get("/assets/:id/variants/:vid/file", s.handleGetVariantFile)
	api.Delete("/assets/:id/variants/:vid", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteVariant)

	// Transform preview (in-memory, no storage write)
	api.Get("/assets/:id/preview", s.handlePreviewTransform)

	// Shares — authenticated, workspace-scoped
	api.Post("/shares", s.handleCreateShare)
	api.Get("/shares", s.handleListShares)
	api.Get("/shares/:id", s.handleGetShare)
	api.Put("/shares/:id", s.handleUpdateShare)
	api.Delete("/shares/:id", s.handleRevokeShare)

	// Share comments — owner moderation (S-7)
	api.Get("/shares/:id/comments", s.handleOwnerListComments)
	api.Delete("/shares/:id/comments/:cid", s.handleOwnerDeleteComment)

	// Public share routes — unauthenticated or share-session-authenticated
	// S-4: access endpoints (no auth required)
	app.Get("/shared/:id/access", s.handleShareInfo)
	app.Post("/shared/:id/access", s.handleShareAccess)

	// S-5 + S-6: content and comment endpoints require a valid share session token
	shareGroup := app.Group("/shared/:id", auth.RequireShareSession(tokenMaker))
	shareGroup.Get("/assets", s.handleShareListAssets)
	shareGroup.Get("/assets/:aid", s.handleShareGetAsset)
	shareGroup.Get("/assets/:aid/file", s.handleShareGetAssetFile)
	shareGroup.Get("/assets/:aid/thumb", s.handleShareGetAssetThumb)
	shareGroup.Post("/comments", s.handleShareCreateComment)
	shareGroup.Get("/comments", s.handleShareListComments)
	shareGroup.Get("/assets/:aid/comments", s.handleShareListAssetComments)

	// Mount the UI with the default configuration under /swagger
	app.Get("/swagger/*", swaggo.HandlerDefault)

	return app
}

func defaultErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}
