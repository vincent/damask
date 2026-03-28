package api

import "github.com/gofiber/fiber/v2"

func handleHealthz(c *fiber.Ctx) error {
	return c.JSON(fiber.Map{"status": "ok"})
}
