package api

import (
	"github.com/gofiber/fiber/v3"
)

type TelemetryStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name"`
	Env         string `json:"env"`
}

// @Summary Get telemetry status
// @Tags Admin
// @Produce json
// @Security BearerAuth
// @Success 200 {object} TelemetryStatusResponse
// @Router /api/v1/admin/telemetry [get].
func (s *Server) handleTelemetryStatus(c fiber.Ctx) error {
	return c.JSON(TelemetryStatusResponse{
		Enabled:     s.cfg.Telemetry.Enabled,
		ServiceName: s.cfg.Telemetry.ServiceName,
		Env:         s.cfg.Telemetry.Env,
	})
}
