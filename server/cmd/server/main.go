package main

import (
	"log"

	"creativo-dam/server/internal/api"
	"creativo-dam/server/internal/auth"
	"creativo-dam/server/internal/config"
	"creativo-dam/server/internal/db"
	"creativo-dam/server/internal/storage"
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

	app := api.New(queries, sqlDB, tokenMaker, stor, cfg.AppEnv)

	log.Printf("server starting on :%s (env=%s)", cfg.Port, cfg.AppEnv)
	if err := app.Listen(":" + cfg.Port); err != nil {
		log.Fatalf("server: %v", err)
	}
}
