//go:build desktop

package main

import (
	"log/slog"

	"damask/server/internal/desktopconfig"

	"github.com/wailsapp/wails/v3/pkg/application"
	"github.com/wailsapp/wails/v3/pkg/events"
)

// setupTray creates the system tray menu and wires window-close behaviour.
func setupTray(wailsApp *application.App, window *application.WebviewWindow, app *App) {
	tray := wailsApp.SystemTray.New()
	tray.SetLabel("Damask")

	menu := wailsApp.NewMenu()
	menu.Add("Open Damask").OnClick(func(_ *application.Context) {
		window.Show()
		window.Focus()
	})
	menu.AddSeparator()
	menu.Add("Reconfigure…").OnClick(func(_ *application.Context) {
		if err := desktopconfig.BackupAndWipe(app.configDir, 5); err != nil {
			slog.Error("tray: reconfigure wipe", "err", err)
			return
		}
		window.Show()
		window.SetURL(app.serverURL("/setup"))
	})
	menu.AddSeparator()
	menu.Add("Quit").OnClick(func(_ *application.Context) {
		wailsApp.Quit()
	})
	tray.SetMenu(menu)

	// Hide window on close instead of destroying it.
	window.RegisterHook(events.Common.WindowClosing, func(e *application.WindowEvent) {
		e.Cancel()
		window.Hide()
	})
}
