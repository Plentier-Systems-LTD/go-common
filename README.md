# go-common

Shared Go libraries for Plentier backend services. Extracted from
[Therapist-api](https://github.com/Plentier-Systems-LTD/Therapist-api) for reuse across projects,
starting with [medbill](https://github.com/Plentier-Systems-LTD/medbill).

## Layout

Framework-agnostic packages live at the top level; anything that depends on a specific web
framework or ORM is nested under that dependency's name, so consumers only pull in what they
actually use. Each package documents itself in its own README.

```
go-common/
├── auth/         User model, registration/login, Apple/Google login, JWTs — no framework, no storage dependency
├── billing/      Apple/Google purchase verification — no framework, no storage dependency
├── fiber/
│   ├── auth/     Fiber middleware wrapping auth (RequireAuth, OptionalAuth, User)
│   └── cors/     Fiber CORS middleware
└── gorm/
    └── auth/     Ready-made GORM UserStore implementation for auth.Service
```

A service on a different framework (chi, net/http, gin, ...) or a different ORM can depend on
`auth` and `billing` directly without ever importing Fiber or GORM — the framework/storage
integrations are opt-in, nested packages.

## Packages

- **[`auth`](auth/README.md)** — user model, registration, password/Apple/Google login, JWT
  issuance and verification. Includes the companion `fiber/auth` middleware and `gorm/auth`
  storage packages.
- **[`billing`](billing/README.md)** — Apple App Store / Google Play purchase verification and
  webhook parsing, plus a `Service`/`Handler`/`Initialize` for persisting subscriptions.

### `fiber/cors`

```go
import fibercors "github.com/Plentier-Systems-LTD/go-common/fiber/cors"

app.Use(fibercors.New(fibercors.Config{AllowOrigins: "*"}))
```

## Versioning

Tag releases (`git tag vX.Y.Z`) and consume with `go get github.com/Plentier-Systems-LTD/go-common@vX.Y.Z`.
