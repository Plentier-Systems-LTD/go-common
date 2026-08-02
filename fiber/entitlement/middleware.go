// Package entitlement provides Fiber middleware on top of
// go-common/entitlement and go-common/billing for gating routes behind a
// subscription or a lifetime free-tier allowance.
package entitlement

import (
	"github.com/gofiber/fiber/v2"

	sharedbilling "github.com/Plentier-Systems-LTD/go-common/billing"
	sharedentitlement "github.com/Plentier-Systems-LTD/go-common/entitlement"
	fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
)

// RequireEntitlement gates a route behind "has an active subscription OR
// hasn't used up their lifetime free allowance for kind". Must run after
// fiberauth.RequireAuth (reads the caller's claims via fiberauth.User).
//
// billingSvc may be nil when billing isn't configured (e.g. local dev
// without provider credentials): subscribed is then always treated as
// false, so the free-tier limit alone governs access. onLimitReached
// builds the 402 response body, letting each project control its own
// copy/error code per Kind.
func RequireEntitlement(billingSvc *sharedbilling.Service, limits sharedentitlement.Limits, kind sharedentitlement.Kind, counter sharedentitlement.Counter, onLimitReached func(kind sharedentitlement.Kind, limit int) fiber.Map) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, _ := fiberauth.User(c)

		if billingSvc != nil {
			subscribed, err := billingSvc.IsSubscribed(c.UserContext(), claims.UserID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to verify subscription status"})
			}
			if subscribed {
				return c.Next()
			}
		}

		limit := limits[kind]
		used, err := counter.Count(c.UserContext(), kind, claims.UserID)
		if err != nil {
			return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to verify usage"})
		}

		if int(used) >= limit {
			return c.Status(fiber.StatusPaymentRequired).JSON(onLimitReached(kind, limit))
		}

		return c.Next()
	}
}

// RequireSubscription gates a route behind "has an active subscription" —
// no free-tier allowance, unlike RequireEntitlement. Must run after
// fiberauth.RequireAuth. message is the JSON body sent on a 402 response.
func RequireSubscription(billingSvc *sharedbilling.Service, message fiber.Map) fiber.Handler {
	return func(c *fiber.Ctx) error {
		claims, _ := fiberauth.User(c)

		var subscribed bool
		if billingSvc != nil {
			var err error
			subscribed, err = billingSvc.IsSubscribed(c.UserContext(), claims.UserID)
			if err != nil {
				return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"message": "failed to verify subscription status"})
			}
		}
		if !subscribed {
			return c.Status(fiber.StatusPaymentRequired).JSON(message)
		}

		return c.Next()
	}
}
