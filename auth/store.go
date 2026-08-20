package auth

import "context"

// UserStore persists and retrieves users. Implement it against whatever
// database your project uses to plug Service into your project — see the
// go-common/gorm/auth package for a ready-made GORM implementation.
//
// Lookup methods must return ErrUserNotFound (wrapped or not, so that
// errors.Is(err, ErrUserNotFound) still matches) when no user is found.
type UserStore[T User] interface {
	Create(ctx context.Context, user T) error
	Update(ctx context.Context, user T) error
	FindByID(ctx context.Context, id string) (T, error)
	FindByEmail(ctx context.Context, email string) (T, error)
	FindByProvider(ctx context.Context, provider Provider, providerID string) (T, error)
	FindByGuestKey(ctx context.Context, guestKey string) (T, error)
}
