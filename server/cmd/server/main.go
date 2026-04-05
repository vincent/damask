package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/db"
	"damask/server/internal/ingress"
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

func main() {
	cfg, err := config.Load()
	if err != nil {
		log.Fatalf("config: %v", err)
	}

	queries, sqlDB, err := db.Open(cfg.DBPath)
	if err != nil {
		log.Fatalf("database: %v", err)
	}
	defer sqlDB.Close()

	tokenMaker, err := auth.NewMaker(cfg.JWTSecret)
	if err != nil {
		log.Fatalf("auth: %v", err)
	}

	stor, err := storage.NewLocalStorage(cfg.StoragePath)
	if err != nil {
		log.Fatalf("storage: %v", err)
	}

	q := queue.New(queries, cfg.QueueWorkers)

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	q.Start(ctx)
	defer q.Stop()

	// Register ingress job handlers
	ingressWorker := ingress.NewWorker(queries, stor, q, cfg.AppSecret)
	q.Register(queue.JobTypeIngestPoll, ingressWorker.HandlePoll)
	q.Register(queue.JobTypeIngestFetch, ingressWorker.HandleFetch)

	// Start ingress scheduler (disabled in tests via ENABLE_SCHEDULER=false)
	if cfg.EnableScheduler {
		scheduler := ingress.NewScheduler(queries, q)
		scheduler.Start(ctx)
		log.Printf("ingress scheduler started")

		fieldCleanup := api.NewFieldCleanupScheduler(queries, q)
		fieldCleanup.Start(ctx)
		log.Printf("field cleanup scheduler started")
	}

	app := api.New(queries, sqlDB, tokenMaker, stor, q, cfg.RemoveBgAPIKey, cfg.AppEnv, cfg.BaseURL.String(), cfg.FrontendPath, cfg.AppSecret)

	mail := services.NewMailServer("0.0.0.0:2525", cfg.BaseURL.Host, queries, q)
	log.Printf("mail server starting on :%s", "2525")
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
