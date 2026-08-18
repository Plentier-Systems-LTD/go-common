package billing

import (
	"fmt"

	"github.com/gofiber/fiber/v2"
	"gorm.io/gorm"
)

type Config struct {
	// Apple. Leave both empty to disable Apple purchase verification and
	// its webhook route.
	AppleSharedSecret string
	AppleRootCAPEM    []byte

	// Google. Leave GoogleServiceAccount empty to disable Google purchase
	// verification and its webhook route.
	GoogleServiceAccount           []byte
	GooglePackageName              string
	GooglePubSubAudience           string
	GooglePubSubServiceAccountMail string
}

// InitResult is what Initialize wires up.
type InitResult struct {
	Service *Service
	Handler *Handler

	// AppleEnabled / GoogleEnabled report whether that provider was
	// actually configured — MountWebhooks only mounts a provider's webhook
	// route when its provider is enabled.
	AppleEnabled  bool
	GoogleEnabled bool
}

// Initialize auto-migrates the billing schema and builds a Service/Handler
// pair; see InitResult and Config for which providers end up enabled.
// Credentials that are set but invalid are still a hard error.
func Initialize(db *gorm.DB, cfg Config) (*InitResult, error) {
	if err := db.AutoMigrate(&BillingPlan{}, &Subscription{}, &Transaction{}, &PromoCode{}, &PromoRedemption{}); err != nil {
		return nil, fmt.Errorf("billing: failed to auto-migrate schema: %w", err)
	}

	var appleProvider PaymentProvider
	appleEnabled := false
	if cfg.AppleSharedSecret != "" && len(cfg.AppleRootCAPEM) > 0 {
		p, err := NewAppleProvider(cfg.AppleSharedSecret, cfg.AppleRootCAPEM)
		if err != nil {
			return nil, fmt.Errorf("billing: failed to initialize Apple provider: %w", err)
		}
		appleProvider = p
		appleEnabled = true
	}

	var googleProvider PaymentProvider
	googleEnabled := false
	if len(cfg.GoogleServiceAccount) > 0 {
		p, err := NewGoogleProvider(
			cfg.GoogleServiceAccount,
			cfg.GooglePackageName,
			cfg.GooglePubSubAudience,
			cfg.GooglePubSubServiceAccountMail,
		)
		if err != nil {
			return nil, fmt.Errorf("billing: failed to initialize Google provider: %w", err)
		}
		googleProvider = p
		googleEnabled = true
	}

	svc := NewService(db, appleProvider, googleProvider)
	h := NewHandler(svc, appleProvider, googleProvider)

	return &InitResult{
		Service:       svc,
		Handler:       h,
		AppleEnabled:  appleEnabled,
		GoogleEnabled: googleEnabled,
	}, nil
}

// MountWebhooks registers each enabled provider's webhook route on router,
// e.g.:
//
//	result, err := billing.Initialize(db, cfg)
//	result.MountWebhooks(app.Group("/api/v1/billing/webhooks"))
func (r *InitResult) MountWebhooks(router fiber.Router) {
	if r.AppleEnabled {
		router.Post("/apple", r.Handler.AppleWebhook)
	}
	if r.GoogleEnabled {
		router.Post("/google", r.Handler.GoogleWebhook)
	}
}
