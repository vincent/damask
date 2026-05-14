//go:build desktop

package main

import (
	"flag"
	"fmt"
	"log/slog"
	"os"
	"strconv"

	"damask/server/internal/config"
	"damask/server/internal/desktopconfig"

	"github.com/joho/godotenv"
	"github.com/pkg/browser"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// version is set at build time via -ldflags "-X main.version=$(git describe --tags --always)".
var version = "dev"

func main() {
	var (
		flagSetup     = flag.Bool("setup", false, "force setup wizard")
		flagConfigDir = flag.String("config", "", "alternate config directory")
		flagPort      = flag.Int("port", 0, "override server port")
	)
	flag.Parse()

	configDir, err := desktopconfig.ConfigDir(*flagConfigDir)
	if err != nil {
		slog.Error("config dir", "err", err)
		os.Exit(1)
	}

	// Single-instance check: acquire a file lock.
	lock, err := acquireLock(configDir)
	if err != nil {
		// Another instance is running — open the browser and exit.
		port := runningPort(configDir)
		if port > 0 {
			_ = browser.OpenURL(fmt.Sprintf("http://localhost:%d", port))
		}
		os.Exit(0)
	}
	defer lock.Release()

	// Load damask.env into the process environment so config.Load() picks it up.
	if exists, _ := desktopconfig.Exists(*flagConfigDir); exists {
		if err := godotenv.Overload(mustConfigFilePath(*flagConfigDir)); err != nil {
			slog.Warn("godotenv", "err", err)
		}
	}

	// Port flag beats config file.
	if *flagPort > 0 {
		os.Setenv("PORT", fmt.Sprintf("%d", *flagPort))
	}
	if os.Getenv("PORT") == "" {
		os.Setenv("PORT", "14000")
	}

	cfg, err := config.Load()
	if err != nil {
		slog.Error("config", "err", err)
		os.Exit(1)
	}

	port, err := strconv.Atoi(cfg.Port)
	if err != nil || port <= 0 {
		port = 14000
	}

	icon, err := assets.ReadFile("frontend/dist/damask.logo.png")
	if err != nil {
		slog.Warn("desktop: app icon", "err", err)
	}

	app := &App{
		cfg:        cfg,
		configDir:  configDir,
		serverPort: port,
	}

	needsSetup := false
	if exists, _ := desktopconfig.Exists(*flagConfigDir); !exists || *flagSetup {
		needsSetup = true
	}

	wailsApp := application.New(application.Options{
		Name:        "Damask",
		Description: "Self-hosted digital asset management",
		Icon:        icon,
		Services: []application.Service{
			application.NewService(app),
		},
		Assets: application.AssetOptions{
			Handler: application.BundledAssetFileServer(assets),
		},
	})
	app.wailsApp = wailsApp

	window := wailsApp.Window.NewWithOptions(application.WebviewWindowOptions{
		Title:           "Damask",
		Width:           1280,
		MinWidth:        1280,
		Height:          800,
		MinHeight:       800,
		EnableFileDrop:  true,
		InitialPosition: application.WindowCentered,
	})

	setupTray(wailsApp, window, app)

	startURL := "/library"
	if needsSetup {
		startURL = "/setup"
	}
	window.SetURL(app.serverURL(startURL))

	if err := app.Startup(wailsApp.Context()); err != nil {
		slog.Error("startup", "err", err)
		os.Exit(1)
	}

	if err := wailsApp.Run(); err != nil {
		slog.Error("wails", "err", err)
		os.Exit(1)
	}
}

func mustConfigFilePath(override string) string {
	p, err := desktopconfig.ConfigFilePath(override)
	if err != nil {
		return ""
	}
	return p
}

// runningPort reads PORT from the existing config file, returns 0 on any error.
func runningPort(configDir string) int {
	m, err := desktopconfig.Load(configDir)
	if err != nil {
		return 0
	}
	var port int
	fmt.Sscanf(m["PORT"], "%d", &port)
	return port
}
