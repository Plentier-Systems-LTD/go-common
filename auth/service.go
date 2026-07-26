package auth

import (
	"context"
	"errors"
	"fmt"
)

// Option configures a Service at construction time.
type Option[T User] func(*Service[T])

// WithPasswordHasher overrides the default bcrypt PasswordHasher.
func WithPasswordHasher[T User](h PasswordHasher) Option[T] {
	return func(s *Service[T]) { s.hasher = h }
}

// WithGoogleProvider enables Google identity-token login.
func WithGoogleProvider[T User](p IdentityProvider) Option[T] {
	return func(s *Service[T]) { s.providers[ProviderGoogle] = p }
}

// WithAppleProvider enables Sign in with Apple identity-token login.
func WithAppleProvider[T User](p IdentityProvider) Option[T] {
	return func(s *Service[T]) { s.providers[ProviderApple] = p }
}

// Service orchestrates registration, login, social login, and token
// issuance. It depends only on the interfaces in this package (UserStore,
// PasswordHasher, IdentityProvider) — never on a concrete database or web
// framework — so it drops into any project that supplies those three.
//
// T is the project's own user type (typically a pointer, e.g. *AppUser)
// embedding BaseUser.
type Service[T User] struct {
	store     UserStore[T]
	hasher    PasswordHasher
	cfg       Config
	providers map[Provider]IdentityProvider
}

// NewService wires a Service around a UserStore and token Config.
// Social login providers and the password hasher are optional
// (WithGoogleProvider, WithAppleProvider, WithPasswordHasher); password
// hashing defaults to bcrypt.
func NewService[T User](store UserStore[T], cfg Config, opts ...Option[T]) (*Service[T], error) {
	if cfg.Secret == "" {
		return nil, errors.New("auth: Config.Secret is required")
	}

	s := &Service[T]{
		store:     store,
		hasher:    NewBcryptHasher(),
		cfg:       cfg.withDefaults(),
		providers: map[Provider]IdentityProvider{},
	}
	for _, opt := range opts {
		opt(s)
	}
	return s, nil
}

// Register creates a password account. Pass in a user value pre-populated
// with your own model's fields — built with auth.NewBaseUser(email) to
// seed the ID and normalized email — plus the plaintext password.
// Register hashes the password, persists the user, and returns a fresh
// token pair.
func (s *Service[T]) Register(ctx context.Context, user T, password string) (T, *TokenPair, error) {
	var zero T

	if password == "" {
		return zero, nil, errors.New("auth: password is required")
	}

	if _, err := s.store.FindByEmail(ctx, user.GetEmail()); err == nil {
		return zero, nil, ErrEmailTaken
	} else if !errors.Is(err, ErrUserNotFound) {
		return zero, nil, err
	}

	hash, err := s.hasher.Hash(password)
	if err != nil {
		return zero, nil, fmt.Errorf("auth: failed to hash password: %w", err)
	}
	user.SetPasswordHash(hash)
	user.SetProvider(ProviderPassword, "")

	if err := s.store.Create(ctx, user); err != nil {
		return zero, nil, fmt.Errorf("auth: failed to create user: %w", err)
	}

	tokens, err := GenerateTokenPair(s.cfg, user.GetID(), user.GetEmail())
	if err != nil {
		return zero, nil, err
	}
	return user, tokens, nil
}

// Login authenticates a password account.
func (s *Service[T]) Login(ctx context.Context, email, password string) (T, *TokenPair, error) {
	var zero T

	user, err := s.store.FindByEmail(ctx, NormalizeEmail(email))
	if err != nil {
		return zero, nil, ErrInvalidCredentials
	}

	if err := s.hasher.Compare(user.GetPasswordHash(), password); err != nil {
		return zero, nil, ErrInvalidCredentials
	}

	tokens, err := GenerateTokenPair(s.cfg, user.GetID(), user.GetEmail())
	if err != nil {
		return zero, nil, err
	}
	return user, tokens, nil
}

// OAuthLogin verifies a Google/Apple identity token and either logs in the
// matching user (matched by provider ID, falling back to email) or
// creates one via newUser. newUser is only called when no existing
// account is found; build the value the same way you would for Register,
// using fields from the given Identity.
//
// The bool return reports whether a new account was created.
func (s *Service[T]) OAuthLogin(ctx context.Context, provider Provider, idToken string, newUser func(Identity) T) (T, *TokenPair, bool, error) {
	var zero T

	p, ok := s.providers[provider]
	if !ok {
		return zero, nil, false, fmt.Errorf("auth: no identity provider configured for %q", provider)
	}

	identity, err := p.VerifyIDToken(ctx, idToken)
	if err != nil {
		return zero, nil, false, err
	}

	if user, err := s.store.FindByProvider(ctx, provider, identity.ProviderID); err == nil {
		tokens, err := GenerateTokenPair(s.cfg, user.GetID(), user.GetEmail())
		return user, tokens, false, err
	} else if !errors.Is(err, ErrUserNotFound) {
		return zero, nil, false, err
	}

	if identity.Email != "" {
		if existing, err := s.store.FindByEmail(ctx, NormalizeEmail(identity.Email)); err == nil {
			existing.SetProvider(provider, identity.ProviderID)
			if err := s.store.Update(ctx, existing); err != nil {
				return zero, nil, false, fmt.Errorf("auth: failed to link provider to existing user: %w", err)
			}
			tokens, err := GenerateTokenPair(s.cfg, existing.GetID(), existing.GetEmail())
			return existing, tokens, false, err
		}
	}

	user := newUser(*identity)
	user.SetProvider(provider, identity.ProviderID)
	user.SetEmailVerified(identity.EmailVerified)

	if err := s.store.Create(ctx, user); err != nil {
		return zero, nil, false, fmt.Errorf("auth: failed to create user: %w", err)
	}

	tokens, err := GenerateTokenPair(s.cfg, user.GetID(), user.GetEmail())
	if err != nil {
		return zero, nil, false, err
	}
	return user, tokens, true, nil
}

// Refresh verifies a refresh token, confirms the user still exists, and
// issues a new token pair.
func (s *Service[T]) Refresh(ctx context.Context, refreshToken string) (*TokenPair, error) {
	claims, err := VerifyRefreshToken(s.cfg, refreshToken)
	if err != nil {
		return nil, err
	}

	if _, err := s.store.FindByID(ctx, claims.UserID); err != nil {
		return nil, ErrUserNotFound
	}

	return GenerateTokenPair(s.cfg, claims.UserID, claims.Email)
}
