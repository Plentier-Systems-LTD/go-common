package billing

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"github.com/ochom/gutils/logs"
	"gorm.io/gorm"
)

type Config struct {
	// Apple
	AppleSharedSecret string
	AppleRootCAPEM    []byte

	// Google
	GoogleServiceAccount           []byte
	GooglePackageName              string
	GooglePubSubAudience           string
	GooglePubSubServiceAccountMail string
}

func Initialize(app *fiber.App, db *gorm.DB, cfg Config) (*Service, error) {
	if err := db.AutoMigrate(&BillingPlan{}, &Subscription{}, &Transaction{}); err != nil {
		return nil, fmt.Errorf("billing: failed to auto-migrate schema: %w", err)
	}

	appleProvider, err := NewAppleProvider(cfg.AppleSharedSecret, cfg.AppleRootCAPEM)
	if err != nil {
		return nil, fmt.Errorf("billing: failed to initialize Apple provider: %w", err)
	}

	googleProvider, err := NewGoogleProvider(
		cfg.GoogleServiceAccount,
		cfg.GooglePackageName,
		cfg.GooglePubSubAudience,
		cfg.GooglePubSubServiceAccountMail,
	)
	if err != nil {
		// @TODO: Uncomment this line when Google provider is implemented
		logs.Warn("Google provider initialization skipped: %v", err)
		// return nil, fmt.Errorf("billing: failed to initialize Google provider: %w", err)
	}

	svc := NewService(db, appleProvider, googleProvider)
	h := NewHandler(svc, appleProvider, googleProvider)

	api := app.Group("/api/v1/billing")
	api.Post("/webhooks/apple", h.AppleWebhook)
	api.Post("/webhooks/google", h.GoogleWebhook)

	return svc, nil
}
