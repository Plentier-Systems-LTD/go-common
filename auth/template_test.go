package auth

import (
	"strconv"
	"strings"
	"testing"
	"time"
)

// substituteYear replaces the "YEAR" placeholder with the current year,
// matching codeEmailTemplate's use of time.Now().Year() — the copyright
// line is the only part of the output that legitimately varies by when
// the test runs.
func substituteYear(t *testing.T, s string) string {
	t.Helper()
	return strings.ReplaceAll(s, "YEAR", strconv.Itoa(time.Now().Year()))
}

// The original, pre-CodeEmailContent template literal — kept verbatim as
// the ground truth for TestRenderVerificationEmailHTMLUnchanged below, so
// a future edit to codeEmailTemplate can't silently change what existing
// verification-only consumers receive.
const wantVerificationHTML = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>Acme verification code</title>
</head>
<body style="margin:0;padding:0;background-color:#f4f4f7;font-family:-apple-system,BlinkMacSystemFont,'Segoe UI',Roboto,Helvetica,Arial,sans-serif;">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="background-color:#f4f4f7;padding:32px 16px;">
<tr>
<td align="center">
<table role="presentation" width="100%" cellpadding="0" cellspacing="0" style="max-width:480px;background-color:#ffffff;border-radius:8px;overflow:hidden;">
<tr>
<td style="padding:32px 32px 24px 32px;text-align:center;">

<div style="font-size:18px;font-weight:600;color:#111827;margin-bottom:24px;">Acme</div>
<div style="font-size:20px;font-weight:600;color:#111827;margin-bottom:8px;">Verify your email</div>
<div style="font-size:14px;color:#6b7280;margin-bottom:24px;">Enter this code to confirm it's you.</div>
<div style="display:inline-block;padding:16px 24px;background-color:#f9fafb;border-radius:8px;font-size:32px;font-weight:700;letter-spacing:8px;color:#4F46E5;margin-bottom:24px;">123456</div>
<div style="font-size:13px;color:#9ca3af;">This code expires shortly. If you didn't request it, you can safely ignore this email.</div>
</td>
</tr>
<tr>
<td style="padding:20px 32px;background-color:#f9fafb;text-align:center;font-size:12px;color:#9ca3af;">
&copy; YEAR Plentier Systems. All rights reserved.
</td>
</tr>
</table>
</td>
</tr>
</table>
</body>
</html>
`

// TestRenderVerificationEmailHTMLUnchanged locks in that CodeEmailContent's
// generalization (added to let non-verification code emails, like a
// caregiver invite, reuse this template) produces byte-for-byte identical
// output for the existing verification path — the vast majority of
// go-common's consumers only ever send a verification email and never
// construct CodeEmailContent themselves.
func TestRenderVerificationEmailHTMLUnchanged(t *testing.T) {
	got, err := RenderVerificationEmailHTML(EmailBranding{BrandName: "Acme"}, "123456")
	if err != nil {
		t.Fatalf("RenderVerificationEmailHTML: %v", err)
	}
	want := substituteYear(t, wantVerificationHTML)
	if got != want {
		t.Fatalf("output changed for the default verification path.\ngot:\n%s\nwant:\n%s", got, want)
	}
}

func TestNewBrandedHTMLBodyUnchanged(t *testing.T) {
	body, err := NewBrandedHTMLBody(EmailBranding{BrandName: "Acme"})("123456")
	if err != nil {
		t.Fatalf("NewBrandedHTMLBody: %v", err)
	}
	want := substituteYear(t, wantVerificationHTML)
	if body != want {
		t.Fatalf("NewBrandedHTMLBody output changed.\ngot:\n%s\nwant:\n%s", body, want)
	}
}
