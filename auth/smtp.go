package auth

import (
	"context"
	"errors"
	"fmt"
	"net/smtp"
)

// SMTPConfig configures SMTPSender.
type SMTPConfig struct {
	// Host and Port are the SMTP server address, e.g. "smtp.gmail.com", 587.
	Host string
	Port int

	Username string
	Password string

	// From is the sender address, e.g. "MyApp <noreply@myapp.com>". Required.
	From string

	// Subject is the email subject line. Defaults to "Your verification code".
	Subject string

	// Body renders the plain-text email body for a given code. Defaults
	// to a minimal message containing just the code.
	Body func(code string) string
}

func (c SMTPConfig) withDefaults() SMTPConfig {
	if c.Subject == "" {
		c.Subject = "Your verification code"
	}
	if c.Body == nil {
		c.Body = func(code string) string {
			return fmt.Sprintf(
				"Your verification code is: %s\r\n\r\nThis code expires shortly. If you didn't request this, you can ignore this email.",
				code,
			)
		}
	}
	return c
}

// SMTPSender sends verification emails over SMTP with AUTH PLAIN — works
// against Gmail, Amazon SES SMTP, Mailgun SMTP, and similar. It's the
// zero-dependency default; implement EmailSender directly to use a
// provider's HTTP API instead.
type SMTPSender struct {
	cfg SMTPConfig
}

var _ EmailSender = (*SMTPSender)(nil)

// NewSMTPSender builds an SMTPSender. cfg.Host and cfg.From are required.
func NewSMTPSender(cfg SMTPConfig) (*SMTPSender, error) {
	if cfg.Host == "" {
		return nil, errors.New("auth: SMTPConfig.Host is required")
	}
	if cfg.From == "" {
		return nil, errors.New("auth: SMTPConfig.From is required")
	}
	return &SMTPSender{cfg: cfg.withDefaults()}, nil
}

// SendVerificationCode sends the code to to over SMTP. The context is
// accepted to satisfy EmailSender but isn't used for cancellation —
// net/smtp has no context support.
func (s *SMTPSender) SendVerificationCode(_ context.Context, to string, code string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	msg := fmt.Sprintf(
		"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
		s.cfg.From, to, s.cfg.Subject, s.cfg.Body(code),
	)

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, []byte(msg)); err != nil {
		return fmt.Errorf("auth: failed to send verification email: %w", err)
	}
	return nil
}
