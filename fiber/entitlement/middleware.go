// Package entitlement provides Fiber middleware on top of
// go-common/entitlement and go-common/billing for gating routes behind a
// subscription or a lifetime free-tier allowance.
package entitlement

import (
	"context"

	"github.com/gofiber/fiber/v2"

	sharedbilling "github.com/Plentier-Systems-LTD/go-common/billing"
	sharedentitlement "github.com/Plentier-Systems-LTD/go-common/entitlement"
	fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
	"github.com/Plentier-Systems-LTD/go-common/fiber/httpx"
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

		subscribed, err := isSubscribed(c.UserContext(), billingSvc, claims.UserID)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to verify subscription status")
		}
		if subscribed {
			return c.Next()
		}

		limit := limits[kind]
		used, err := counter.Count(c.UserContext(), kind, claims.UserID)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to verify usage")
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

		subscribed, err := isSubscribed(c.UserContext(), billingSvc, claims.UserID)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to verify subscription status")
		}
		if !subscribed {
			return c.Status(fiber.StatusPaymentRequired).JSON(message)
		}

		return c.Next()
	}
}

// isSubscribed treats a nil billingSvc as "never subscribed" so callers
// don't each need their own nil guard around IsSubscribed.
func isSubscribed(ctx context.Context, billingSvc *sharedbilling.Service, userID string) (bool, error) {
	if billingSvc == nil {
		return false, nil
	}
	return billingSvc.IsSubscribed(ctx, userID)
}
