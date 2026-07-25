package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"
)

const ctxUserKey = "plentier_auth_user"

// RequireAuth rejects requests without a valid Bearer token and stores the
// verified claims in the request context for handlers to read via User.
func RequireAuth(cfg Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := bearerToken(c)
		if token == "" {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized: no token provided"})
		}

		claims, err := VerifyToken(cfg, token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized: invalid token"})
		}

		c.Locals(ctxUserKey, claims)
		return c.Next()
	}
}

// OptionalAuth verifies the Bearer token if one is present, but lets the
// request through either way. Handlers should use TryUser to check.
func OptionalAuth(cfg Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := bearerToken(c)
		if token == "" {
			return c.Next()
		}

		claims, err := VerifyToken(cfg, token)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized: invalid token"})
		}

		c.Locals(ctxUserKey, claims)
		return c.Next()
	}
}

// User returns the authenticated claims set by RequireAuth/OptionalAuth. It
// panics if called on a route not behind one of those middlewares.
func User(c *fiber.Ctx) *Claims {
	claims, ok := TryUser(c)
	if !ok {
		panic("auth: no authenticated user in context")
	}
	return claims
}

// TryUser returns the authenticated claims, if any, without panicking.
func TryUser(c *fiber.Ctx) (*Claims, bool) {
	claims, ok := c.Locals(ctxUserKey).(*Claims)
	return claims, ok
}

func bearerToken(c *fiber.Ctx) string {
	header := c.Get("Authorization")
	if header == "" {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}
