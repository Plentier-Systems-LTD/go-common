package auth

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"crypto/subtle"
	"encoding/hex"
	"fmt"
	"math/big"
	"time"
)

// EmailSender delivers a verification code to a user's inbox. auth never
// talks to an SMTP server or email API itself — implement this against
// whatever provider your project uses (SES, SendGrid, Postmark, ...), or
// use the zero-dependency NewSMTPSender. Swapping providers never touches
// Service.
type EmailSender interface {
	SendVerificationCode(ctx context.Context, to string, code string) error
}

// VerificationCode is a single outstanding email verification attempt.
// The schema is fixed and owned by this package (see gorm/auth, which
// migrates it for you) — it never needs to be embedded or extended.
type VerificationCode struct {
	ID        string    `gorm:"primaryKey" json:"-"`
	Email     string    `gorm:"index;not null" json:"-"`
	CodeHash  string    `gorm:"not null" json:"-"`
	Attempts  int       `gorm:"not null;default:0" json:"-"`
	ExpiresAt time.Time `gorm:"index;not null" json:"-"`
	CreatedAt time.Time `gorm:"autoCreateTime;index" json:"-"`
}

// VerificationStore persists outstanding email verification codes.
// Implement it against whatever database your project uses — see the
// go-common/gorm/auth package for a ready-made GORM implementation.
type VerificationStore interface {
	Create(ctx context.Context, code VerificationCode) error

	// FindLatest returns the most recently created outstanding code for
	// an email. Must return ErrVerificationCodeNotFound (wrapped or not,
	// so errors.Is still matches) when none exists.
	FindLatest(ctx context.Context, email string) (VerificationCode, error)

	IncrementAttempts(ctx context.Context, id string) error
	Delete(ctx context.Context, id string) error
}

// VerificationConfig tunes email verification codes. All fields default
// to sane values when left zero.
type VerificationConfig struct {
	// CodeLength is how many digits the numeric code has. Defaults to 6.
	CodeLength int

	// TTL is how long a code stays valid after it's sent. Defaults to 15 minutes.
	TTL time.Duration

	// MaxAttempts is how many wrong guesses are allowed before a code is
	// locked out and a new one must be requested. Defaults to 5.
	MaxAttempts int

	// ResendCooldown is the minimum time between two codes being sent to
	// the same email, to slow down abuse of your email provider. Defaults
	// to 60 seconds.
	ResendCooldown time.Duration
}

func (c VerificationConfig) withDefaults() VerificationConfig {
	if c.CodeLength <= 0 {
		c.CodeLength = 6
	}
	if c.TTL <= 0 {
		c.TTL = 15 * time.Minute
	}
	if c.MaxAttempts <= 0 {
		c.MaxAttempts = 5
	}
	if c.ResendCooldown <= 0 {
		c.ResendCooldown = 60 * time.Second
	}
	return c
}

func generateNumericCode(length int) (string, error) {
	const digits = "0123456789"
	code := make([]byte, length)
	for i := range code {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(digits))))
		if err != nil {
			return "", fmt.Errorf("auth: failed to generate verification code: %w", err)
		}
		code[i] = digits[n.Int64()]
	}
	return string(code), nil
}

func hashCode(code string) string {
	sum := sha256.Sum256([]byte(code))
	return hex.EncodeToString(sum[:])
}

func codeMatches(hash, code string) bool {
	return subtle.ConstantTimeCompare([]byte(hash), []byte(hashCode(code))) == 1
}
