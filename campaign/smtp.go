package campaign

import (
	"bytes"
	"context"
	"errors"
	"fmt"
	"net/smtp"
	"strings"
)

// stripCRLF removes header-injection characters before a value is interpolated into a raw SMTP message.
var stripCRLF = strings.NewReplacer("\r", "", "\n", "").Replace

// RawEmailSender delivers an arbitrary subject/body email — kept separate from auth's verification-code sender.
type RawEmailSender interface {
	SendEmail(ctx context.Context, to, subject, htmlBody, textBody string) error
}

// SMTPTransportConfig is the bare SMTP connection info RawEmailSender needs.
type SMTPTransportConfig struct {
	Host, Username, Password, From string
	Port                           int
}

// SMTPRawSender sends campaign emails over SMTP with AUTH PLAIN.
type SMTPRawSender struct {
	cfg SMTPTransportConfig
}

var _ RawEmailSender = (*SMTPRawSender)(nil)

// NewSMTPRawSender builds an SMTPRawSender. cfg.Host and cfg.From are required.
func NewSMTPRawSender(cfg SMTPTransportConfig) (*SMTPRawSender, error) {
	if cfg.Host == "" {
		return nil, errors.New("campaign: SMTPTransportConfig.Host is required")
	}
	if cfg.From == "" {
		return nil, errors.New("campaign: SMTPTransportConfig.From is required")
	}
	return &SMTPRawSender{cfg: cfg}, nil
}

// SendEmail sends to over SMTP; htmlBody, if non-empty, is sent alongside textBody as multipart/alternative.
func (s *SMTPRawSender) SendEmail(_ context.Context, to, subject, htmlBody, textBody string) error {
	addr := fmt.Sprintf("%s:%d", s.cfg.Host, s.cfg.Port)

	var auth smtp.Auth
	if s.cfg.Username != "" {
		auth = smtp.PlainAuth("", s.cfg.Username, s.cfg.Password, s.cfg.Host)
	}

	msg := s.buildMessage(to, subject, htmlBody, textBody)

	if err := smtp.SendMail(addr, auth, s.cfg.From, []string{to}, msg); err != nil {
		return fmt.Errorf("campaign: failed to send email: %w", err)
	}
	return nil
}

func (s *SMTPRawSender) buildMessage(to, subject, htmlBody, textBody string) []byte {
	from, to, subject := stripCRLF(s.cfg.From), stripCRLF(to), stripCRLF(subject)

	if htmlBody == "" {
		return []byte(fmt.Sprintf(
			"From: %s\r\nTo: %s\r\nSubject: %s\r\nContent-Type: text/plain; charset=UTF-8\r\n\r\n%s",
			from, to, subject, textBody,
		))
	}

	const boundary = "go-common-campaign-boundary"
	var buf bytes.Buffer
	fmt.Fprintf(&buf, "From: %s\r\n", from)
	fmt.Fprintf(&buf, "To: %s\r\n", to)
	fmt.Fprintf(&buf, "Subject: %s\r\n", subject)
	fmt.Fprintf(&buf, "MIME-Version: 1.0\r\n")
	fmt.Fprintf(&buf, "Content-Type: multipart/alternative; boundary=%s\r\n\r\n", boundary)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/plain; charset=UTF-8\r\n\r\n%s\r\n\r\n", textBody)

	fmt.Fprintf(&buf, "--%s\r\n", boundary)
	fmt.Fprintf(&buf, "Content-Type: text/html; charset=UTF-8\r\n\r\n%s\r\n\r\n", htmlBody)

	fmt.Fprintf(&buf, "--%s--\r\n", boundary)

	return buf.Bytes()
}
