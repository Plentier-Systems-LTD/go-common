package auth

import "errors"

var (
	// ErrEmailTaken is returned by Service.Register when the email is
	// already associated with an account.
	ErrEmailTaken = errors.New("auth: email already registered")

	// ErrInvalidCredentials is returned by Service.Login when the email or
	// password is wrong. It intentionally doesn't distinguish the two, so
	// callers can't use it to enumerate registered emails.
	ErrInvalidCredentials = errors.New("auth: invalid email or password")

	// ErrUserNotFound is the error a UserStore implementation must return
	// (wrapped or not, errors.Is must still match) when a lookup finds no
	// matching user.
	ErrUserNotFound = errors.New("auth: user not found")

	// ErrInvalidToken is returned when a token is malformed, expired, or
	// fails signature verification.
	ErrInvalidToken = errors.New("auth: invalid token")

	// ErrWrongTokenType is returned when an access token is presented
	// where a refresh token is required, or vice versa.
	ErrWrongTokenType = errors.New("auth: wrong token type")
)
