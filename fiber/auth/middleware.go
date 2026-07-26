// Package auth provides Fiber middleware on top of go-common/auth's
// framework-agnostic JWT verification.
package auth

import (
	"strings"

	sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
	"github.com/gofiber/fiber/v2"
)

const localsKey = "go-common:auth:claims"

// RequireAuth rejects requests without a valid Bearer access token, and
// otherwise stores its claims for User to retrieve.
func RequireAuth(cfg sharedauth.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, err := claimsFromRequest(c, cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		c.Locals(localsKey, claims)
		return c.Next()
	}
}

// OptionalAuth attaches claims when a valid Bearer token is present, but
// lets the request through either way if no token was sent. A malformed
// or expired token is still rejected. Use User(c) to check whether the
// caller ended up authenticated.
func OptionalAuth(cfg sharedauth.Config) fiber.Handler {
	return func(c *fiber.Ctx) error {
		if c.Get("Authorization") == "" {
			return c.Next()
		}
		claims, err := claimsFromRequest(c, cfg)
		if err != nil {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": "unauthorized"})
		}
		c.Locals(localsKey, claims)
		return c.Next()
	}
}

func claimsFromRequest(c *fiber.Ctx, cfg sharedauth.Config) (*sharedauth.Claims, error) {
	token := strings.TrimPrefix(c.Get("Authorization"), "Bearer ")
	if token == "" {
		return nil, sharedauth.ErrInvalidToken
	}
	return sharedauth.VerifyToken(cfg, token)
}

// User returns the authenticated caller's claims. ok is false when
// RequireAuth/OptionalAuth wasn't applied to this route, or OptionalAuth
// let an unauthenticated request through.
func User(c *fiber.Ctx) (*sharedauth.Claims, bool) {
	claims, ok := c.Locals(localsKey).(*sharedauth.Claims)
	return claims, ok
}
