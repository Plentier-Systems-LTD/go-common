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

// Mount registers GET /internal/stats and GET /internal/users on router,
// both behind RequireAPIKey(apiKey).
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
}
