# auth

Everything needed to authenticate users — password + Apple/Google login, JWT issuance, user
persistence — independent of any web framework or database. This package is the core; the
Fiber middleware and GORM store described below are optional companions nested under their own
dependency (`fiber/auth`, `gorm/auth`) so you only pull in what you actually use.

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## What's in here

- **`BaseUser`** — a struct with the fields every project needs (ID, email, password hash,
  provider, verified flag, timestamps). Embed it in your own user model to inherit those fields
  and the methods that satisfy the `User` interface, the same way `gorm.Model` works.
- **`Service[T User]`** — orchestrates registration, password login, Apple/Google login, and
  token refresh. It depends only on the `UserStore`, `PasswordHasher`, and `IdentityProvider`
  interfaces below (dependency inversion), so it never needs to know what database or framework
  it's running inside of.
- **`AppleProvider` / `GoogleProvider`** — verify Sign in with Apple / Google identity tokens
  against each provider's live JWKS (fetched and cached automatically). Both implement
  `IdentityProvider`, so adding a third provider (e.g. Facebook) later means writing one small
  type, not touching `Service`.
- **`GenerateTokenPair` / `VerifyToken` / `VerifyRefreshToken` / `RefreshTokenPair`** — access +
  refresh JWT issuance and verification. Access and refresh tokens are tagged with a `Type` claim
  so one can't be used in place of the other.
- **`PasswordHasher`** — bcrypt by default (`NewBcryptHasher`), swappable via
  `WithPasswordHasher` if a project needs a different algorithm.
- **`SendVerificationEmail` / `VerifyEmail`** — generate and check a numeric email verification
  code. `EmailSender` is pluggable (bring SES/SendGrid/Postmark, or use the zero-dependency
  `NewSMTPSender`); `VerificationStore` persists outstanding codes. Codes expire, are attempt-
  limited, and are rate-limited on resend by default — see [Email verification](#email-verification) below.

## Design (SOLID)

- **Single responsibility** — each file owns one concern: `tokens.go` (JWTs), `password.go`
  (hashing), `apple.go` / `google.go` (one identity provider each), `verification.go` /
  `smtp.go` (email codes), `service.go` (orchestration), `user.go` (the embeddable user model).
- **Open/closed** — new social login providers are added by implementing `IdentityProvider`;
  `Service` never changes.
- **Liskov substitution** — any type embedding `BaseUser` can stand in wherever the `User`
  interface is expected, since the promoted methods satisfy it automatically.
- **Interface segregation** — `UserStore`, `PasswordHasher`, `IdentityProvider`,
  `VerificationStore`, and `EmailSender` are small, single-purpose interfaces instead of one
  large "auth backend" interface.
- **Dependency inversion** — `Service` is constructed with those interfaces injected in
  (`NewService(store, cfg, opts...)`); it never imports GORM, Fiber, or a specific provider SDK.

## Minimal usage — just tokens

If a project already has its own user model and registration flow and only wants JWT
issuance/verification:

```go
cfg := auth.Config{Secret: os.Getenv("JWT_SECRET")}
pair, err := auth.GenerateTokenPair(cfg, user.ID, user.Email)
claims, err := auth.VerifyToken(cfg, tokenString)
```

## Full example

This walks through plugging `auth` into a Fiber + GORM project from scratch: your own user
model, the identity providers, the service, handlers, routes, and protecting an endpoint. Swap
the Fiber/GORM pieces for whatever your project uses — `auth.Service` itself doesn't care.

### 1. Your user model — embed `BaseUser` to inherit it

```go
// models/user.go
package models

import "github.com/Plentier-Systems-LTD/go-common/auth"

type User struct {
    auth.BaseUser        // ID, Email, PasswordHash, Provider, EmailVerified, CreatedAt, UpdatedAt
    FirstName string      `json:"firstName"`
    LastName  string      `json:"lastName"`
    Phone     string      `json:"phone"`
}
```

`BaseUser`'s fields carry `gorm` tags already, so `db.AutoMigrate(&models.User{})` just works —
GORM picks up `FirstName`/`LastName`/`Phone` alongside the inherited ones.

### 2. Wire the service once at startup

```go
// main.go
package main

import (
    "log"
    "os"
    "time"

    "github.com/Plentier-Systems-LTD/go-common/auth"
    gormauth "github.com/Plentier-Systems-LTD/go-common/gorm/auth"
    "github.com/gofiber/fiber/v2"
    "myapp/models"
)

func main() {
    db := mustOpenDB()
    if err := db.AutoMigrate(&models.User{}); err != nil {
        log.Fatal(err)
    }

    google, err := auth.NewGoogleProvider(os.Getenv("GOOGLE_CLIENT_ID"))
    if err != nil {
        log.Fatal(err)
    }
    apple, err := auth.NewAppleProvider(os.Getenv("APPLE_BUNDLE_ID"))
    if err != nil {
        log.Fatal(err)
    }

    authSvc, err := auth.NewService[*models.User](
        gormauth.NewStore[models.User](db),
        auth.Config{
            Secret:          os.Getenv("JWT_SECRET"),
            AccessTokenTTL:  15 * time.Minute,
            RefreshTokenTTL: 30 * 24 * time.Hour,
        },
        auth.WithGoogleProvider[*models.User](google),
        auth.WithAppleProvider[*models.User](apple),
    )
    if err != nil {
        log.Fatal(err)
    }

    app := fiber.New()
    registerRoutes(app, authSvc)
    log.Fatal(app.Listen(":8080"))
}
```

### 3. Handlers — your own DTOs, `Service` does the auth logic

```go
// handlers/auth.go
package handlers

import (
    "github.com/Plentier-Systems-LTD/go-common/auth"
    fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
    "github.com/gofiber/fiber/v2"
    "myapp/api"
    "myapp/models"
)

type AuthHandler struct {
    svc *auth.Service[*models.User]
}

func NewAuthHandler(svc *auth.Service[*models.User]) *AuthHandler {
    return &AuthHandler{svc: svc}
}

func (h *AuthHandler) Register(c *fiber.Ctx) error {
    var req api.RegisterRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    u := &models.User{
        BaseUser:  auth.NewBaseUser(req.Email),
        FirstName: req.FirstName,
        LastName:  req.LastName,
        Phone:     req.Phone,
    }

    user, tokens, err := h.svc.Register(c.UserContext(), u, req.Password)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(fiber.Map{"user": user, "tokens": tokens})
}

func (h *AuthHandler) Login(c *fiber.Ctx) error {
    var req api.LoginRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    user, tokens, err := h.svc.Login(c.UserContext(), req.Email, req.Password)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(fiber.Map{"user": user, "tokens": tokens})
}

// GoogleLogin and AppleLogin share one implementation — OAuthLogin only
// needs to know which provider to verify against.
func (h *AuthHandler) oauthLogin(provider auth.Provider) fiber.Handler {
    return func(c *fiber.Ctx) error {
        var req api.OAuthLoginRequest
        if err := c.BodyParser(&req); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
        }

        user, tokens, isNewAccount, err := h.svc.OAuthLogin(c.UserContext(), provider, req.IDToken,
            func(identity auth.Identity) *models.User {
                return &models.User{
                    BaseUser:  auth.NewBaseUser(identity.Email),
                    FirstName: identity.Name,
                }
            })
        if err != nil {
            return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(fiber.Map{"user": user, "tokens": tokens, "isNewAccount": isNewAccount})
    }
}

func (h *AuthHandler) GoogleLogin(c *fiber.Ctx) error { return h.oauthLogin(auth.ProviderGoogle)(c) }
func (h *AuthHandler) AppleLogin(c *fiber.Ctx) error  { return h.oauthLogin(auth.ProviderApple)(c) }

func (h *AuthHandler) Refresh(c *fiber.Ctx) error {
    var req api.RefreshRequest
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    tokens, err := h.svc.Refresh(c.UserContext(), req.RefreshToken)
    if err != nil {
        return c.Status(fiber.StatusUnauthorized).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(fiber.Map{"tokens": tokens})
}

// Profile is registered behind fiberauth.RequireAuth — see routes below.
func (h *AuthHandler) Profile(c *fiber.Ctx) error {
    claims, _ := fiberauth.User(c)
    return c.JSON(fiber.Map{"id": claims.UserID, "email": claims.Email})
}
```

### 4. Routes — register the handlers and protect what needs protecting

```go
// routes/routes.go
package routes

import (
    sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
    fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
    "github.com/gofiber/fiber/v2"
    "myapp/handlers"
)

func Register(app *fiber.App, h *handlers.AuthHandler, cfg sharedauth.Config) {
    v1 := app.Group("/api/v1")

    v1.Post("/auth/register", h.Register)
    v1.Post("/auth/login", h.Login)
    v1.Post("/auth/login/google", h.GoogleLogin)
    v1.Post("/auth/login/apple", h.AppleLogin)
    v1.Post("/auth/refresh", h.Refresh)

    v1.Use(fiberauth.RequireAuth(cfg))
    v1.Get("/profile", h.Profile)
}
```

That's the whole integration: your model embeds `BaseUser`, `Service` handles the auth logic
against your `UserStore`, and `fiberauth.RequireAuth` protects routes using the same `Config`
the service signs tokens with. Swapping ORM or web framework later only means writing a new
`UserStore` or a new middleware file — `Service` and everything that calls it stays untouched.

## Email verification

`BaseUser.EmailVerified` starts `false` for password accounts (Google/Apple logins set it from
the identity provider's own `email_verified` claim instead — see `OAuthLogin`). To let users
verify a password account's email with a one-time code:

### 1. Wire a `VerificationStore` and `EmailSender` into the service

```go
verificationStore, err := gormauth.NewVerificationStore(db) // migrates its own table
if err != nil {
    log.Fatal(err)
}

emailSender, err := auth.NewSMTPSender(auth.SMTPConfig{
    Host:     "smtp.sendgrid.net",
    Port:     587,
    Username: "apikey",
    Password: os.Getenv("SENDGRID_API_KEY"),
    From:     "MyApp <noreply@myapp.com>",
})
if err != nil {
    log.Fatal(err)
}

authSvc, err := auth.NewService[*models.User](
    gormauth.NewStore[models.User](db),
    auth.Config{Secret: os.Getenv("JWT_SECRET")},
    auth.WithVerificationStore[*models.User](verificationStore),
    auth.WithEmailSender[*models.User](emailSender),
    // Optional — these are the defaults:
    auth.WithVerificationConfig[*models.User](auth.VerificationConfig{
        CodeLength:     6,
        TTL:            15 * time.Minute,
        MaxAttempts:    5,
        ResendCooldown: 60 * time.Second,
    }),
)
```

`NewSMTPSender` works against Gmail, Amazon SES SMTP, SendGrid SMTP, Mailgun SMTP, and similar —
it's the zero-dependency default. Using a provider's HTTP API instead (e.g. the SendGrid Web
API) just means implementing `EmailSender` yourself:

```go
type SendGridSender struct{ client *sendgrid.Client }

func (s *SendGridSender) SendVerificationCode(ctx context.Context, to, code string) error {
    // build and send your own templated email via the provider's API
}
```

### 2. Send a code, then verify it

```go
// handlers/auth.go
func (h *AuthHandler) SendVerificationEmail(c *fiber.Ctx) error {
    var req struct {
        Email string `json:"email"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    if err := h.svc.SendVerificationEmail(c.UserContext(), req.Email); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    return c.SendStatus(fiber.StatusOK)
}

func (h *AuthHandler) VerifyEmail(c *fiber.Ctx) error {
    var req struct {
        Email string `json:"email"`
        Code  string `json:"code"`
    }
    if err := c.BodyParser(&req); err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
    }

    user, err := h.svc.VerifyEmail(c.UserContext(), req.Email, req.Code)
    if err != nil {
        return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
    }
    return c.JSON(fiber.Map{"user": user})
}
```

```go
// routes/routes.go
v1.Post("/auth/send-verification-email", h.SendVerificationEmail)
v1.Post("/auth/verify-email", h.VerifyEmail)
```

Call `SendVerificationEmail` right after `Register` if you want verification to kick off
immediately, or leave it for the user to trigger from a "resend code" button — both are just
calling the same method. Whether to block unverified users out of protected routes (checking
`claims`/your stored user's `EmailVerified`) is a product decision left to your own middleware;
`Service` doesn't enforce it.

### Defaults and failure modes

- Codes are 6 digits, valid for 15 minutes, and stored as a SHA-256 hash (never the raw code) —
  tune via `WithVerificationConfig`.
- `VerifyEmail` locks a code out after 5 wrong guesses (`ErrTooManyAttempts`); the user must
  request a fresh one.
- `SendVerificationEmail` refuses to send a second code within 60 seconds of the last one
  (`ErrVerificationCodeResendTooSoon`), to keep it from being used to spam an inbox.
- Both return `ErrEmailVerificationNotConfigured` if the service wasn't built with
  `WithVerificationStore`/`WithEmailSender` — verification is entirely opt-in.

## Companion packages

### `fiber/auth`

Fiber middleware built on top of `auth`. Depends on Fiber; nothing else in `auth` does.

```go
import (
    sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
    fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
)

cfg := sharedauth.Config{Secret: os.Getenv("JWT_SECRET")}

app.Get("/profile", fiberauth.RequireAuth(cfg), func(c *fiber.Ctx) error {
    claims, _ := fiberauth.User(c) // *sharedauth.Claims
    return c.JSON(fiber.Map{"id": claims.UserID, "email": claims.Email})
})

// Lets both guests and logged-in users through; claims are attached only
// when a valid token was sent.
app.Get("/feed", fiberauth.OptionalAuth(cfg), handlers.Feed)
```

### `gorm/auth`

A generic `auth.UserStore` implementation backed by GORM, so most projects don't need to write
their own persistence layer. Depends on GORM; nothing else in `auth` does.

```go
import gormauth "github.com/Plentier-Systems-LTD/go-common/gorm/auth"

db.AutoMigrate(&models.User{}) // your model, your migration — the store doesn't own the schema

store := gormauth.NewStore[models.User](db) // implements auth.UserStore[*models.User]
```

Using a different database? Implement `auth.UserStore[*models.User]` yourself — it's five
methods (`Create`, `Update`, `FindByID`, `FindByEmail`, `FindByProvider`); see
[`store.go`](store.go) for the exact contract, including that lookup misses must return
`auth.ErrUserNotFound`.

`gorm/auth` also provides `NewVerificationStore(db)`, a ready-made `auth.VerificationStore` for
[email verification](#email-verification) — unlike `Store[T]`, it migrates its own table since
the schema is fixed and owned by `auth`, not by your project.
