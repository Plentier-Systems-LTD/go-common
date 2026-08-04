package billing

import (
	"github.com/gofiber/fiber/v2"
	"github.com/ochom/gutils/logs"
)

type Handler struct {
	service *Service
	apple   PaymentProvider
	google  PaymentProvider
}

func NewHandler(s *Service, apple PaymentProvider, google PaymentProvider) *Handler {
	return &Handler{service: s, apple: apple, google: google}
}

// WebhookErrorResponse is returned when a provider webhook payload is
// rejected or fails to process.
type WebhookErrorResponse struct {
	Error string `json:"error"`
}

// AppleWebhook receives App Store Server Notifications v2.
//
//	@Summary		Apple webhook
//	@Description	Receives signed App Store Server Notifications v2 (subscription renewals, cancellations, refunds). Authenticated via the notification's signed JWS payload, not a bearer token.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object	true	"Signed Apple notification envelope ({\"signedPayload\": \"<JWS>\"})"
//	@Success		200		"Notification processed"
//	@Failure		400		{object}	WebhookErrorResponse	"Invalid or unverifiable payload"
//	@Failure		500		{object}	WebhookErrorResponse	"Failed to process notification"
//	@Router			/billing/webhooks/apple [post]
func (h *Handler) AppleWebhook(c *fiber.Ctx) error {
	if h.apple == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "apple billing is not configured"})
	}

	payload := c.Body()
	ctx := c.UserContext()

	res, eventType, err := h.apple.ParseWebhook(ctx, payload)
	if err != nil {
		logs.Debug("[Billing Warning] Apple webhook rejected: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid data"})
	}

	userID := res.AccountToken

	if userID == "" {
		userID, err = h.service.FindUserIDByOriginalTransactionID(ctx, res.OriginalTransactionID)
		if err != nil {
			logs.Debug("[Billing Warning] Apple transaction unmapped to user: %v", err)
			return c.SendStatus(fiber.StatusOK)
		}
	}

	if err := h.service.ProcessPurchaseResult(ctx, userID, "apple", res, eventType); err != nil {
		logs.Debug("[Billing Error] failed to process Apple purchase result: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to process"})
	}

	return c.SendStatus(fiber.StatusOK)
}

// GoogleWebhook receives Real-time Developer Notifications via Pub/Sub push.
//
//	@Summary		Google webhook
//	@Description	Receives Google Play Real-time Developer Notifications delivered as a Pub/Sub push message. Authenticated via the push subscription's OIDC bearer token (service-to-service), not a user bearer token.
//	@Tags			Billing
//	@Accept			json
//	@Produce		json
//	@Param			request	body		object	true	"Pub/Sub push envelope ({\"message\": {\"data\": \"<base64 developer notification>\"}})"
//	@Success		200		"Notification processed"
//	@Failure		400		{object}	WebhookErrorResponse	"Invalid payload or failed push authentication"
//	@Failure		500		{object}	WebhookErrorResponse	"Failed to process notification"
//	@Router			/billing/webhooks/google [post]
func (h *Handler) GoogleWebhook(c *fiber.Ctx) error {
	if h.google == nil {
		return c.Status(fiber.StatusNotFound).JSON(fiber.Map{"error": "google billing is not configured"})
	}

	payload := c.Body()

	ctx := WithGoogleAuthHeader(c.UserContext(), c.Get("Authorization"))

	res, eventType, err := h.google.ParseWebhook(ctx, payload)
	if err != nil {
		logs.Debug("[Billing Warning] Google webhook rejected: %v", err)
		return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid data"})
	}

	userID := res.AccountToken

	if userID == "" {
		userID, err = h.service.FindUserIDByOriginalTransactionID(ctx, res.OriginalTransactionID)
		if err != nil {
			logs.Debug("[Billing Warning] Google transaction unmapped to user: %v", err)
			return c.SendStatus(fiber.StatusOK)
		}
	}

	if err := h.service.ProcessPurchaseResult(ctx, userID, "google", res, eventType); err != nil {
		logs.Debug("[Billing Error] failed to process Google purchase result: %v", err)
		return c.Status(fiber.StatusInternalServerError).JSON(fiber.Map{"error": "failed to process"})
	}

	return c.SendStatus(fiber.StatusOK)
}
