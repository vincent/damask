package api

import (
	"context"
	"database/sql"
	"fmt"
	"io/fs"

	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	dbgen "damask/server/internal/db/gen"
	"damask/server/internal/events"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	"damask/server/internal/storage"

	swaggo "github.com/gofiber/contrib/v3/swaggo"
	"github.com/gofiber/fiber/v3"
	"github.com/gofiber/fiber/v3/middleware/cors"

	_ "damask/server/internal/docs"
)

const defaultBodyLimitBytes = 100 * 1024 * 1024 // 100 MB

// Server holds shared dependencies injected at startup.
type Server struct {
	db           *dbgen.Queries
	sqlDB        *sql.DB
	tokenMaker   *auth.Maker
	storage      storage.Storage
	queue        queue.JobQueue
	mailer       mail.Mailer
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
	mailer mail.Mailer,
	cfg *config.Config,
	demoSeeder DemoSeeder,
) *Server {
	return &Server{
		db:           db,
		sqlDB:        sqlDB,
		tokenMaker:   tokenMaker,
		storage:      stor,
		queue:        q,
		mailer:       mailer,
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
	mailer mail.Mailer,
	cfg *config.Config,
	demoSeeder DemoSeeder,
	uiFS fs.FS,
) *fiber.App {
	s := NewHttpServer(db, sqlDB, tokenMaker, stor, hub, q, mailer, cfg, demoSeeder)

	bodyLimit := defaultBodyLimitBytes
	if cfg.BodyLimit > 0 {
		bodyLimit = cfg.BodyLimit
	}
	app := fiber.New(fiber.Config{
		ErrorHandler: createDefaultErrorHandler(bodyLimit),
		BodyLimit:    bodyLimit,
	})

	app.Use(cors.New(cors.Config{
		AllowOrigins:     []string{"http://localhost:5173", s.cfg.BaseURL.String()},
		AllowCredentials: true,
		AllowHeaders:     []string{"Origin", "Content-Type", "Authorization"},
		AllowMethods:     []string{"GET", "POST", "PUT", "PATCH", "DELETE", "OPTIONS"},
	}))

	// Health check (public)
	app.Get("/healthz", handleHealthz)

	// Public server config (demo flag, etc.)
	app.Get("/config", auth.OptionalAuth(s.tokenMaker), s.handleConfig)

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

	getRoleFn := func(ctx context.Context, workspaceID, userID string) (auth.Role, error) {
		member, err := s.db.GetMember(ctx, dbgen.GetMemberParams{
			WorkspaceID: workspaceID,
			UserID:      userID,
		})
		if err != nil {
			return "", err
		}
		return auth.Role(member.Role), nil
	}

	// Workspace settings — owner only; blocked in demo mode
	api.Put("/workspace/settings", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleUpdateWorkspaceSettings)

	// Generic job trigger — owner only
	api.Post("/workspace/jobs/:type/trigger", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleTriggerWorkspaceJob)

	// Invites — owner only; blocked in demo mode
	api.Post("/workspace/invites", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleCreateInvite)
	api.Get("/workspace/invites", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleListInvites)
	api.Delete("/workspace/invites/:inviteId", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteInvite)

	// Members — owner only
	api.Get("/workspace/members", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleListMembers)
	api.Delete("/workspace/members/:userId", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleRemoveMember)
	api.Put("/workspace/members/:userId", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleUpdateMemberRole)

	// Invite acceptance is public — the caller has no account yet
	authGroup.Post("/invite/accept", s.handleAcceptInvite)

	// Field definitions — reorder must be registered before /:id to avoid conflict
	api.Get("/field-definitions", s.handleListFieldDefinitions)
	api.Post("/field-definitions", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateFieldDefinition)
	api.Put("/field-definitions/reorder", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleReorderFieldDefinitions)
	api.Get("/field-definitions/:id", s.handleGetFieldDefinition)
	api.Get("/field-definitions/:id/stats", s.handleGetFieldDefinitionStats)
	api.Put("/field-definitions/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateFieldDefinition)
	api.Delete("/field-definitions/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleDeleteFieldDefinition)

	// Projects
	api.Post("/projects", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateProject)
	api.Get("/projects", s.handleListProjects)
	api.Get("/projects/:id", s.handleGetProject)
	api.Put("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateProject)
	api.Delete("/projects/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteProject)

	// Project field values
	api.Get("/projects/:id/fields", s.handleGetProjectFields)
	api.Patch("/projects/:id/fields", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handlePatchProjectFields)

	// Tags
	api.Get("/tags", s.handleListTags)
	api.Post("/tags", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateTag)
	api.Patch("/tags/:name", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handlePatchTag)
	api.Delete("/tags", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleBulkDeleteTags)
	api.Post("/tags/merge", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleMergeTags)
	api.Get("/tags/suggestions/duplicates", s.handleTagDuplicateSuggestions)

	// Folders
	api.Post("/projects/:id/folders", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateFolder)
	api.Get("/projects/:id/folders", s.handleGetFolders)
	api.Put("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateFolder)
	api.Delete("/folders/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteFolder)

	// Stack — export and merge
	api.Post("/stack/export", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleStackExport)
	api.Post("/stack/merge", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleStackMerge)

	// Collections
	api.Get("/collections", s.handleListCollections)
	api.Post("/collections", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateCollection)
	api.Get("/collections/:id", s.handleGetCollection)
	api.Put("/collections/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateCollection)
	api.Delete("/collections/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteCollection)
	api.Post("/collections/:id/assets/:aid", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleAddCollectionAsset)
	api.Delete("/collections/:id/assets/:aid", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleRemoveCollectionAsset)

	// Jobs — status polling (for async merge results)
	api.Get("/jobs/:id", auth.RequireAuth(tokenMaker), s.handleGetJob)

	// Assets — bulk routes must be registered before /:id to avoid conflict
	api.Post("/assets/bulk/tag", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleBulkTag)
	api.Post("/assets/bulk/project", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleBulkProject)
	api.Delete("/assets/bulk", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleBulkDelete)
	api.Patch("/assets/bulk/fields", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleBulkPatchAssetFields)

	api.Post("/assets", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUploadAsset)
	api.Get("/assets", s.handleListAssets)
	api.Patch("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateAssetFolder)
	api.Put("/assets/:id/rename", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleRenameAsset)
	api.Get("/assets/:id", s.handleGetAsset)
	api.Get("/assets/:id/comments", s.handleGetComments)
	api.Get("/assets/:id/file", s.handleGetAssetFile)
	api.Get("/assets/:id/thumb", s.handleGetAssetThumb)
	api.Delete("/assets/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleDeleteAsset)

	// Asset field values
	api.Get("/assets/:id/fields", s.handleGetAssetFields)
	api.Patch("/assets/:id/fields", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handlePatchAssetFields)

	// Asset tags
	api.Get("/assets/:id/tags", s.handleGetAssetTags)
	api.Post("/assets/:id/tags", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleAddTagToAsset)
	api.Delete("/assets/:id/tags/:name", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleRemoveTagFromAsset)

	// Variants
	api.Get("/assets/:id/variants", s.handleListVariants)
	api.Post("/assets/:id/variants", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateVariant)
	api.Post("/assets/:id/variants/upload", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUploadManualVariant)
	api.Get("/assets/:id/variants/:vid/file", s.handleGetVariantFile)
	api.Delete("/assets/:id/variants/:vid", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleDeleteVariant)

	// Asset versions
	api.Get("/assets/:id/versions", s.handleListAssetVersions)
	api.Post("/assets/:id/versions", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUploadAssetVersion)
	api.Post("/assets/:id/versions/:vid/restore", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleRestoreAssetVersion)
	api.Get("/assets/:id/versions/:vid/file", s.handleGetVersionFile)
	api.Get("/assets/:id/versions/:vid/thumb", s.handleGetVersionThumb)
	api.Delete("/assets/:id/versions/:vid", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteAssetVersion)

	// Transform preview (in-memory, no storage write)
	api.Get("/assets/:id/preview", s.handlePreviewTransform)

	// Server-Sent Events (workspace-scoped)
	api.Get("/events", s.handleSSEEvents)

	// Asset event log
	api.Get("/assets/:id/events", s.handleListAssetEvents)

	// Project event log
	api.Get("/projects/:id/events", s.handleListProjectEvents)

	// Workspace activity feed
	api.Get("/activity", s.handleListWorkspaceActivity)
	api.Get("/activity/export", s.handleExportActivity)

	// Ingress sources
	ingressGroup := api.Group("/ingress")
	ingressGroup.Post("/sources", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateIngressSource)
	ingressGroup.Get("/sources", s.handleListIngressSources)
	ingressGroup.Get("/sources/:id", s.handleGetIngressSource)
	ingressGroup.Put("/sources/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateIngressSource)
	ingressGroup.Delete("/sources/:id", auth.RequireRole(tokenMaker, getRoleFn, auth.Owner), s.handleDeleteIngressSource)
	ingressGroup.Post("/sources/:id/test", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleTestIngressSource)
	ingressGroup.Post("/sources/:id/poll", demoBlockMiddleware(), auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handlePollIngressSource)
	ingressGroup.Get("/sources/:id/log", s.handleListIngressSourceLog)
	ingressGroup.Get("/log", s.handleListIngressLog)
	ingressGroup.Delete("/log/:entry_id", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleDeleteIngressLogEntry)
	ingressGroup.Post("/log/:entry_id/retry", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleRetryIngressLogEntry)

	// Ingress rules — reorder must be registered before /:rid to avoid conflict
	ingressGroup.Get("/sources/:id/rules", s.handleListIngressRules)
	ingressGroup.Post("/sources/:id/rules", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleCreateIngressRule)
	ingressGroup.Put("/sources/:id/rules/reorder", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleReorderIngressRules)
	ingressGroup.Put("/sources/:id/rules/:rid", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleUpdateIngressRule)
	ingressGroup.Delete("/sources/:id/rules/:rid", auth.RequireRole(tokenMaker, getRoleFn, auth.Editor), s.handleDeleteIngressRule)

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
	shareGroup.Get("/export", s.handleShareExport)
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

func createDefaultErrorHandler(bodyLimitBytes int) func(c fiber.Ctx, err error) error {
	bodyLimitMb := fmt.Sprintf("%.2f", float32(bodyLimitBytes)/(1<<20))

	return func(c fiber.Ctx, err error) error {
		code := fiber.StatusInternalServerError
		if e, ok := err.(*fiber.Error); ok {
			code = e.Code
		}
		if code == fiber.StatusRequestEntityTooLarge {
			return c.Status(code).JSON(fiber.Map{"error": "file too large: maximum upload size is " + bodyLimitMb + " MB"})
		}
		return c.Status(code).JSON(fiber.Map{"error": err.Error()})
	}
}
