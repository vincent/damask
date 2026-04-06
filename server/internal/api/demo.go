package api

import (
	"database/sql"
	"time"

	"github.com/gofiber/fiber/v3"
)

// handleDemoSession issues a passwordless JWT for the demo workspace.
// Only registered when DEMO_MODE=true.
//
// POST /demo/session
func (s *Server) handleDemoSession(c fiber.Ctx) error {
	if s.demo == nil {
		return fiber.NewError(fiber.StatusNotFound, "demo mode not enabled")
	}

	// If a reset is in progress, tell the client to retry
	if s.demo.IsResetting() {
		c.Set("Retry-After", "10")
		return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
			"error":   "demo_resetting",
			"message": "The demo workspace is resetting. Please try again in a few seconds.",
		})
	}

	userID, workspaceID, err := s.demo.GetDemoUser(c.Context())
	if err != nil {
		if err == sql.ErrNoRows {
			// Workspace doesn't exist — mid-reset
			c.Set("Retry-After", "10")
			return c.Status(fiber.StatusServiceUnavailable).JSON(fiber.Map{
				"error":   "demo_resetting",
				"message": "The demo workspace is resetting. Please try again in a few seconds.",
			})
		}
		return fiber.NewError(fiber.StatusInternalServerError, "demo session unavailable")
	}

	// 2-hour demo token with is_demo=true claim
	tokenStr, err := s.tokenMaker.CreateDemoToken(userID, workspaceID, 2*time.Hour)
	if err != nil {
		return fiber.NewError(fiber.StatusInternalServerError, "could not create demo token")
	}

	// Set httpOnly cookie (same shape as regular login)
	secure := s.cfg.AppEnv != "development"
	c.Cookie(&fiber.Cookie{
		Name:     "auth_token",
		Value:    tokenStr,
		HTTPOnly: true,
		SameSite: "Lax",
		Path:     "/",
		Secure:   secure,
		MaxAge:   2 * 60 * 60, // 2 hours
	})

	return c.Status(fiber.StatusOK).JSON(fiber.Map{
		"token":        tokenStr,
		"workspace_id": workspaceID,
		"user_id":      userID,
		"is_demo":      true,
	})
}
