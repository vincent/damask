package api

import (
	"context"
	"database/sql"
	"io/fs"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/queue"
	"damask/server/internal/storage"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	_ "damask/server/internal/docs"
)

// Server holds shared dependencies injected at startup.
type Server struct {
	db           *dbgen.Queries
	sqlDB        *sql.DB
	tokenMaker   *auth.Maker
	storage      storage.Storage
	queue        queue.JobQueue
	hub          events.EventHub
	previewCache *lruPreviewCache
	cfg          *config.Config
	audit        *audit.EventWriter
	demo         DemoSeeder // nil when demo build tag is not set
}

func NewHttpServer(
	db *dbgen.Queries,
	sqlDB *sql.DB,
	tokenMaker *auth.Maker,
	stor storage.Storage,
	hub events.EventHub,
	q queue.JobQueue,
	cfg *config.Config,
	demoSeeder DemoSeeder,
) *Server {
	return &Server{
		db:           db,
		sqlDB:        sqlDB,
		tokenMaker:   tokenMaker,
		storage:      stor,
		queue:        q,
		hub:          hub,
		previewCache: NewLRUPreviewCache(100),
		cfg:          cfg,
		audit:        audit.New(sqlDB),
		demo:         demoSeeder,
	}
}

// NewRouter creates a configured Fiber app with all routes registered.
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
func NewRouter(
	db *dbgen.Queries,
	sqlDB *sql.DB,
	tokenMaker *auth.Maker,
	stor storage.Storage,
	hub events.EventHub,
	q queue.JobQueue,
	cfg *config.Config,
	demoSeeder DemoSeeder,
	uiFS fs.FS,
) *fiber.App {
	s := NewHttpServer(db, sqlDB, tokenMaker, stor, hub, q, cfg, demoSeeder)

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

	// Public server config (demo flag, etc.)
	app.Get("/config", s.handleConfig)

	// Demo routes — only compiled and registered with -tags=demo
	s.registerDemoRoutes(app, cfg)

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

	// Workspace settings — owner only; blocked in demo mode
	api.Put("/workspace/settings", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleUpdateWorkspaceSettings)

	// Invites — owner only; blocked in demo mode
	api.Post("/workspace/invites", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleCreateInvite)

	// Invite acceptance is public — the caller has no account yet
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	// Field definitions — reorder must be registered before /:id to avoid conflict
	api.Get("/field-definitions", s.handleListFieldDefinitions)
	api.Post("/field-definitions", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateFieldDefinition)
	api.Put("/field-definitions/reorder", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleReorderFieldDefinitions)
	api.Get("/field-definitions/:id", s.handleGetFieldDefinition)
	api.Get("/field-definitions/:id/stats", s.handleGetFieldDefinitionStats)
	api.Put("/field-definitions/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateFieldDefinition)
	api.Delete("/field-definitions/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteFieldDefinition)

	// Projects
	api.Post("/projects", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateProject)
	api.Get("/projects", s.handleListProjects)
	api.Get("/projects/:id", s.handleGetProject)
	api.Put("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateProject)
	api.Delete("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteProject)

	// Project field values
	api.Get("/projects/:id/fields", s.handleGetProjectFields)
	api.Patch("/projects/:id/fields", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handlePatchProjectFields)

	// Tags
	api.Get("/tags", s.handleListTags)
	api.Post("/tags", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateTag)
	api.Patch("/tags/:name", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handlePatchTag)
	api.Delete("/tags", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkDeleteTags)
	api.Post("/tags/merge", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleMergeTags)
	api.Get("/tags/suggestions/duplicates", s.handleTagDuplicateSuggestions)

	// Folders
	api.Post("/projects/:id/folders", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateFolder)
	api.Get("/projects/:id/folders", s.handleGetFolders)
	api.Put("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateFolder)
	api.Delete("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteFolder)

	// Assets — bulk routes must be registered before /:id to avoid conflict
	api.Post("/assets/bulk/tag", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkTag)
	api.Post("/assets/bulk/project", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkProject)
	api.Delete("/assets/bulk", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleBulkDelete)
	api.Patch("/assets/bulk/fields", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleBulkPatchAssetFields)

	api.Post("/assets", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUploadAsset)
	api.Get("/assets", s.handleListAssets)
	api.Patch("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateAssetFolder)
	api.Put("/assets/:id/rename", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleRenameAsset)
	api.Get("/assets/:id", s.handleGetAsset)
	api.Get("/assets/:id/file", s.handleGetAssetFile)
	api.Get("/assets/:id/thumb", s.handleGetAssetThumb)
	api.Delete("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteAsset)

	// Asset field values
	api.Get("/assets/:id/fields", s.handleGetAssetFields)
	api.Patch("/assets/:id/fields", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handlePatchAssetFields)

	// Asset tags
	api.Get("/assets/:id/tags", s.handleGetAssetTags)
	api.Post("/assets/:id/tags", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleAddTagToAsset)
	api.Delete("/assets/:id/tags/:name", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleRemoveTagFromAsset)

	// Variants
	api.Get("/assets/:id/variants", s.handleListVariants)
	api.Post("/assets/:id/variants", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateVariant)
	api.Post("/assets/:id/variants/upload", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUploadManualVariant)
	api.Get("/assets/:id/variants/:vid/file", s.handleGetVariantFile)
	api.Delete("/assets/:id/variants/:vid", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteVariant)

	// Asset versions
	api.Get("/assets/:id/versions", s.handleListAssetVersions)
	api.Post("/assets/:id/versions", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUploadAssetVersion)
	api.Post("/assets/:id/versions/:vid/restore", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleRestoreAssetVersion)
	api.Get("/assets/:id/versions/:vid/file", s.handleGetVersionFile)
	api.Get("/assets/:id/versions/:vid/thumb", s.handleGetVersionThumb)
	api.Delete("/assets/:id/versions/:vid", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteAssetVersion)

	// Transform preview (in-memory, no storage write)
	api.Get("/assets/:id/preview", s.handlePreviewTransform)

	// Server-Sent Events (workspace-scoped)
	api.Get("/events", hub.EventHandler)

	// Asset event log
	api.Get("/assets/:id/events", s.handleListAssetEvents)

	// Project event log
	api.Get("/projects/:id/events", s.handleListProjectEvents)

	// Workspace activity feed
	api.Get("/activity", s.handleListWorkspaceActivity)
	api.Get("/activity/export", s.handleExportActivity)

	// Ingress sources
	ingressGroup := api.Group("/ingress")
	ingressGroup.Post("/sources", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateIngressSource)
	ingressGroup.Get("/sources", s.handleListIngressSources)
	ingressGroup.Get("/sources/:id", s.handleGetIngressSource)
	ingressGroup.Put("/sources/:id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateIngressSource)
	ingressGroup.Delete("/sources/:id", auth.RequireRole(tokenMaker, getRoleFn, "owner"), s.handleDeleteIngressSource)
	ingressGroup.Post("/sources/:id/test", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleTestIngressSource)
	ingressGroup.Post("/sources/:id/poll", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handlePollIngressSource)
	ingressGroup.Get("/sources/:id/log", s.handleListIngressSourceLog)
	ingressGroup.Get("/log", s.handleListIngressLog)
	ingressGroup.Delete("/log/:entry_id", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteIngressLogEntry)
	ingressGroup.Post("/log/:entry_id/retry", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleRetryIngressLogEntry)

	// Ingress rules — reorder must be registered before /:rid to avoid conflict
	ingressGroup.Get("/sources/:id/rules", s.handleListIngressRules)
	ingressGroup.Post("/sources/:id/rules", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleCreateIngressRule)
	ingressGroup.Put("/sources/:id/rules/reorder", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleReorderIngressRules)
	ingressGroup.Put("/sources/:id/rules/:rid", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleUpdateIngressRule)
	ingressGroup.Delete("/sources/:id/rules/:rid", auth.RequireRole(tokenMaker, getRoleFn, "editor"), s.handleDeleteIngressRule)

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

	// Serve the SvelteKit SPA
	if uiFS != nil {
		// Production: serve embedded SPA
		app.Use("/", newSPAHandler(uiFS))
	} else {
		// Development: proxy to Vite dev server
		app.Use("/", newViteProxy())
	}

	return app
}

func defaultErrorHandler(c fiber.Ctx, err error) error {
	code := fiber.StatusInternalServerError
	if e, ok := err.(*fiber.Error); ok {
		code = e.Code
	}
	return c.Status(code).JSON(fiber.Map{"error": err.Error()})
}
