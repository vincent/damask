//go:build desktop

package main

import (
	"context"
	"fmt"
	"io/fs"
	"log/slog"
	"net"
	"time"

	"damask/server/internal/api"
	"damask/server/internal/auth"
	"damask/server/internal/config"
	"damask/server/internal/db"
	"damask/server/internal/events"
	"damask/server/internal/imagerouter"
	"damask/server/internal/jobs"
	"damask/server/internal/mail"
	"damask/server/internal/media/ingest"
	"damask/server/internal/queue"
	reposqlc "damask/server/internal/repository/sqlc"
	"damask/server/internal/service"
	"damask/server/internal/storage"
	"damask/server/internal/transform"

	"github.com/pkg/browser"
	"github.com/wailsapp/wails/v3/pkg/application"
)

// App holds all server-side state for the Wails desktop application.
type App struct {
	cfg        *config.Config
	configDir  string
	serverPort int
	stopServer context.CancelFunc
	stopQueue  func()
	wailsApp   *application.App
}

// Startup is called by Wails after the window is ready.
// It starts the Fiber HTTP server in a goroutine bound to loopback only.
func (a *App) Startup(ctx context.Context) error {
	queries, sqlDB, err := db.Open(a.cfg.DBPath)
	if err != nil {
		return fmt.Errorf("desktop: open db: %w", err)
	}

	desktopUI, err := fs.Sub(assets, "frontend/dist")
	if err != nil {
		return fmt.Errorf("desktop: frontend assets: %w", err)
	}

	tokenMaker, err := auth.NewMaker(a.cfg.JWTSecret)
	if err != nil {
		return fmt.Errorf("desktop: auth maker: %w", err)
	}

	eventsHub := events.NewEventHub()
	mailer := mail.NewMailer(&a.cfg.MailSenderConfig)

	stor, err := buildStorage(a.cfg)
	if err != nil {
		return fmt.Errorf("desktop: storage: %w", err)
	}

	q := queue.New(queries, a.cfg.QueueWorkers)
	q.Start(ctx)
	a.stopQueue = q.Stop

	trf := transform.NewTransformer(a.cfg.FFmpeg)
	tmb := transform.NewThumbnailer(trf)
	media := ingest.NewRegistry(trf)
	injestor := service.NewAssetInjestor(queries, sqlDB, stor, q, media)
	workspaceRepo := reposqlc.NewWorkspaceRepo(queries, sqlDB)
	resolveImageRouterKey := imagerouter.NewKeyResolver(workspaceRepo, a.cfg.AppSecret, a.cfg.ImageRouter.APIKey)
	jobs.NewJobServer(queries, sqlDB, stor, eventsHub, q, mailer, trf, tmb, a.cfg, injestor, resolveImageRouterKey).RegisterJobHandlers()

	addr := fmt.Sprintf("127.0.0.1:%d", a.serverPort)

	_, cancel := context.WithCancel(ctx)
	a.stopServer = cancel

	go func() {
		slog.Info("desktop: server starting", "addr", addr)
		fiberApp := api.NewRouter(queries, sqlDB, tokenMaker, stor, eventsHub, q, mailer, trf, a.cfg, a.configDir, nil, desktopUI)
		if err := fiberApp.Listen(addr); err != nil {
			slog.Error("desktop: server error", "err", err)
		}
	}()

	// Wait up to 5 s for the port to be ready before the window loads.
	if err := waitForPort(addr, 5*time.Second); err != nil {
		slog.Warn("desktop: server did not start in time", "err", err)
	}

	_ = media
	return nil
}

// Shutdown is called by Wails on application quit.
// Gracefully stops the server with a 10-second deadline.
func (a *App) Shutdown() error {
	if a.stopServer != nil {
		a.stopServer()
	}
	if a.stopQueue != nil {
		a.stopQueue()
	}
	return nil
}

// ServerPort returns the port the embedded server is listening on.
// Exposed to the frontend via Wails bindings.
func (a *App) ServerPort() int {
	return a.serverPort
}

// OpenBrowser opens a URL in the system browser.
// Used by the deps wizard to open install docs.
func (a *App) OpenBrowser(url string) {
	if err := browser.OpenURL(url); err != nil {
		slog.Warn("desktop: open browser", "err", err)
	}
}

// ConfigDir returns the config directory path.
func (a *App) ConfigDir() string {
	return a.configDir
}

// PickDirectory opens a native directory picker when running in desktop mode.
func (a *App) PickDirectory() string {
	if a.wailsApp == nil {
		return ""
	}

	dir, err := a.wailsApp.Dialog.OpenFile().
		SetTitle("Choose storage folder").
		CanChooseDirectories(true).
		CanChooseFiles(false).
		PromptForSingleSelection()
	if err != nil {
		slog.Warn("desktop: pick directory", "err", err)
		return ""
	}
	return dir
}

func (a *App) serverURL(path string) string {
	return fmt.Sprintf("http://localhost:%d%s", a.serverPort, path)
}

// waitForPort dials addr in a loop until it succeeds or deadline.
func waitForPort(addr string, timeout time.Duration) error {
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		conn, err := net.DialTimeout("tcp", addr, 100*time.Millisecond)
		if err == nil {
			conn.Close()
			return nil
		}
		time.Sleep(50 * time.Millisecond)
	}
	return fmt.Errorf("timeout waiting for %s", addr)
}

func buildStorage(cfg *config.Config) (storage.Storage, error) {
	switch cfg.StorageType {
	case "s3":
		return storage.NewAferoS3Storage(cfg.StorageS3)
	case "sftp":
		return storage.NewSFTPStorage(cfg.StorageSFTP)
	default:
		return storage.NewLocalStorage(cfg.StoragePath)
	}
}
