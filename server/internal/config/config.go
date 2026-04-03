package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"

	"github.com/joho/godotenv"
)

type Config struct {
	Port            string
	DBPath          string
	StoragePath     string
	JWTSecret       string
	AppSecret       string
	AppEnv          string
	BaseURL         *url.URL
	RemoveBgAPIKey  string
	QueueWorkers    int
	FrontendPath    string
	EnableScheduler bool
}

func Load() (*Config, error) {
	// Load .env file in development (ignore error if file doesn't exist)
	_ = godotenv.Load("../.env")

	workers := 4
	if w := os.Getenv("QUEUE_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			workers = n
		}
	}

	cfg := &Config{
		Port:            getEnv("PORT", "8080"),
		DBPath:          getEnv("DB_PATH", "./damask.db"),
		StoragePath:     getEnv("STORAGE_PATH", "./storage"),
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AppSecret:       os.Getenv("APP_SECRET"),
		AppEnv:          getEnv("APP_ENV", "development"),
		RemoveBgAPIKey:  os.Getenv("REMOVEBG_API_KEY"),
		QueueWorkers:    workers,
		FrontendPath:    os.Getenv("FRONTEND_PATH"),
		EnableScheduler: getEnv("ENABLE_SCHEDULER", "true") != "false",
	}

	if cfg.JWTSecret == "" {
		return nil, errors.New("JWT_SECRET env var is required")
	}

	if cfg.AppSecret == "" {
		return nil, errors.New("APP_SECRET env var is required")
	}

	baseURL, err := url.Parse(getEnv("BASE_URL", "http://localhost:5173"))
	if err != nil {
		return nil, errors.New("BASE_URL env var is required")
	}
	cfg.BaseURL = baseURL

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
