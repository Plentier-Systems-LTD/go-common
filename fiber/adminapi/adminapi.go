// Package adminapi mounts go-common/adminapi's read-only contract as Fiber
// routes: "import package, register route" is the entire integration
// surface a platform needs to expose itself to the stats dashboard.
package adminapi

import (
	"crypto/subtle"

	sharedadminapi "github.com/Plentier-Systems-LTD/go-common/adminapi"
	"github.com/Plentier-Systems-LTD/go-common/fiber/httpx"
	"github.com/gofiber/fiber/v2"
)

const apiKeyHeader = "X-Internal-Api-Key"

// RequireAPIKey rejects requests whose X-Internal-Api-Key header doesn't
// match key, using a constant-time comparison. This is service-to-service
// auth for the dashboard calling into a platform — deliberately a shared
// secret rather than a user JWT, since the caller isn't a user session.
func RequireAPIKey(key string) fiber.Handler {
	return func(c *fiber.Ctx) error {
		got := c.Get(apiKeyHeader)
		if got == "" || subtle.ConstantTimeCompare([]byte(got), []byte(key)) != 1 {
			return httpx.SendError(c, fiber.StatusUnauthorized, "unauthorized")
		}
		return c.Next()
	}
}

// Mount registers GET /internal/stats, GET /internal/users, and
// GET /internal/stats/trend on router, all behind RequireAPIKey(apiKey).
// If provider also satisfies sharedadminapi.MutableProvider, it additionally
// registers PATCH /internal/users/:id and DELETE /internal/users/:id —
// omitted entirely for a read-only Provider, so those routes simply 404
// rather than existing but always failing.
func Mount(router fiber.Router, provider sharedadminapi.Provider, apiKey string) {
	group := router.Group("/internal", RequireAPIKey(apiKey))

	group.Get("/stats", func(c *fiber.Ctx) error {
		stats, err := provider.PlatformStats(c.UserContext())
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to load stats")
		}
		return c.JSON(stats)
	})

	group.Get("/users", func(c *fiber.Ctx) error {
		page := c.QueryInt("page", 1)
		limit := c.QueryInt("limit", 50)
		search := c.Query("search")

		users, err := provider.ListUsers(c.UserContext(), page, limit, search)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to load users")
		}
		return c.JSON(users)
	})

	group.Get("/stats/trend", func(c *fiber.Ctx) error {
		days := c.QueryInt("days", 30)
		if days < 1 || days > 90 {
			days = 30
		}

		trend, err := provider.SignupTrend(c.UserContext(), days)
		if err != nil {
			return httpx.SendError(c, fiber.StatusInternalServerError, "failed to load trend")
		}
		return c.JSON(trend)
	})

	if mutable, ok := provider.(sharedadminapi.MutableProvider); ok {
		group.Patch("/users/:id", func(c *fiber.Ctx) error {
			var patch sharedadminapi.UserPatch
			if err := c.BodyParser(&patch); err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, "invalid request body")
			}

			user, err := mutable.UpdateUser(c.UserContext(), c.Params("id"), patch)
			if err != nil {
				return httpx.SendError(c, fiber.StatusInternalServerError, "failed to update user")
			}
			return c.JSON(user)
		})

		group.Delete("/users/:id", func(c *fiber.Ctx) error {
			if err := mutable.DeleteUser(c.UserContext(), c.Params("id")); err != nil {
				return httpx.SendError(c, fiber.StatusInternalServerError, "failed to delete user")
			}
			return c.SendStatus(fiber.StatusNoContent)
		})
	}

	if promo, ok := provider.(sharedadminapi.PromoProvider); ok {
		group.Get("/promo-codes", func(c *fiber.Ctx) error {
			codes, err := promo.ListPromoCodes(c.UserContext())
			if err != nil {
				return httpx.SendError(c, fiber.StatusInternalServerError, "failed to load promo codes")
			}
			return c.JSON(codes)
		})

		group.Post("/promo-codes", func(c *fiber.Ctx) error {
			var req sharedadminapi.CreatePromoCodeRequest
			if err := c.BodyParser(&req); err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, "invalid request body")
			}

			code, err := promo.CreatePromoCode(c.UserContext(), req)
			if err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, err.Error())
			}
			return c.Status(fiber.StatusCreated).JSON(code)
		})

		group.Patch("/promo-codes/:code", func(c *fiber.Ctx) error {
			var req struct {
				Active bool `json:"active"`
			}
			if err := c.BodyParser(&req); err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, "invalid request body")
			}

			code, err := promo.SetPromoCodeActive(c.UserContext(), c.Params("code"), req.Active)
			if err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, err.Error())
			}
			return c.JSON(code)
		})

		group.Delete("/promo-codes/:code", func(c *fiber.Ctx) error {
			if err := promo.DeletePromoCode(c.UserContext(), c.Params("code")); err != nil {
				return httpx.SendError(c, fiber.StatusBadRequest, err.Error())
			}
			return c.SendStatus(fiber.StatusNoContent)
		})
	}
}
