package main

import (
	"context"
	"log"
	"os"
	"os/signal"
	"syscall"

	"badam/server/internal/api"
	"badam/server/internal/auth"
	"badam/server/internal/config"
	"badam/server/internal/db"
	"badam/server/internal/queue"
	"badam/server/internal/storage"
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

	app := api.New(queries, sqlDB, tokenMaker, stor, q, cfg.RemoveBgAPIKey, cfg.AppEnv)

	log.Printf("server starting on :%s (env=%s, workers=%d)", cfg.Port, cfg.AppEnv, cfg.QueueWorkers)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
