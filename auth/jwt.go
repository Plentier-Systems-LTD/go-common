package auth

import (
	"errors"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

// Config holds the signing secret and token lifetimes for a service.
type Config struct {
	Secret          string
	AccessTokenTTL  time.Duration // defaults to 15m if zero
	RefreshTokenTTL time.Duration // defaults to 30 days if zero
}

func (c Config) accessTTL() time.Duration {
	if c.AccessTokenTTL == 0 {
		return 15 * time.Minute
	}
	return c.AccessTokenTTL
}

func (c Config) refreshTTL() time.Duration {
	if c.RefreshTokenTTL == 0 {
		return 30 * 24 * time.Hour
	}
	return c.RefreshTokenTTL
}

// TokenPair is the access/refresh token pair returned on login/register.
type TokenPair struct {
	AccessToken  string `json:"accessToken"`
	RefreshToken string `json:"refreshToken"`
}

// GenerateTokenPair mints a new access and refresh token for a user.
func GenerateTokenPair(cfg Config, userID, email string) (TokenPair, error) {
	now := time.Now()

	access, err := signClaims(cfg.Secret, Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.accessTTL())),
		},
	})
	if err != nil {
		return TokenPair{}, err
	}

	refresh, err := signClaims(cfg.Secret, Claims{
		UserID: userID,
		Email:  email,
		RegisteredClaims: jwt.RegisteredClaims{
			IssuedAt:  jwt.NewNumericDate(now),
			ExpiresAt: jwt.NewNumericDate(now.Add(cfg.refreshTTL())),
		},
	})
	if err != nil {
		return TokenPair{}, err
	}

	return TokenPair{AccessToken: access, RefreshToken: refresh}, nil
}

func signClaims(secret string, claims Claims) (string, error) {
	return jwt.NewWithClaims(jwt.SigningMethodHS256, claims).SignedString([]byte(secret))
}

// VerifyToken validates a token's signature and expiry and returns its claims.
func VerifyToken(cfg Config, tokenString string) (*Claims, error) {
	claims := &Claims{}

	token, err := jwt.ParseWithClaims(tokenString, claims, func(t *jwt.Token) (interface{}, error) {
		if _, ok := t.Method.(*jwt.SigningMethodHMAC); !ok {
			return nil, errors.New("auth: unexpected signing method")
		}
		return []byte(cfg.Secret), nil
	})
	if err != nil {
		return nil, err
	}
	if !token.Valid {
		return nil, errors.New("auth: invalid token")
	}

	return claims, nil
}
