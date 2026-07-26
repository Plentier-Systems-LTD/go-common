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

	// ErrVerificationCodeNotFound is the error a VerificationStore
	// implementation must return (wrapped or not, errors.Is must still
	// match) when a lookup finds no outstanding code.
	ErrVerificationCodeNotFound = errors.New("auth: verification code not found")

	// ErrVerificationCodeExpired is returned by Service.VerifyEmail when
	// the most recent code for the email has expired.
	ErrVerificationCodeExpired = errors.New("auth: verification code expired")

	// ErrInvalidVerificationCode is returned by Service.VerifyEmail when
	// the submitted code doesn't match.
	ErrInvalidVerificationCode = errors.New("auth: invalid verification code")

	// ErrTooManyAttempts is returned by Service.VerifyEmail once a code has
	// been guessed wrong VerificationConfig.MaxAttempts times; the caller
	// must request a new code.
	ErrTooManyAttempts = errors.New("auth: too many attempts, request a new code")

	// ErrVerificationCodeResendTooSoon is returned by
	// Service.SendVerificationEmail when called again for the same email
	// before VerificationConfig.ResendCooldown has elapsed.
	ErrVerificationCodeResendTooSoon = errors.New("auth: verification code was already sent recently")

	// ErrEmailVerificationNotConfigured is returned by
	// Service.SendVerificationEmail/VerifyEmail when the Service wasn't
	// built with WithVerificationStore and WithEmailSender.
	ErrEmailVerificationNotConfigured = errors.New("auth: email verification not configured")
)
