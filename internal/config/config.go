package config

import (
	"errors"
	"net/url"
	"os"
	"strconv"
	"strings"

	"github.com/joho/godotenv"
)

type Config struct {
	MailServerPort  string
	MailServerHost  string
	Port            string
	DBPath          string
	StoragePath     string
	StorageType     string // "local" | "sftp"
	StorageSFTP     StorageSFTPConfig
	JWTSecret       string
	AppSecret       string
	AppEnv          string
	BaseURL         *url.URL
	RemoveBgAPIKey  string
	QueueWorkers    int
	FrontendPath    string
	EnableScheduler bool
	Demo            DemoConfig
	// BodyLimit overrides the default 100 MB upload limit. Zero means use the default.
	BodyLimit       int
}

// StorageSFTPConfig holds SFTP backend connection parameters.
type StorageSFTPConfig struct {
	Host       string
	Port       int // default 22
	User       string
	AuthMethod string // "password" | "key"
	Password   string
	PrivateKey string // PEM-encoded
	BasePath   string // remote base directory, default "/"
}

// DemoConfig holds all demo-mode settings. All fields are zero-valued when
// DemoMode is false.
type DemoConfig struct {
	DemoMode           bool
	ResetIntervalHours int
	UserEmail          string
	WorkspaceName      string
	ShowBanner         bool
	SignupURL          string
}

func Load() (*Config, error) {
	// Load .env file in development (ignore error if file doesn't exist)
	_ = godotenv.Load(".env")

	workers := 4
	if w := os.Getenv("QUEUE_WORKERS"); w != "" {
		if n, err := strconv.Atoi(w); err == nil && n > 0 {
			workers = n
		}
	}

	sftpPort := 22
	if p := os.Getenv("SFTP_PORT"); p != "" {
		if n, err := strconv.Atoi(p); err == nil && n > 0 {
			sftpPort = n
		}
	}

	demoMode := getEnv("DEMO_MODE", "false") == "true"
	demoResetHours := 6
	if h := os.Getenv("DEMO_RESET_INTERVAL_HOURS"); h != "" {
		if n, err := strconv.Atoi(h); err == nil && n > 0 {
			demoResetHours = n
		}
	}

	cfg := &Config{
		Port:           getEnv("PORT", "8080"),
		MailServerPort: getEnv("MAIL_PORT", "2525"),
		DBPath:         getEnv("DB_PATH", "./damask.db"),
		StoragePath:    getEnv("STORAGE_PATH", "./storage"),
		StorageType:    getEnv("STORAGE", "local"),
		StorageSFTP: StorageSFTPConfig{
			Host:       os.Getenv("SFTP_HOST"),
			Port:       sftpPort,
			User:       os.Getenv("SFTP_USER"),
			AuthMethod: getEnv("SFTP_AUTH_METHOD", "password"),
			Password:   os.Getenv("SFTP_PASSWORD"),
			PrivateKey: os.Getenv("SFTP_PRIVATE_KEY"),
			BasePath:   getEnv("SFTP_BASE_PATH", "/"),
		},
		JWTSecret:       os.Getenv("JWT_SECRET"),
		AppSecret:       os.Getenv("APP_SECRET"),
		AppEnv:          getEnv("APP_ENV", "development"),
		RemoveBgAPIKey:  os.Getenv("REMOVEBG_API_KEY"),
		QueueWorkers:    workers,
		FrontendPath:    os.Getenv("FRONTEND_PATH"),
		EnableScheduler: getEnv("ENABLE_SCHEDULER", "true") != "false",
		Demo: DemoConfig{
			DemoMode:           demoMode,
			ResetIntervalHours: demoResetHours,
			UserEmail:          getEnv("DEMO_USER_EMAIL", "demo@damask.studio"),
			WorkspaceName:      getEnv("DEMO_WORKSPACE_NAME", "Demo Agency"),
			ShowBanner:         demoMode && getEnv("DEMO_BANNER", "true") != "false",
			SignupURL:          getEnv("DEMO_SIGNUP_URL", "/signup"),
		},
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

	mailHost := strings.TrimSpace(os.Getenv("MAIL_HOST"))
	if len(mailHost) == 0 {
		mailHost = "ingress." + baseURL.Host
	}
	cfg.MailServerHost = mailHost

	return cfg, nil
}

func getEnv(key, defaultVal string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return defaultVal
}
