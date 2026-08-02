// Package httpx holds tiny Fiber response helpers shared across handlers.
package httpx

import "github.com/gofiber/fiber/v2"

// SendError writes a JSON error response shaped {"message": "..."}.
func SendError(c *fiber.Ctx, status int, message string) error {
	return c.Status(status).JSON(fiber.Map{"message": message})
}
