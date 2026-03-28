package config

import (
	"errors"
	"os"

	"github.com/joho/godotenv"
)

type Config struct {
	Port        string
	DBPath      string
	StoragePath string
	JWTSecret   string
	AppEnv      string
}

func Load() (*Config, error) {
	// Load .env file in development (ignore error if file doesn't exist)
	_ = godotenv.Load()

	cfg := &Config{
		Port:        getEnv("PORT", "8080"),
		DBPath:      getEnv("DB_PATH", "./creativo.db"),
		StoragePath: getEnv("STORAGE_PATH", "./storage"),
		JWTSecret:   os.Getenv("JWT_SECRET"),
		AppEnv:      getEnv("APP_ENV", "development"),
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET env var is required")
	}

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
