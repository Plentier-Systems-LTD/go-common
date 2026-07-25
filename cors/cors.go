// Package cors provides a shared CORS middleware setup for Plentier's Fiber
// services.
package cors

import (
	"github.com/gofiber/fiber/v2"
	fibercors "github.com/gofiber/fiber/v2/middleware/cors"
)

// Config controls which origins may call the API from a browser. Native
// mobile clients are unaffected by CORS.
type Config struct {
	// AllowOrigins is a comma-separated allow-list, e.g.
	// "https://app.example.com,http://localhost:8081". Use "*" to allow any
	// origin (fine for local development, not recommended in production).
	AllowOrigins string

	// AllowCredentials lets cookies/Authorization headers be sent
	// cross-origin. Must not be combined with AllowOrigins "*".
	AllowCredentials bool
}

// New builds the CORS middleware from Config.
func New(cfg Config) fiber.Handler {
	return fibercors.New(fibercors.Config{
		AllowOrigins:     cfg.AllowOrigins,
		AllowCredentials: cfg.AllowCredentials,
	})
}
