package auth

import (
	"strings"
	"time"

	"github.com/google/uuid"
)

// Provider identifies how a user authenticates.
type Provider string

const (
	ProviderPassword Provider = "password"
	ProviderGoogle   Provider = "google"
	ProviderApple    Provider = "apple"
)

// User is the contract Service needs from a project's user model. Embed
// BaseUser in your own struct to satisfy it for free — its methods have
// pointer receivers, so use the pointer type (*YourUser) wherever this
// package expects a User.
type User interface {
	GetID() string
	GetEmail() string
	GetPasswordHash() string
	SetPasswordHash(hash string)
	GetProvider() Provider
	SetProvider(provider Provider, providerID string)
	IsEmailVerified() bool
	SetEmailVerified(verified bool)
}

// BaseUser holds the fields every project needs for authentication.
// Embed it in your own user model to inherit them:
//
//	type User struct {
//	    auth.BaseUser
//	    FirstName string
//	    LastName  string
//	}
type BaseUser struct {
	ID            string    `gorm:"primaryKey" json:"id"`
	Email         string    `gorm:"uniqueIndex;not null" json:"email"`
	PasswordHash  string    `gorm:"column:password_hash" json:"-"`
	Provider      Provider  `gorm:"not null;default:password" json:"provider"`
	ProviderID    string    `gorm:"index" json:"-"`
	EmailVerified bool      `gorm:"not null;default:false" json:"emailVerified"`
	CreatedAt     time.Time `gorm:"autoCreateTime" json:"createdAt"`
	UpdatedAt     time.Time `gorm:"autoUpdateTime" json:"updatedAt"`
}

// NewBaseUser builds a BaseUser ready for password registration: a fresh
// ID and a normalized (lowercased, trimmed) email. Use it when building
// the value you pass to Service.Register.
func NewBaseUser(email string) BaseUser {
	return BaseUser{
		ID:       uuid.NewString(),
		Email:    NormalizeEmail(email),
		Provider: ProviderPassword,
	}
}

// NormalizeEmail lowercases and trims an email so lookups and uniqueness
// checks are consistent regardless of how the client cased it.
func NormalizeEmail(email string) string {
	return strings.ToLower(strings.TrimSpace(email))
}

func (u *BaseUser) GetID() string {
	return u.ID
}

func (u *BaseUser) GetEmail() string {
	return u.Email
}

func (u *BaseUser) GetPasswordHash() string {
	return u.PasswordHash
}

func (u *BaseUser) SetPasswordHash(hash string) {
	u.PasswordHash = hash
}

func (u *BaseUser) GetProvider() Provider {
	return u.Provider
}

func (u *BaseUser) SetProvider(provider Provider, providerID string) {
	u.Provider = provider
	u.ProviderID = providerID
}

func (u *BaseUser) IsEmailVerified() bool {
	return u.EmailVerified
}

func (u *BaseUser) SetEmailVerified(verified bool) {
	u.EmailVerified = verified
}
