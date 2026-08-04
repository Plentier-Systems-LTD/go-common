// Package ratelimit provides a shared rate-limiting middleware setup for
// Plentier's Fiber services, on top of gofiber's fixed-window limiter.
package ratelimit

import (
	"time"

	"github.com/gofiber/fiber/v2"
	fiberlimiter "github.com/gofiber/fiber/v2/middleware/limiter"

	"github.com/Plentier-Systems-LTD/go-common/fiber/httpx"
)

// Config controls how many requests a caller may make in a given window.
type Config struct {
	Max int

	// Expiration is the sliding window size. Default: 1 minute.
	Expiration time.Duration

	// KeyGenerator groups requests for the Max/Expiration count. Default:
	// the caller's IP (c.IP()).
	KeyGenerator func(c *fiber.Ctx) string
}

func New(cfg Config) fiber.Handler {
	return fiberlimiter.New(fiberlimiter.Config{
		Max:          cfg.Max,
		Expiration:   cfg.Expiration,
		KeyGenerator: cfg.KeyGenerator,
		LimitReached: func(c *fiber.Ctx) error {
			return httpx.SendError(c, fiber.StatusTooManyRequests, "too many requests")
		},
	})
}
