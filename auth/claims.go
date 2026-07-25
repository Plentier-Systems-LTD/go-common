// Package auth provides JWT issuance/verification and a Fiber middleware
// for protecting routes, shared across Plentier backend services.
package auth

import "github.com/golang-jwt/jwt/v5"

// Claims is the payload carried in an access or refresh token.
type Claims struct {
	UserID string `json:"id"`
	Email  string `json:"email"`
	jwt.RegisteredClaims
}
