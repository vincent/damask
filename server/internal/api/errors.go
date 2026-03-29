package api

import "github.com/gofiber/fiber/v3"

func errRes(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}
