package auth

import "time"

// Config holds the settings needed to issue and verify JWTs. Secret is
// required; everything else has a sensible default.
type Config struct {
	// Secret signs and verifies tokens (HS256). Required.
	Secret string

	// Issuer is stamped into the "iss" claim. Defaults to "go-common/auth".
	Issuer string

	// AccessTokenTTL is how long access tokens are valid for. Defaults to 15 minutes.
	AccessTokenTTL time.Duration

	// RefreshTokenTTL is how long refresh tokens are valid for. Defaults to 30 days.
	RefreshTokenTTL time.Duration
}

func (c Config) withDefaults() Config {
	if c.Issuer == "" {
		c.Issuer = "go-common/auth"
	}
	if c.AccessTokenTTL <= 0 {
		c.AccessTokenTTL = 15 * time.Minute
	}
	if c.RefreshTokenTTL <= 0 {
		c.RefreshTokenTTL = 30 * 24 * time.Hour
	}
	return c
}
