package api

import "github.com/gofiber/fiber/v3"

// handleHealthz returns a basic availability response.
//
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags Health
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /healthz [get]
func handleHealthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// handleConfig returns public server configuration for the frontend.
//
// @Summary Get server config
// @Description Returns public feature flags and configuration values that the frontend needs before authentication. Currently exposes the <code>demo</code> boolean which, when true, puts the server into read-only demonstration mode.
// @Tags health
// @Produce json
// @Success 200 {object} object{demo=bool}
// @Router /config [get]
// GET /config
func (s *Server) handleConfig(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"demo": s.cfg.Demo.DemoMode,
	})
}
