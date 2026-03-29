package auth

import (
	"context"
	"strings"

	"github.com/gofiber/fiber/v3"
)

const claimsKey = "claims"

// RequireAuth validates the token from Authorization header or auth_token cookie.
// On success it stores the Claims in c.Locals for downstream handlers.
func RequireAuth(maker *Maker) fiber.Handler {
	return func(c fiber.Ctx) error {
		tokenStr := extractToken(c)
		if tokenStr == "" {
			return fiber.NewError(fiber.StatusUnauthorized, "missing token")
		}

		claims, err := maker.VerifyToken(tokenStr)
		if err != nil {
			return fiber.NewError(fiber.StatusUnauthorized, "invalid token")
		}

		c.Locals(claimsKey, claims)
		return c.Next()
	}
}

// GetClaims retrieves the validated Claims from the request context.
// Must be called after RequireAuth middleware.
func GetClaims(c fiber.Ctx) *Claims {
	claims, _ := c.Locals(claimsKey).(*Claims)
	return claims
}

// roleRank maps role names to an integer for comparison.
var roleRank = map[string]int{
	"viewer": 1,
	"editor": 2,
	"owner":  3,
}

// RequireRole returns a middleware that enforces a minimum role level.
// It expects a getRoleFn to look up the current user's role in the workspace.
func RequireRole(maker *Maker, getRoleFn func(ctx context.Context, workspaceID, userID string) (string, error), minRole string) fiber.Handler {
	return func(c fiber.Ctx) error {
		claims := GetClaims(c)
		if claims == nil {
			return fiber.NewError(fiber.StatusUnauthorized, "missing token")
		}

		role, err := getRoleFn(c.Context(), claims.WorkspaceID, claims.UserID)
		if err != nil {
			return fiber.NewError(fiber.StatusForbidden, "access denied")
		}

		if roleRank[role] < roleRank[minRole] {
			return fiber.NewError(fiber.StatusForbidden, "insufficient permissions")
		}

		return c.Next()
	}
}

func extractToken(c fiber.Ctx) string {
	// Try Authorization: Bearer <token>
	if h := c.Get("Authorization"); strings.HasPrefix(h, "Bearer ") {
		return strings.TrimPrefix(h, "Bearer ")
	}
	// Fall back to httpOnly cookie
	return c.Cookies("auth_token")
}

// fiber:context-methods migrated
