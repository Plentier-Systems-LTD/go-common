package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// AppleProvider verifies "Sign in with Apple" identity tokens against
// Apple's published JWKS.
type AppleProvider struct {
	clientIDs []string
	jwks      *jwksCache
}

// NewAppleProvider configures Apple identity token verification.
// clientIDs are the accepted audiences — your app's bundle ID and/or your
// web Services ID, whichever ones you expect tokens to be issued for.
func NewAppleProvider(clientIDs ...string) (*AppleProvider, error) {
	if len(clientIDs) == 0 {
		return nil, errors.New("auth: apple provider requires at least one client id")
	}
	return &AppleProvider{
		clientIDs: clientIDs,
		jwks:      newJWKSCache("https://appleid.apple.com/auth/keys"),
	}, nil
}

type appleClaims struct {
	Email string `json:"email"`
	// Apple encodes this as a bool for native flows and a string for some
	// web flows, so it's decoded loosely and normalized below.
	EmailVerified any `json:"email_verified"`
	jwt.RegisteredClaims
}

func (a *AppleProvider) VerifyIDToken(ctx context.Context, idToken string) (*Identity, error) {
	claims := &appleClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	_, err := parser.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token header missing kid")
		}
		return a.jwks.key(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("auth: invalid apple identity token: %w", err)
	}

	if claims.Issuer != "https://appleid.apple.com" {
		return nil, fmt.Errorf("auth: unexpected apple token issuer %q", claims.Issuer)
	}
	if !audienceAllowed(claims.Audience, a.clientIDs) {
		return nil, fmt.Errorf("auth: unexpected apple token audience %v", claims.Audience)
	}
	if claims.Subject == "" {
		return nil, errors.New("auth: apple token missing subject")
	}

	verified := false
	switch v := claims.EmailVerified.(type) {
	case bool:
		verified = v
	case string:
		verified = v == "true"
	}

	return &Identity{
		Provider:      ProviderApple,
		ProviderID:    claims.Subject,
		Email:         claims.Email,
		EmailVerified: verified,
	}, nil
}

func audienceAllowed(aud jwt.ClaimStrings, allowed []string) bool {
	for _, a := range aud {
		for _, allow := range allowed {
			if a == allow {
				return true
			}
		}
	}
	return false
}
