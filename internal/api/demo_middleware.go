//go:build demo

package api

import (
	"github.com/gofiber/fiber/v3"
)

// demoRestrictedResponse is the standard 403 body for blocked demo actions.
var demoRestrictedResponse = fiber.Map{
	"error":      "not_available_in_demo",
	"message":    "This action is not available in the demo. Sign up for a free account to unlock it.",
	"signup_url": "/signup",
}
