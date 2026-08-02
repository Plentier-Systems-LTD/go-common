// Package billing provides Fiber middleware on top of go-common/billing.
package billing

import (
	"github.com/gofiber/fiber/v2"

	sharedbilling "github.com/Plentier-Systems-LTD/go-common/billing"
	fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
	"github.com/Plentier-Systems-LTD/go-common/fiber/httpx"
)

// PremiumProtected gates a route behind "has an active, unexpired
// subscription" (Service.IsSubscribed). Must run after
// fiberauth.RequireAuth so claims are available via fiberauth.User.
//
// For free-tier/lifetime-allowance gating (subscribed OR hasn't used up a
// free quota), use go-common/fiber/entitlement instead — this middleware
// only covers the simpler "subscription required, no free tier" case.
func PremiumProtected(svc *sharedbilling.Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := fiberauth.User(c)
		if !ok {
			return httpx.SendError(c, fiber.StatusUnauthorized, "Authentication required")
		}

		isSubscribed, err := svc.IsSubscribed(c.UserContext(), claims.UserID)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "Failed to verify your subscription standing")
		}

		if !isSubscribed {
			return httpx.SendError(c, fiber.StatusForbidden, "Premium subscription required to access this resource")
		}

		return c.Next()
	}
}
