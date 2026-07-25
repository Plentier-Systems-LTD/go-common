// Package auth provides a Fiber middleware around the framework-agnostic
// github.com/Plentier-Systems-LTD/go-common/auth package. Only import this
// package from Fiber services; non-Fiber consumers depend on the parent
// auth package alone.
package auth

import (
	"strings"

	"github.com/gofiber/fiber/v2"

	sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
)

const ctxUserKey = "go_common_auth_user"

// RequireAuth rejects requests without a valid Bearer token and stores the
// verified claims in the request context for handlers to read via User.
func RequireAuth(cfg sharedauth.Config) fiber.Handler {
	return middleware(cfg, true)
}

// OptionalAuth verifies the Bearer token if one is present, but lets the
// request through either way. Handlers should use TryUser to check.
func OptionalAuth(cfg sharedauth.Config) fiber.Handler {
	return middleware(cfg, false)
}

// middleware is the single implementation behind RequireAuth/OptionalAuth —
// they differ only in what happens when no token is present.
func middleware(cfg sharedauth.Config, required bool) fiber.Handler {
	return func(c *fiber.Ctx) error {
		token := bearerToken(c)
		if token == "" {
			if required {
				return unauthorized(c, "no token provided")
			}
			return c.Next()
		}

		claims, err := sharedauth.VerifyToken(cfg, token)
		if err != nil {
			return unauthorized(c, "invalid token")
		}

		c.Locals(ctxUserKey, claims)
		return c.Next()
	}
}

// User returns the authenticated claims set by RequireAuth/OptionalAuth. It
// panics if called on a route not behind one of those middlewares.
func User(c *fiber.Ctx) *sharedauth.Claims {
	claims, ok := TryUser(c)
	if !ok {
		panic("auth: no authenticated user in context")
	}
	return claims
}

// TryUser returns the authenticated claims, if any, without panicking.
func TryUser(c *fiber.Ctx) (*sharedauth.Claims, bool) {
	claims, ok := c.Locals(ctxUserKey).(*sharedauth.Claims)
	return claims, ok
}

func bearerToken(c *fiber.Ctx) string {
	header := c.Get("Authorization")
	if header == "" {
		return ""
	}
	return strings.TrimPrefix(header, "Bearer ")
}

func unauthorized(c *fiber.Ctx, reason string) error {
	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"message": "unauthorized: " + reason})
}
