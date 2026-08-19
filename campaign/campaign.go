package campaign

import (
	"fmt"
	"time"

	"gorm.io/gorm"
)

// Config configures Initialize.
type Config struct {
	// SMTP enables sending when set; left nil, SendNow/dispatch fail with ErrSenderNotConfigured.
	SMTP *SMTPTransportConfig

	// MatchUsers resolves a UserFilter to recipients — required for PreviewAudience/SendNow to do anything.
	MatchUsers MatchUsersFunc

	// DispatchInterval is how often Dispatcher checks for due scheduled campaigns. Defaults to 1 minute.
	DispatchInterval time.Duration

	// OnDispatchError is called whenever a dispatched or admin-triggered send fails.
	OnDispatchError func(campaignID uint, err error)
}

// InitResult is what Initialize wires up.
type InitResult struct {
	Service    *Service
	Dispatcher *Dispatcher

	// SMTPEnabled reports whether Config.SMTP was actually configured.
	SMTPEnabled bool
}

// Initialize auto-migrates the campaign schema and builds a Service/Dispatcher pair.
func Initialize(db *gorm.DB, cfg Config) (*InitResult, error) {
	if err := db.AutoMigrate(&Campaign{}, &CampaignTemplate{}); err != nil {
		return nil, fmt.Errorf("campaign: failed to auto-migrate schema: %w", err)
	}

	var sender RawEmailSender
	smtpEnabled := false
	if cfg.SMTP != nil && cfg.SMTP.Host != "" && cfg.SMTP.From != "" {
		s, err := NewSMTPRawSender(*cfg.SMTP)
		if err != nil {
			return nil, fmt.Errorf("campaign: failed to initialize SMTP sender: %w", err)
		}
		sender = s
		smtpEnabled = true
	}

	svc := NewService(db, sender, cfg.MatchUsers, cfg.OnDispatchError)
	dispatcher := NewDispatcher(svc, cfg.DispatchInterval, cfg.OnDispatchError)

	return &InitResult{Service: svc, Dispatcher: dispatcher, SMTPEnabled: smtpEnabled}, nil
}
