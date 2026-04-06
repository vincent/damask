package api

import "github.com/gofiber/fiber/v3"

// HealthCheck godoc
// @Summary Show the status of server.
// @Description get the status of server.
// @Tags health
// @Accept */*
// @Produce json
// @Success 200 {object} map[string]interface{}
// @Router /healthz [get]
func handleHealthz(c fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}

// handleConfig returns public server configuration for the frontend.
// GET /config
func (s *Server) handleConfig(c fiber.Ctx) error {
	return c.JSON(fiber.Map{
		"demo": s.cfg.Demo.DemoMode,
	})
}
