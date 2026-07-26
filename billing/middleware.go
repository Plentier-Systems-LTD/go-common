package billing

import (
	// "github.com/Plentier-Systems-LTD/therapist-api/server"
	"github.com/gofiber/fiber/v2"
	// "github.com/ochom/gutils/logs"
)

func PremiumProtected(svc *Service) fiber.Handler {
	return func(c *fiber.Ctx) error {
		// @TODO: Implement subscription verification logic here
		// userID := server.GetCtxUser(c)
		// if userID.ID == "" {
		// 	return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{
		// 		"message": "Authentication required",
		// 	})
		// }

		// isSubscribed, err := svc.IsSubscribed(c.Context(), userID.ID)
		// if err != nil {
		// 	logs.Debug("[Billing Error] failed to verify subscription status: %v", err)
		// 	return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{
		// 		"message": "Failed to verify your subscription standing",
		// 	})
		// }

		// if !isSubscribed {
		// 	return c.Status(fiber.StatusForbidden).JSON(fiber.Map{
		// 		"message": "Premium subscription required to access this resource",
		// 	})
		// }

		return c.Next()
	}
}
