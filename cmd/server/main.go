package main

import (
	"context"
	"io/fs"
	"log"
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
)

var uiFS fs.FS // Populated by ui.go (prod) or ui_dev.go (dev)

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	log.Println("database:", cfg.DBPath)

	queries, sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	tokenMaker, err := auth.NewMaker(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	eventsHub := events.NewEventHub()

	stor, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("storage: %v", err)
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
		log.Printf("ingress scheduler started")
		jobs.NewFieldCleanupScheduler(queries, q).Start(ctx)
		log.Printf("field cleanup scheduler started")
		jobs.NewRetentionScheduler(q).Start(ctx)
		log.Printf("retention scheduler started")
		jobs.NewAuditLogRetentionScheduler(q).Start(ctx)
		log.Printf("audit-log retention scheduler started")
	}

	// Demo mode: ensure workspace exists on startup, seed if missing, start reset loop.
	// initDemoSeeder is a no-op stub in non-demo builds (main_nodemo.go).
	demoSeeder := initDemoSeeder(ctx, cfg, sqlDB, stor)

	app := api.NewRouter(queries, sqlDB, tokenMaker, stor, eventsHub, q, cfg, demoSeeder, uiFS)

	mail := services.NewMailServer("0.0.0.0:"+cfg.MailServerPort, cfg.BaseURL.Host, queries, q)
	log.Printf("mail server starting on :%s", cfg.MailServerPort)
	go func() {
		if err := mail.Start(); err != nil {
			log.Fatalf("mail server: %v", err)
		}
	}()

	log.Printf("api server starting on :%s (env=%s, workers=%d)", cfg.Port, cfg.AppEnv, cfg.QueueWorkers)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("api server: %v", err)
	}
}
