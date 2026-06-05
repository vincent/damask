package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/audit"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/imagerouter"
	"damask/server/internal/ingress"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/mailserver"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/telemetry"
	"damask/server/internal/transform"
	"damask/server/internal/workflow"

	// Side-effect imports to register ingress source types.
	"damask/server/internal/ingress/sources/canva"
	_ "damask/server/internal/ingress/sources/dav"
	_ "damask/server/internal/ingress/sources/email_api"
	"damask/server/internal/ingress/sources/gdrive"
	_ "damask/server/internal/ingress/sources/imap"
	_ "damask/server/internal/ingress/sources/s3"
	_ "damask/server/internal/ingress/sources/sftp"

	// Side-effect imports to register export destination types.
	exportgdrive "damask/server/internal/export/destinations/gdrive"
	_ "damask/server/internal/export/destinations/sftp"
	"damask/server/internal/oauth"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var uiFS fs.FS // Populated by ui.go (prod) or ui_dev.go (dev)

func main() {
	logLevel := new(slog.LevelVar)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	switch os.Getenv("LOG_LEVEL") {
	case "DEBUG":
		logLevel.Set(slog.LevelDebug)
	case "WARN":
		logLevel.Set(slog.LevelWarn)
	case "ERROR":
		logLevel.Set(slog.LevelError)
	default:
		logLevel.Set(slog.LevelInfo)
	}
	slog.Info("log", "level", logLevel.Level())

	slog.Info("database", "path", cfg.DBPath)
	queries, sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	service.VerifySizeColumns(context.Background(), sqlDB, slog.Default())

	tokenMaker, err := auth.NewMaker(cfg.JWTSecret)
	if err != nil {
		slog.Error("auth", "error", err)
		os.Exit(1) //nolint: gocritic // Defered sqlDB.Close() is not needed on exit.
	}

	eventsHub := events.NewEventHub()

	mailer := mail.NewMailer(&cfg.MailSenderConfig)

	var stor storage.Storage
	switch cfg.StorageType {
	case "memory":
		slog.Info("storage", "using", cfg.StorageType)
		stor, err = storage.NewAferoMemoryStorage()
	case "s3":
		slog.Info("storage", "using", cfg.StorageType, "bucket", cfg.StorageS3.Bucket)
		stor, err = storage.NewAferoS3Storage(cfg.StorageS3)
	case "sftp":
		slog.Info("storage", "using", cfg.StorageType, "host", cfg.StorageSFTP.Host)
		stor, err = storage.NewSFTPStorage(cfg.StorageSFTP)
	default: // "local" and anything unrecognized
		slog.Info("storage", "using", cfg.StorageType, "host", cfg.StoragePath)
		stor, err = storage.NewLocalStorage(cfg.StoragePath)
	}
	if err != nil {
		slog.Error("storage", "error", err)
		os.Exit(1)
	}

	q := queue.New(queries, cfg.QueueWorkers)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	tmConfig := telemetry.Config{
		Enabled:     cfg.Telemetry.Enabled,
		Endpoint:    cfg.Telemetry.Endpoint,
		Token:       cfg.Telemetry.Token,
		ServiceName: cfg.Telemetry.ServiceName,
		Env:         cfg.Telemetry.Env,
	}
	telemetry.SetupMetrics(tmConfig)

	otelTracesShutdown, err := telemetry.SetupTraces(ctx, tmConfig)
	if err != nil {
		slog.WarnContext(ctx, "otel setup failed; continuing without sending traces", "error", err)
	}

	otelLogsShutdown, err := telemetry.SetupLogs(ctx, tmConfig)
	if err != nil {
		slog.WarnContext(ctx, "otel setup failed; continuing without sending logs", "error", err)
	}

	defer func() {
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := otelTracesShutdown(shutdownCtx); err != nil {
			slog.WarnContext(shutdownCtx, "otel shutdown failed", "error", err)
		}
		if err := otelLogsShutdown(shutdownCtx); err != nil {
			slog.WarnContext(shutdownCtx, "otel shutdown failed", "error", err)
		}
	}()

	q.Start(ctx)
	defer q.Stop()

	// --- transform ---
	trf := transform.NewTransformer(cfg.FFmpeg)
	tmb := transform.NewThumbnailer(trf)
	media := ingest.NewRegistry(trf)

	// --- repositories ---
	workspaceRepo := reposqlc.NewWorkspaceRepo(queries, sqlDB)
	assetRepo := reposqlc.NewAssetRepo(queries, sqlDB)
	tagRepo := reposqlc.NewTagRepo(queries, sqlDB)
	fieldRepo := reposqlc.NewFieldRepo(queries, sqlDB)
	userRepo := reposqlc.NewUserRepo(queries, sqlDB)
	versionRepo := reposqlc.NewVersionRepo(queries, sqlDB)
	variantRepo := reposqlc.NewVariantRepo(sqlDB)
	assetFieldRepo := reposqlc.NewAssetFieldRepo(queries, sqlDB)
	workflowRepo := reposqlc.NewWorkflowRepo(queries, sqlDB)
	workflowRunRepo := reposqlc.NewWorkflowRunRepo(queries, sqlDB)

	// --- services ---
	auditWriter := audit.New(sqlDB)
	resolveImageRouterKey := imagerouter.NewKeyResolver(workspaceRepo, cfg.AppSecret, cfg.ImageRouter.APIKey)
	ingester := service.NewAssetIngester(queries, sqlDB, stor, q, media)
	tagSvc := service.NewTagService(tagRepo, auditWriter, service.TagServiceDeps{
		Assets: assetRepo,
	})
	variantSvc := service.NewVariantServiceWithDeps(
		variantRepo,
		assetRepo,
		tagSvc,
		auditWriter,
		service.VariantServiceDeps{
			Actions: service.NewSQLVariantActionsStore(sqlDB),
			Queue:   q,
			Storage: stor,
		},
	)
	fieldSvc := service.NewFieldService(fieldRepo)
	assetSvc := service.NewAssetService(assetRepo, versionRepo, tagRepo, fieldRepo, stor, auditWriter, q)
	assetFieldSvc := service.NewAssetFieldService(assetRepo, fieldRepo, assetFieldRepo, auditWriter)
	shareSvc := service.NewShareService(reposqlc.NewShareRepo(queries, sqlDB), auditWriter)
	workspaceSvc := service.NewWorkspaceService(workspaceRepo, userRepo, cfg.AppSecret, cfg.ImageRouter.APIKey)
	exportSvc := service.NewExportService(queries, sqlDB, stor, cfg.AppSecret, q)
	exifSvc := service.NewExifService(queries, stor)
	textTrackSvc := service.NewTextTrackService(queries, q, stor)
	storageSvc := service.NewStorageService(queries)

	// --- workflow executor ---
	workflowExec := workflow.NewExecutor(workflow.Deps{
		Workflows:     workflowRepo,
		Runs:          workflowRunRepo,
		Queue:         q,
		Storage:       stor,
		Mailer:        mailer,
		Hub:           eventsHub,
		Audit:         auditWriter,
		Assets:        newAssetManager(assetSvc),
		Variants:      newVariantManager(variantSvc),
		Versions: newVersionManager(versionRepo, variantRepo),
		Shares:        newShareManager(shareSvc),
		Tags:          newTagManager(tagSvc),
		AssetFields:   newAssetFieldManager(assetFieldSvc),
		Workspace:     newWorkspaceManager(workspaceSvc),
		Config:        cfg,
	})

	// --- job server ---
	js := jobs.NewJobServer(
		queries,
		sqlDB,
		stor,
		eventsHub,
		q,
		mailer,
		trf,
		tmb,
		cfg,
		ingester,
		resolveImageRouterKey,
		workflowExec,
		exportSvc,
		exifSvc,
		fieldSvc,
		textTrackSvc,
		storageSvc,
	)
	js.RegisterJobHandlers()

	// --- http ---
	// Demo mode: ensure workspace exists on startup, seed if missing, start reset loop.
	// initDemoSeeder is a no-op stub in non-demo builds (main_nodemo.go).
	demoSeeder := initDemoSeeder(ctx, cfg, sqlDB, stor, trf, tmb)

	app := api.NewRouter(queries, sqlDB, tokenMaker, stor, eventsHub, q, mailer, trf, cfg, demoSeeder, uiFS, storageSvc)

	mail := mailserver.NewMailServer("0.0.0.0:"+cfg.MailServerPort, cfg.BaseURL.Host, queries, q)
	slog.Info("mail server starting", "port", cfg.MailServerPort)
	mailErr := make(chan error, 1)
	go func() {
		if err := mail.Start(); err != nil {
			mailErr <- err
		}
	}()

	config.InitOIDCProviders(cfg)

	// Wire TokenRefresher into OAuth-backed ingress sources.
	refresher := oauth.NewTokenRefresher(queries, cfg.AppSecret)
	if cfg.Google.ClientID != "" {
		slog.Info("register Google tokens refresher")
		refresher.RegisterProvider("google", &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  cfg.BaseURL.String() + "/integrations/callback/google",
			Scopes:       []string{"openid", "email", "profile", "https://www.googleapis.com/auth/drive.file"},
		})
	}
	if cfg.Canva.ClientID != "" {
		slog.Info("register Canva tokens refresher")
		refresher.RegisterProvider("canva", &oauth2.Config{
			ClientID:     cfg.Canva.ClientID,
			ClientSecret: cfg.Canva.ClientSecret,
			Endpoint: oauth2.Endpoint{ //nolint:gosec // canva api endpoints
				AuthURL:  "https://www.canva.com/api/oauth/authorize",
				TokenURL: "https://api.canva.com/rest/v1/oauth/token",
			},
			RedirectURL: cfg.BaseURL.String() + "/integrations/callback/canva",
			Scopes:      []string{"profile:read", "design:content:read"},
		})
	}
	gdrive.SetRefresher(refresher)
	canva.SetRefresher(refresher)
	exportgdrive.SetRefresher(refresher)

	/// start background services

	if cfg.EnableScheduler {
		ingress.NewScheduler(queries, q).Start(ctx)
		slog.Info("ingress scheduler started")
		jobs.NewFieldCleanupScheduler(queries, q).Start(ctx)
		slog.Info("field cleanup scheduler started")
		jobs.NewRetentionScheduler(q).Start(ctx)
		slog.Info("retention scheduler started")
		jobs.NewAuditLogRetentionScheduler(q).Start(ctx)
		slog.Info("audit-log retention scheduler started")
		jobs.NewScratchPurgeScheduler(q, cfg).Start(ctx)
		slog.Info("scratch purge scheduler started")
		jobs.NewExportScheduler(q, js).Start(ctx)
		slog.Info("export scheduler started")
	}

	slog.Info("api server starting", "port", cfg.Port, "env", cfg.AppEnv, "workers", cfg.QueueWorkers)
	appErr := make(chan error, 1)
	go func() {
		if err := app.Listen(":"+cfg.Port, fiber.ListenConfig{
			DisableStartupMessage: true,
		}); err != nil {
			appErr <- err
		}
	}()

	select {
	case err := <-mailErr:
		slog.Error("mail server", "error", err)
		os.Exit(1)
	case err := <-appErr:
		slog.Error("api server", "error", err)
		os.Exit(1)
	case <-ctx.Done():
	}
}
