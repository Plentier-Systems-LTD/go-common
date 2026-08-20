// Package auth provides a ready-made GORM-backed implementation of
// go-common/auth's UserStore interface, so most projects never need to
// write their own persistence layer to use auth.Service.
package auth

import (
	"context"
	"errors"

	sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
	"gorm.io/gorm"
)

// Model is the constraint linking a user struct T to its pointer type PT:
// PT must be *T and satisfy sharedauth.User. This lets Store allocate new
// T values internally while still handing back the pointer type Service
// expects.
type Model[T any] interface {
	*T
	sharedauth.User
}

// Store is a sharedauth.UserStore[PT] backed by GORM. T is your user
// struct (embedding auth.BaseUser); PT is its pointer type and is
// inferred automatically.
//
//	type AppUser struct {
//	    auth.BaseUser
//	    FirstName string
//	}
//
//	store := gormauth.NewStore[AppUser](db) // UserStore[*AppUser]
type Store[T any, PT Model[T]] struct {
	db *gorm.DB
}

// NewStore wraps a *gorm.DB as a sharedauth.UserStore[PT]. Migrate your
// concrete user type yourself (db.AutoMigrate(&AppUser{})) — only you
// know its full shape.
func NewStore[T any, PT Model[T]](db *gorm.DB) *Store[T, PT] {
	return &Store[T, PT]{db: db}
}

func (s *Store[T, PT]) Create(ctx context.Context, user PT) error {
	return s.db.WithContext(ctx).Create(user).Error
}

func (s *Store[T, PT]) Update(ctx context.Context, user PT) error {
	return s.db.WithContext(ctx).Save(user).Error
}

func (s *Store[T, PT]) FindByID(ctx context.Context, id string) (PT, error) {
	return s.find(ctx, "id = ?", id)
}

func (s *Store[T, PT]) FindByEmail(ctx context.Context, email string) (PT, error) {
	return s.find(ctx, "email = ?", sharedauth.NormalizeEmail(email))
}

func (s *Store[T, PT]) FindByProvider(ctx context.Context, provider sharedauth.Provider, providerID string) (PT, error) {
	return s.find(ctx, "provider = ? AND provider_id = ?", provider, providerID)
}

func (s *Store[T, PT]) FindByGuestKey(ctx context.Context, guestKey string) (PT, error) {
	return s.find(ctx, "guest_key = ?", guestKey)
}

func (s *Store[T, PT]) find(ctx context.Context, query string, args ...any) (PT, error) {
	var row T
	err := s.db.WithContext(ctx).Where(query, args...).First(&row).Error
	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return nil, sharedauth.ErrUserNotFound
		}
		return nil, err
	}
	return &row, nil
}
