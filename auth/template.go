package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

// EmailBranding customizes the built-in HTML verification email.
// BrandName is the only customization most projects need — the footer's
// copyright line always credits Plentier Systems regardless of
// branding, since these templates ship as part of go-common.
type EmailBranding struct {
	// BrandName is shown in the email header and, by default, isn't used
	// in the subject line (set SMTPConfig.Subject yourself if you want it
	// there too).
	BrandName string

	// LogoURL, if set, is rendered above BrandName. Must be a publicly
	// reachable image URL — most email clients block loading images
	// referenced any other way.
	LogoURL string

	// AccentColor is the CSS color used for the code box text. Defaults
	// to "#4F46E5" (indigo).
	AccentColor string
}

func (b EmailBranding) withDefaults() EmailBranding {
	if b.AccentColor == "" {
		b.AccentColor = "#4F46E5"
	}
	return b
}

type verificationEmailData struct {
	BrandName   string
	LogoURL     string
	AccentColor string
	Code        string
	Year        int
}

// verificationEmailTemplate is a table-based layout with inline styles —
// the safest combination for rendering consistently across email clients
// (Gmail, Apple Mail, Outlook), which strip <style> blocks and modern CSS.
var verificationEmailTemplate = template.Must(template.New("verification").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .BrandName}}{{.BrandName}} {{end}}verification code</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f7;padding:32px 16px;">
<tr>
<td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;background-color:#ffffff;border-radius:8px;overflow:hidden;">
<tr>
<td style="padding:32px 32px 24px 32px;text-align:center;">
{{if .LogoURL}}<img src="{{.LogoURL}}" alt="{{.BrandName}}" style="max-height:40px;margin-bottom:16px;">{{end}}
{{if .BrandName}}<div style="font-size:18px;font-weight:600;color:#111827;margin-bottom:24px;">{{.BrandName}}</div>{{end}}
<div style="font-size:20px;font-weight:600;color:#111827;margin-bottom:8px;">Verify your email</div>
<div style="font-size:14px;color:#6b7280;margin-bottom:24px;">Enter this code to confirm it's you.</div>
<div style="display:inline-block;padding:16px 24px;background-color:#f9fafb;border-radius:8px;font-size:32px;font-weight:700;letter-spacing:8px;color:{{.AccentColor}};margin-bottom:24px;">{{.Code}}</div>
<div style="font-size:13px;color:#9ca3af;">This code expires shortly. If you didn't request it, you can safely ignore this email.</div>
</td>
</tr>
<tr>
<td style="padding:20px 32px;background-color:#f9fafb;text-align:center;font-size:12px;color:#9ca3af;">
&copy; {{.Year}} Plentier Systems. All rights reserved.
</td>
</tr>
</table>
</td>
</tr>
</table>
</body>
</html>
`))

// RenderVerificationEmailHTML renders the built-in branded HTML template
// for a verification code. Use it directly, or pass NewBrandedHTMLBody's
// result as SMTPConfig.HTMLBody to wire it into SMTPSender.
func RenderVerificationEmailHTML(branding EmailBranding, code string) (string, error) {
	branding = branding.withDefaults()

	var buf bytes.Buffer
	err := verificationEmailTemplate.Execute(&buf, verificationEmailData{
		BrandName:   branding.BrandName,
		LogoURL:     branding.LogoURL,
		AccentColor: branding.AccentColor,
		Code:        code,
		Year:        time.Now().Year(),
	})
	if err != nil {
		return "", fmt.Errorf("auth: failed to render verification email template: %w", err)
	}
	return buf.String(), nil
}

// NewBrandedHTMLBody returns an SMTPConfig.HTMLBody function backed by
// the built-in branded template — the brand name is the only thing you
// supply; the copyright line is fixed.
func NewBrandedHTMLBody(branding EmailBranding) func(code string) (string, error) {
	return func(code string) (string, error) {
		return RenderVerificationEmailHTML(branding, code)
	}
}
