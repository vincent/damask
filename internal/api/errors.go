package api

import (
	"errors"

	"damask/server/internal/apperr"

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

func isInvalidInput(err error) bool {
	return errors.Is(err, apperr.ErrInvalidInput)
}

// ErrorStatusResponse maps a service-layer error to the appropriate HTTP response.
// ErrNotFound -> 404, ErrForbidden -> 403, ErrConflict -> 409,
// ErrInvalidInput -> 422, anything else -> 500.
func ErrorStatusResponse(c fiber.Ctx, err error) error {
	switch {
	case errors.Is(err, apperr.ErrNotFound):
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, apperr.ErrForbidden):
		return c.Status(fiber.StatusForbidden).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, apperr.ErrConflict):
		return c.Status(fiber.StatusConflict).JSON(fiber.Map{"error": err.Error()})
	case errors.Is(err, apperr.ErrInvalidInput):
		return c.Status(fiber.StatusUnprocessableEntity).JSON(fiber.Map{"error": err.Error()})
	default:
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "internal error"})
	}
}
