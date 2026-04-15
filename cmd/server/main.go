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
	"damask/server/internal/queue"
	"damask/server/internal/services"
	"damask/server/internal/storage"

	// Side-effect imports to register ingress source types
	_ "damask/server/internal/ingress/sources/dav"
	_ "damask/server/internal/ingress/sources/email_api"
	_ "damask/server/internal/ingress/sources/imap"
	_ "damask/server/internal/ingress/sources/s3"
	_ "damask/server/internal/ingress/sources/sftp"

	"github.com/gofiber/fiber/v3"
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

	slog.Info("storage", "using", cfg.StorageType)
	var stor storage.Storage
	switch cfg.StorageType {
	case "sftp":
		stor, err = storage.NewSFTPStorage(storage.SFTPConfig{
			Host:       cfg.StorageSFTP.Host,
			Port:       cfg.StorageSFTP.Port,
			User:       cfg.StorageSFTP.User,
			AuthMethod: cfg.StorageSFTP.AuthMethod,
			Password:   cfg.StorageSFTP.Password,
			PrivateKey: cfg.StorageSFTP.PrivateKey,
			BasePath:   cfg.StorageSFTP.BasePath,
		})
	default: // "local" and anything unrecognized
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

	js := jobs.NewJobServer(queries, sqlDB, stor, eventsHub, q, cfg)
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

	app := api.NewRouter(queries, sqlDB, tokenMaker, stor, eventsHub, q, cfg, demoSeeder, uiFS)

	mail := services.NewMailServer("0.0.0.0:"+cfg.MailServerPort, cfg.BaseURL.Host, queries, q)
	slog.Info("mail server starting", "port", cfg.MailServerPort)
	mailErr := make(chan error, 1)
	go func() {
		if err := mail.Start(); err != nil {
			mailErr <- err
		}
	}()

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
