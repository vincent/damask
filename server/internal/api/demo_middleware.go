package api

import (
	"damask/server/internal/auth"

	"github.com/gofiber/fiber/v3"
)

// demoRestrictedResponse is the standard 403 body for blocked demo actions.
var demoRestrictedResponse = fiber.Map{
	"error":      "not_available_in_demo",
	"message":    "This action is not available in the demo. Sign up for a free account to unlock it.",
	"signup_url": "https://damask.io/signup",
}

// demoBlock is an inline middleware that returns 403 when the request carries a
// demo token. Attach it on individual routes that are blocked in demo mode.
// It must appear after auth.RequireAuth in the middleware chain.
func demoBlock(c fiber.Ctx) error {
	claims := auth.GetClaims(c)
	if claims != nil && claims.IsDemo {
		return c.Status(fiber.StatusForbidden).JSON(demoRestrictedResponse)
	}
	return c.Next()
}
