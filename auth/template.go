package auth

import (
	"bytes"
	"fmt"
	"html/template"
	"time"
)

// EmailBranding customizes the built-in HTML code email template.
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

// CodeEmailContent customizes the copy shown around the code box, letting
// the same branded layout serve any code-driven flow — email
// verification, a caregiver invite code, etc. — without duplicating the
// template. Zero value renders byte-for-byte as the original
// verification-code email (including the <title> tag, which historically
// read "verification code" rather than the on-page heading — kept as its
// own field below rather than reusing Title, specifically so existing
// verification-only consumers see no output change at all).
type CodeEmailContent struct {
	// TitleTag is the <head><title> text (after "{{BrandName}} "), shown
	// in a browser tab if the HTML is opened directly — most email clients
	// ignore it. Defaults to "verification code".
	TitleTag string
	// Title is the large heading above the code box.
	Title string
	// Subtext is the smaller line below Title, above the code box.
	Subtext string
	// FooterNote replaces the default expiry/ignore note below the code
	// box, if set.
	FooterNote string
}

func (c CodeEmailContent) withDefaults() CodeEmailContent {
	if c.TitleTag == "" {
		c.TitleTag = "verification code"
	}
	if c.Title == "" {
		c.Title = "Verify your email"
	}
	if c.Subtext == "" {
		c.Subtext = "Enter this code to confirm it's you."
	}
	if c.FooterNote == "" {
		c.FooterNote = "This code expires shortly. If you didn't request it, you can safely ignore this email."
	}
	return c
}

type codeEmailData struct {
	BrandName string
	LogoURL   string
	// AccentColor is placed directly into a CSS value, not text content —
	// html/template applies CSS-context escaping to it regardless of type,
	// so it stays a plain string.
	AccentColor string
	// TitleTag/Title/Subtext/FooterNote are template.HTML, not string:
	// these come from CodeEmailContent, written by the calling
	// application's own Go code (the same trust level the original,
	// pre-CodeEmailContent template gave this copy when it was literal
	// template source rather than a data substitution) — using string
	// here would have html/template's auto-escaper HTML-escape it (e.g.
	// turning "it's" into "it&#39;s"), which the original static text was
	// never subject to. Never populate these from end-user input.
	TitleTag   template.HTML
	Title      template.HTML
	Subtext    template.HTML
	FooterNote template.HTML
	Code       string
	Year       int
}

// codeEmailTemplate is a table-based layout with inline styles — the
// safest combination for rendering consistently across email clients
// (Gmail, Apple Mail, Outlook), which strip <style> blocks and modern CSS.
var codeEmailTemplate = template.Must(template.New("code-email").Parse(`<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>{{if .BrandName}}{{.BrandName}} {{end}}{{.TitleTag}}</title>
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
<div style="font-size:20px;font-weight:600;color:#111827;margin-bottom:8px;">{{.Title}}</div>
<div style="font-size:14px;color:#6b7280;margin-bottom:24px;">{{.Subtext}}</div>
<div style="display:inline-block;padding:16px 24px;background-color:#f9fafb;border-radius:8px;font-size:32px;font-weight:700;letter-spacing:8px;color:{{.AccentColor}};margin-bottom:24px;">{{.Code}}</div>
<div style="font-size:13px;color:#9ca3af;">{{.FooterNote}}</div>
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

// RenderCodeEmailHTML renders the built-in branded HTML template for any
// code-driven email. Use it directly, or pass NewBrandedCodeHTMLBody's
// result as SMTPConfig.HTMLBody to wire it into SMTPSender.
func RenderCodeEmailHTML(branding EmailBranding, content CodeEmailContent, code string) (string, error) {
	branding = branding.withDefaults()
	content = content.withDefaults()

	var buf bytes.Buffer
	err := codeEmailTemplate.Execute(&buf, codeEmailData{
		BrandName:   branding.BrandName,
		LogoURL:     branding.LogoURL,
		AccentColor: branding.AccentColor,
		TitleTag:    template.HTML(content.TitleTag),   //nolint:gosec // developer-supplied copy, not end-user input — see codeEmailData's doc comment
		Title:       template.HTML(content.Title),      //nolint:gosec // same as above
		Subtext:     template.HTML(content.Subtext),    //nolint:gosec // same as above
		FooterNote:  template.HTML(content.FooterNote), //nolint:gosec // same as above
		Code:        code,
		Year:        time.Now().Year(),
	})
	if err != nil {
		return "", fmt.Errorf("auth: failed to render code email template: %w", err)
	}
	return buf.String(), nil
}

// RenderVerificationEmailHTML renders the built-in branded HTML template
// for a verification code — a thin wrapper over RenderCodeEmailHTML using
// the default "Verify your email" copy, kept as its own entry point since
// it's the most common case and predates CodeEmailContent.
func RenderVerificationEmailHTML(branding EmailBranding, code string) (string, error) {
	return RenderCodeEmailHTML(branding, CodeEmailContent{}, code)
}

// NewBrandedHTMLBody returns an SMTPConfig.HTMLBody function backed by
// the built-in branded template — the brand name is the only thing you
// supply; the copyright line is fixed.
func NewBrandedHTMLBody(branding EmailBranding) func(code string) (string, error) {
	return func(code string) (string, error) {
		return RenderVerificationEmailHTML(branding, code)
	}
}

// NewBrandedCodeHTMLBody is NewBrandedHTMLBody's generalized counterpart —
// use it for any code-driven email whose copy isn't "Verify your email"
// (e.g. a caregiver invite code), while still reusing the same branded
// layout.
func NewBrandedCodeHTMLBody(branding EmailBranding, content CodeEmailContent) func(code string) (string, error) {
	return func(code string) (string, error) {
		return RenderCodeEmailHTML(branding, content, code)
	}
}
