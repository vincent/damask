package api

import (
	"strings"

	"github.com/gofiber/fiber/v3"
)

// ErrorResponse is the standard error envelope returned on non-2xx responses.
type ErrorResponse struct {
	Error string `json:"error"`
}

// ValidationErrorResponse is returned with HTTP 422 when request validation fails.
type ValidationErrorResponse struct {
	Error  string            `json:"error"`
	Fields map[string]string `json:"fields"`
}

func errRes(c fiber.Ctx, status int, msg string) error {
	return c.Status(status).JSON(fiber.Map{"error": msg})
}

func isUniqueConstraintError(err error) bool {
	return strings.Contains(err.Error(), "UNIQUE constraint failed")
}
