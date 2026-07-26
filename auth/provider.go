package auth

import "context"

// Identity is the normalized profile returned by an IdentityProvider after
// verifying a client-supplied identity token.
type Identity struct {
	Provider      Provider
	ProviderID    string // the token's "sub" claim
	Email         string
	EmailVerified bool
	Name          string
	Picture       string
}

// IdentityProvider verifies a client-supplied identity token (a Sign in
// with Apple or Google ID token) and returns the identity it asserts.
// Implement this interface to add a new social login provider without
// touching Service — that's the whole seam.
type IdentityProvider interface {
	VerifyIDToken(ctx context.Context, idToken string) (*Identity, error)
}
