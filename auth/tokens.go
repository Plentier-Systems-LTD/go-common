package auth

import (
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// TokenType distinguishes access tokens from refresh tokens so one can't
// be used in place of the other.
type TokenType string

const (
	AccessToken  TokenType = "access"
	RefreshToken TokenType = "refresh"
)

// Claims is the payload embedded in every token this package issues.
type Claims struct {
	UserID string    `json:"uid"`
	Email  string    `json:"email"`
	Type   TokenType `json:"typ"`
	jwt.RegisteredClaims
}

// TokenPair is an access/refresh token pair issued on register, login, or refresh.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

func newToken(cfg Config, userID, email string, tokenType TokenType, ttl time.Duration) (string, error) {
	now := time.Now()
	claims := Claims{
		UserID: userID,
		Email:  email,
		Type:   tokenType,
		RegisteredClaims: jwt.RegisteredClaims{
			Issuer:    cfg.Issuer,
			Subject:   userID,
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(ttl)),
		},
	}
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(cfg.Secret))
}

// GenerateTokenPair issues a fresh access/refresh token pair for a user.
func GenerateTokenPair(cfg Config, userID, email string) (*TokenPair, error) {
	cfg = cfg.withDefaults()

	access, err := newToken(cfg, userID, email, AccessToken, cfg.AccessTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate access token: %w", err)
	}

	refresh, err := newToken(cfg, userID, email, RefreshToken, cfg.RefreshTokenTTL)
	if err != nil {
		return nil, fmt.Errorf("auth: failed to generate refresh token: %w", err)
	}

	return &TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func parseToken(cfg Config, tokenString string) (*Claims, error) {
	cfg = cfg.withDefaults()

	claims := &Claims{}
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"HS256"}))
	_, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, fmt.Errorf("%w: %v", ErrInvalidToken, err)
	}

	return claims, nil
}

// VerifyToken verifies an access token and returns its claims.
func VerifyToken(cfg Config, tokenString string) (*Claims, error) {
	claims, err := parseToken(cfg, tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type != AccessToken {
		return nil, ErrWrongTokenType
	}
	return claims, nil
}

// VerifyRefreshToken verifies a refresh token and returns its claims.
func VerifyRefreshToken(cfg Config, tokenString string) (*Claims, error) {
	claims, err := parseToken(cfg, tokenString)
	if err != nil {
		return nil, err
	}
	if claims.Type != RefreshToken {
		return nil, ErrWrongTokenType
	}
	return claims, nil
}

// RefreshTokenPair verifies a refresh token and issues a brand new token pair.
func RefreshTokenPair(cfg Config, refreshToken string) (*TokenPair, error) {
	claims, err := VerifyRefreshToken(cfg, refreshToken)
	if err != nil {
		return nil, err
	}
	return GenerateTokenPair(cfg, claims.UserID, claims.Email)
}
