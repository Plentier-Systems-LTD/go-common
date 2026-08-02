package billing

import (
	"github.com/gofiber/fiber/v2"

	fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
)

// PremiumProtected gates a route behind "has an active, unexpired
// subscription" (Service.IsSubscribed). Must run after
// fiberauth.RequireAuth so claims are available via fiberauth.User.
//
// For free-tier/lifetime-allowance gating (subscribed OR hasn't used up a
// free quota), use go-common/fiber/entitlement instead — this middleware
// only covers the simpler "subscription required, no free tier" case.
func PremiumProtected(svc *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, ok := fiberauth.User(c)
		if !ok {
			return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
				"message": "Authentication required",
			})
		}

		isSubscribed, err := svc.IsSubscribed(c.UserContext(), claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
				"message": "Failed to verify your subscription standing",
			})
		}

		if !isSubscribed {
			return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
				"message": "Premium subscription required to access this resource",
			})
		}

		return c.Next()
	}
}
