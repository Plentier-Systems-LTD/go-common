package auth

import (
	"fmt"
	"strings"

	sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
	"gorm.io/gorm"
)

// SMTPConfig configures email verification for Bootstrap. Host and From
// are both required for verification to be enabled — leave the zero value
// to disable it entirely.
type SMTPConfig struct {
	Host, Username, Password, From string
	Port                           int

	// Subject and Body customize the verification email's text part; both
	// fall back to sharedauth.SMTPConfig's defaults when left zero.
	Subject string
	Body    func(code string) string

	// Branding drives the built-in HTML verification email
	// (sharedauth.NewBrandedHTMLBody).
	Branding sharedauth.EmailBranding
}

// BootstrapConfig configures Bootstrap.
type BootstrapConfig struct {
	JWTSecret string

	// GoogleOAuthClientIDs / AppleOAuthClientIDs are comma-separated
	// client/bundle IDs. Sign-in for that provider is only enabled when
	// its list is non-empty.
	GoogleOAuthClientIDs string
	AppleOAuthClientIDs  string

	// SMTP enables email verification when non-nil with Host and From set.
	SMTP *SMTPConfig

	// OnWarn is called whenever an optional integration (Google/Apple
	// sign-in, email verification) is skipped because it's unconfigured or
	// failed to initialize. Defaults to a no-op; wire it to your logger to
	// see why something didn't come up.
	OnWarn func(msg string, err error)
}

// BootstrapResult is what Bootstrap wires up.
type BootstrapResult[PT sharedauth.User] struct {
	Service       *sharedauth.Service[PT]
	GoogleEnabled bool
	AppleEnabled  bool

	// Close shuts down the async email sender, if verification was
	// enabled — safe to call even when it wasn't (a no-op then). Call it
	// during graceful shutdown so an email queued right before exit still
	// gets delivered instead of silently dropped.
	Close func()
}

// Bootstrap wires a sharedauth.Service[PT] around a GORM-backed Store for
// T, with Google/Apple sign-in and SMTP email verification each
// independently enabled only when configured — the same
// configure-or-degrade pattern every optional integration typically
// follows, so a project's own server/auth_setup.go collapses to a single
// call.
//
//	type AppUser struct {
//	    auth.BaseUser
//	    FullName string
//	}
//
//	result, err := gormauth.Bootstrap[AppUser](db, gormauth.BootstrapConfig{
//	    JWTSecret:            cfg.JWTSecret,
//	    GoogleOAuthClientIDs: cfg.GoogleOAuthClientIDs,
//	    AppleOAuthClientIDs:  cfg.AppleOAuthClientIDs,
//	    SMTP: &gormauth.SMTPConfig{
//	        Host: cfg.SMTPHost, Port: cfg.SMTPPort, From: cfg.SMTPFrom,
//	        Subject:  "Your MyApp verification code",
//	        Branding: sharedauth.EmailBranding{BrandName: "MyApp"},
//	    },
//	})
func Bootstrap[T any, PT Model[T]](db *gorm.DB, cfg BootstrapConfig) (*BootstrapResult[PT], error) {
	warn := cfg.OnWarn
	if warn == nil {
		warn = func(string, error) {}
	}

	store := NewStore[T, PT](db)

	var opts []sharedauth.Option[PT]
	closeFn := func() {}

	googleEnabled := false
	if ids := splitCommaList(cfg.GoogleOAuthClientIDs); len(ids) > 0 {
		provider, err := sharedauth.NewGoogleProvider(ids...)
		if err != nil {
			warn("google sign-in disabled: failed to initialize provider", err)
		} else {
			opts = append(opts, sharedauth.WithGoogleProvider[PT](provider))
			googleEnabled = true
		}
	}

	appleEnabled := false
	if ids := splitCommaList(cfg.AppleOAuthClientIDs); len(ids) > 0 {
		provider, err := sharedauth.NewAppleProvider(ids...)
		if err != nil {
			warn("apple sign-in disabled: failed to initialize provider", err)
		} else {
			opts = append(opts, sharedauth.WithAppleProvider[PT](provider))
			appleEnabled = true
		}
	}

	if cfg.SMTP != nil && cfg.SMTP.Host != "" && cfg.SMTP.From != "" {
		verificationStore, err := NewVerificationStore(db)
		if err != nil {
			warn("email verification disabled: failed to initialize store", err)
		} else {
			sender, err := sharedauth.NewSMTPSender(sharedauth.SMTPConfig{
				Host:     cfg.SMTP.Host,
				Port:     cfg.SMTP.Port,
				Username: cfg.SMTP.Username,
				Password: cfg.SMTP.Password,
				From:     cfg.SMTP.From,
				Subject:  cfg.SMTP.Subject,
				Body:     cfg.SMTP.Body,
				HTMLBody: sharedauth.NewBrandedHTMLBody(cfg.SMTP.Branding),
			})
			if err != nil {
				warn("email verification disabled: failed to initialize SMTP sender", err)
			} else {
				asyncSender := sharedauth.NewAsyncEmailSender(sender, sharedauth.AsyncEmailSenderConfig{
					OnError: func(to string, err error) { warn("failed to deliver verification email to "+to, err) },
				})
				opts = append(opts,
					sharedauth.WithVerificationStore[PT](verificationStore),
					sharedauth.WithEmailSender[PT](asyncSender),
				)
				closeFn = asyncSender.Close
			}
		}
	}

	svc, err := sharedauth.NewService[PT](store, sharedauth.Config{Secret: cfg.JWTSecret}, opts...)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to initialize service: %w", err)
	}

	return &BootstrapResult[PT]{
		Service:       svc,
		GoogleEnabled: googleEnabled,
		AppleEnabled:  appleEnabled,
		Close:         closeFn,
	}, nil
}

func splitCommaList(s string) []string {
	if s == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}
