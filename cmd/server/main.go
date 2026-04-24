package main

import (
	"context"
	"io/fs"
	"log/slog"
	"os"
	"os/signal"
	"syscall"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/ingress"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/queue"
	services "damask/server/internal/fileproc"
	"damask/server/internal/storage"

	// Side-effect imports to register ingress source types
	canvasrc "damask/server/internal/ingress/sources/canva"
	_ "damask/server/internal/ingress/sources/dav"
	_ "damask/server/internal/ingress/sources/email_api"
	gdrivesrc "damask/server/internal/ingress/sources/gdrive"
	_ "damask/server/internal/ingress/sources/imap"
	_ "damask/server/internal/ingress/sources/s3"
	_ "damask/server/internal/ingress/sources/sftp"
	oauthpkg "damask/server/internal/oauth"

	"github.com/gofiber/fiber/v3"
	"golang.org/x/oauth2"
	"golang.org/x/oauth2/google"
)

var uiFS fs.FS // Populated by ui.go (prod) or ui_dev.go (dev)

func main() {
	logLevel := new(slog.LevelVar)
	handler := slog.NewJSONHandler(os.Stderr, &slog.HandlerOptions{Level: logLevel})
	slog.SetDefault(slog.New(handler))
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

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "error", err)
		os.Exit(1)
	}

	slog.Info("database", "path", cfg.DBPath)

	queries, sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		slog.Error("database", "error", err)
		os.Exit(1)
	}
	defer sqlDB.Close()

	tokenMaker, err := auth.NewMaker(cfg.JWTSecret)
	if err != nil {
		slog.Error("auth", "error", err)
		os.Exit(1)
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

	q.Start(ctx)
	defer q.Stop()

	js := jobs.NewJobServer(queries, sqlDB, stor, eventsHub, q, mailer, cfg)
	js.RegisterJobHandlers()

	if cfg.EnableScheduler {
		ingress.NewScheduler(queries, q).Start(ctx)
		slog.Info("ingress scheduler started")
		jobs.NewFieldCleanupScheduler(queries, q).Start(ctx)
		slog.Info("field cleanup scheduler started")
		jobs.NewRetentionScheduler(q).Start(ctx)
		slog.Info("retention scheduler started")
		jobs.NewAuditLogRetentionScheduler(q).Start(ctx)
		slog.Info("audit-log retention scheduler started")
	}

	// Demo mode: ensure workspace exists on startup, seed if missing, start reset loop.
	// initDemoSeeder is a no-op stub in non-demo builds (main_nodemo.go).
	demoSeeder := initDemoSeeder(ctx, cfg, sqlDB, stor)

	app := api.NewRouter(queries, sqlDB, tokenMaker, stor, eventsHub, q, mailer, cfg, demoSeeder, uiFS)

	mail := services.NewMailServer("0.0.0.0:"+cfg.MailServerPort, cfg.BaseURL.Host, queries, q)
	slog.Info("mail server starting", "port", cfg.MailServerPort)
	mailErr := make(chan error, 1)
	go func() {
		if err := mail.Start(); err != nil {
			mailErr <- err
		}
	}()

	config.InitOIDCProviders(cfg)

	// Wire TokenRefresher into OAuth-backed ingress sources.
	refresher := oauthpkg.NewTokenRefresher(queries, cfg.AppSecret)
	if cfg.Google.ClientID != "" {
		slog.Info("register Google tokens refresher")
		refresher.RegisterProvider("google", &oauth2.Config{
			ClientID:     cfg.Google.ClientID,
			ClientSecret: cfg.Google.ClientSecret,
			Endpoint:     google.Endpoint,
			RedirectURL:  cfg.BaseURL.String() + "/integrations/callback/google",
			Scopes:       []string{"openid", "email", "profile", "https://www.googleapis.com/auth/drive.readonly"},
		})
	}
	if cfg.Canva.ClientID != "" {
		slog.Info("register Canva tokens refresher")
		refresher.RegisterProvider("canva", &oauth2.Config{
			ClientID:     cfg.Canva.ClientID,
			ClientSecret: cfg.Canva.ClientSecret,
			Endpoint: oauth2.Endpoint{
				AuthURL:  "https://www.canva.com/api/oauth/authorize",
				TokenURL: "https://api.canva.com/rest/v1/oauth/token",
			},
			RedirectURL: cfg.BaseURL.String() + "/integrations/callback/canva",
			Scopes:      []string{"profile:read", "design:content:read"},
		})
	}
	gdrivesrc.SetRefresher(refresher)
	canvasrc.SetRefresher(refresher)

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
