package api

import (
	"github.com/gofiber/fiber/v3"
)

type TelemetryStatusResponse struct {
	Enabled     bool   `json:"enabled"`
	ServiceName string `json:"service_name"`
	Env         string `json:"env"`
}

func (s *Server) handleTelemetryStatus(c fiber.Ctx) error {
	return c.JSON(TelemetryStatusResponse{
		Enabled:     s.cfg.Telemetry.Enabled,
		ServiceName: s.cfg.Telemetry.ServiceName,
		Env:         s.cfg.Telemetry.Env,
	})
}
