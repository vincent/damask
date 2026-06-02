package api

import (
	"damask/server/internal/auth"
	"damask/server/internal/config"

	"github.com/gofiber/fiber/v3"
)

// handleHealthz returns a basic availability response.
//
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags Config
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /healthz [get].
func handleHealthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{apiStatusKey: "ok"})
}

// handleSSEEvents streams workspace-scoped Server-Sent Events to the caller.
//
// @Summary Subscribe to real-time events
// @Description Opens a persistent Server-Sent Events (SSE) stream scoped to the authenticated user's workspace. The connection stays open until the client disconnects. A keep-alive comment is sent every 25 seconds to prevent proxy timeouts.<br><br> Each event is delivered as a standard SSE <code>data:</code> line containing a JSON object with at minimum an <code>event</code> field. Current event types:<br><ul><li><strong>thumbnail_ready</strong> — emitted after an image or video thumbnail job completes; payload contains <code>asset_id</code>.</li></ul> Clients should reconnect automatically on disconnect using the browser's native <code>EventSource</code> API or equivalent.
// @Tags Events
// @Produce text/event-stream
// @Security BearerAuth
// @Success 200 {string} string "SSE stream"
// @Failure 401 {object} ErrorResponse "Not authenticated"
// @Router /api/v1/events [get].
func (s *Server) handleSSEEvents(c fiber.Ctx) error {
	return s.hub.EventHandler(c)
}

// handleConfig returns public server configuration for the frontend.
//
// @Summary Get server config
// @Description Returns public feature flags and configuration values that the frontend needs before authentication. Currently exposes the <code>demo</code> boolean which, when true, puts the server into read-only demonstration mode.
// @Tags Config
// @Produce json
// @Success 200 {object} object{demo=bool, mailHost=string}
// @Router /config [get]
// GET /config.
func (s *Server) handleConfig(c fiber.Ctx) error {
	out := fiber.Map{
		"demo": s.cfg.Demo.DemoMode,
	}
	if claims := auth.GetClaims(c); claims != nil {
		out["mailHost"] = s.cfg.MailServerHost
		if ws, err := s.workspace.Get(c.Context(), claims.WorkspaceID); err == nil {
			out["exif_keep"] = ws.ExifKeep
		}
	}
	return c.JSON(out)
}

// handleAuthConfig returns which login methods are enabled. Public, reads only config.
func (s *Server) handleAuthConfig(c fiber.Ctx) error {
	oidcRT := config.GetOIDCRuntime()
	googleRT := config.GetGoogleRuntime()
	return c.JSON(fiber.Map{
		"password_auth":  true,
		"signup_enabled": s.cfg.EnableSignup,
		"oidc_enabled":   oidcRT != nil,
		"oidc_label":     s.cfg.OIDC.Label,
		"google_enabled": googleRT != nil && s.cfg.Google.Auth,
		"canva_enabled":  s.cfg.Canva.ClientID != "" && s.cfg.Canva.Auth,
	})
}
