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

	// ResetLinkURL, if set, builds a clickable reset-password URL from the
	// recipient's email and the code — e.g.
	//
	//	func(to, code string) string {
	//	    return fmt.Sprintf("%s/?resetEmail=%s&resetCode=%s", frontendURL, url.QueryEscape(to), url.QueryEscape(code))
	//	}
	//
	// The email then leads with that link (sharedauth.NewBrandedHTMLBodyWithLink)
	// instead of showing only the bare code.
	ResetLinkURL func(to, code string) string
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
// T, enabling Google/Apple sign-in and SMTP email verification only when
// their config is present (see BootstrapConfig). See the package README
// for a full example.
func Bootstrap[T any, PT Model[T]](db *gorm.DB, cfg BootstrapConfig) (*BootstrapResult[PT], error) {
	warn := cfg.OnWarn
	if warn == nil {
		warn = func(string, error) {}
	}

	var opts []sharedauth.Option[PT]

	googleOpt, googleEnabled := setupGoogleProvider[PT](cfg.GoogleOAuthClientIDs, warn)
	if googleEnabled {
		opts = append(opts, googleOpt)
	}
	appleOpt, appleEnabled := setupAppleProvider[PT](cfg.AppleOAuthClientIDs, warn)
	if appleEnabled {
		opts = append(opts, appleOpt)
	}
	verificationOpts, closeFn := setupVerification[PT](db, cfg.SMTP, warn)
	opts = append(opts, verificationOpts...)

	svc, err := sharedauth.NewService[PT](NewStore[T, PT](db), sharedauth.Config{Secret: cfg.JWTSecret}, opts...)
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

// setupGoogleProvider builds the Google sign-in option, or (nil, false) if
// clientIDs is empty or the provider fails to initialize (in which case
// warn is called).
func setupGoogleProvider[PT sharedauth.User](clientIDs string, warn func(string, error)) (sharedauth.Option[PT], bool) {
	ids := splitCommaList(clientIDs)
	if len(ids) == 0 {
		return nil, false
	}
	provider, err := sharedauth.NewGoogleProvider(ids...)
	if err != nil {
		warn("google sign-in disabled: failed to initialize provider", err)
		return nil, false
	}
	return sharedauth.WithGoogleProvider[PT](provider), true
}

// setupAppleProvider is setupGoogleProvider's Apple counterpart.
func setupAppleProvider[PT sharedauth.User](clientIDs string, warn func(string, error)) (sharedauth.Option[PT], bool) {
	ids := splitCommaList(clientIDs)
	if len(ids) == 0 {
		return nil, false
	}
	provider, err := sharedauth.NewAppleProvider(ids...)
	if err != nil {
		warn("apple sign-in disabled: failed to initialize provider", err)
		return nil, false
	}
	return sharedauth.WithAppleProvider[PT](provider), true
}

// setupVerification builds the verification-store + email-sender options
// when cfg is fully configured, plus the Close func for the async sender.
// Returns (nil, no-op) when cfg is nil/incomplete or any step fails (warn
// is called in that case).
func setupVerification[PT sharedauth.User](db *gorm.DB, cfg *SMTPConfig, warn func(string, error)) ([]sharedauth.Option[PT], func()) {
	noop := func() {}
	if cfg == nil || cfg.Host == "" || cfg.From == "" {
		return nil, noop
	}

	verificationStore, err := NewVerificationStore(db)
	if err != nil {
		warn("email verification disabled: failed to initialize store", err)
		return nil, noop
	}

	smtpCfg := sharedauth.SMTPConfig{
		Host:     cfg.Host,
		Port:     cfg.Port,
		Username: cfg.Username,
		Password: cfg.Password,
		From:     cfg.From,
		Subject:  cfg.Subject,
		Body:     cfg.Body,
	}
	if cfg.ResetLinkURL != nil {
		smtpCfg.HTMLBodyWithLink = sharedauth.NewBrandedHTMLBodyWithLink(cfg.Branding, cfg.ResetLinkURL)
	} else {
		smtpCfg.HTMLBody = sharedauth.NewBrandedHTMLBody(cfg.Branding)
	}

	sender, err := sharedauth.NewSMTPSender(smtpCfg)
	if err != nil {
		warn("email verification disabled: failed to initialize SMTP sender", err)
		return nil, noop
	}

	asyncSender := sharedauth.NewAsyncEmailSender(sender, sharedauth.AsyncEmailSenderConfig{
		OnError: func(to string, err error) { warn("failed to deliver verification email to "+to, err) },
	})

	return []sharedauth.Option[PT]{
		sharedauth.WithVerificationStore[PT](verificationStore),
		sharedauth.WithEmailSender[PT](asyncSender),
	}, asyncSender.Close
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
