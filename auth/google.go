package auth

import (
	"context"
	"errors"
	"fmt"

	"github.com/golang-jwt/jwt/v5"
)

// GoogleProvider verifies Google ID tokens against Google's published JWKS.
type GoogleProvider struct {
	clientIDs []string
	jwks      *jwksCache
}

// NewGoogleProvider configures Google ID token verification. clientIDs are
// the OAuth client IDs (web/iOS/Android) your app expects tokens to be
// issued for.
func NewGoogleProvider(clientIDs ...string) (*GoogleProvider, error) {
	if len(clientIDs) == 0 {
		return nil, errors.New("auth: google provider requires at least one client id")
	}
	return &GoogleProvider{
		clientIDs: clientIDs,
		jwks:      newJWKSCache("https://www.googleapis.com/oauth2/v3/certs"),
	}, nil
}

type googleClaims struct {
	Email         string `json:"email"`
	EmailVerified bool   `json:"email_verified"`
	Name          string `json:"name"`
	Picture       string `json:"picture"`
	jwt.RegisteredClaims
}

func (g *GoogleProvider) VerifyIDToken(ctx context.Context, idToken string) (*Identity, error) {
	claims := &googleClaims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"}))
	_, err := parser.ParseWithClaims(idToken, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token header missing kid")
		}
		return g.jwks.key(ctx, kid)
	})
	if err != nil {
		return nil, fmt.Errorf("auth: invalid google identity token: %w", err)
	}

	if claims.Issuer != "https://accounts.google.com" && claims.Issuer != "accounts.google.com" {
		return nil, fmt.Errorf("auth: unexpected google token issuer %q", claims.Issuer)
	}
	if !audienceAllowed(claims.Audience, g.clientIDs) {
		return nil, fmt.Errorf("auth: unexpected google token audience %v", claims.Audience)
	}
	if claims.Subject == "" {
		return nil, errors.New("auth: google token missing subject")
	}

	return &Identity{
		Provider:      ProviderGoogle,
		ProviderID:    claims.Subject,
		Email:         claims.Email,
		EmailVerified: claims.EmailVerified,
		Name:          claims.Name,
		Picture:       claims.Picture,
	}, nil
}
