# adminapi

The read-only contract a platform implements so a central admin/stats dashboard can pull its user
counts and user list, without the dashboard ever writing to the platform or owning a copy of its
full schema. Framework-agnostic; see [`fiber/adminapi`](../fiber/adminapi) for the Fiber routes
that serve this contract.

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## What's in here

- **`PlatformStats`** / **`UserSummary`** / **`UserPage`** / **`TrendPoint`** — the wire types the
  dashboard reads.
- **`Provider`** — the interface a platform implements: `PlatformStats(ctx)`,
  `ListUsers(ctx, page, limit, search)`, and `SignupTrend(ctx, days)`. Deliberately small.
- **`fiber/adminapi.Mount`** — registers `GET /internal/stats`, `GET /internal/users`, and
  `GET /internal/stats/trend` behind `RequireAPIKey`, given a `Provider`. "Import package,
  register route" is the entire integration surface a platform needs.

## Example

```go
// internal/adminstats/provider.go
type Provider struct{}

func (Provider) PlatformStats(ctx context.Context) (adminapi.PlatformStats, error) {
    var userCount, subscribed int64
    sqlr.GORM().WithContext(ctx).Model(&models.User{}).Count(&userCount)
    // ...count subscribed via billing.Subscription, count new-in-window, etc.
    return adminapi.PlatformStats{UserCount: userCount, SubscribedCount: subscribed, GeneratedAt: time.Now()}, nil
}

func (Provider) ListUsers(ctx context.Context, page, limit int, search string) (adminapi.UserPage, error) {
    // query models.User (+ left join billing.Subscription), ILIKE search on email/name
}

func (Provider) SignupTrend(ctx context.Context, days int) ([]adminapi.TrendPoint, error) {
    // group models.User by day for the last `days` days, similarly for new Subscription rows
}
```

```go
// server/server.go
fiberadminapi.Mount(app, adminstats.Provider{}, cfg.InternalAPIKey)
```

The dashboard side (registering platforms, polling, caching, its own admin-facing API) is a
separate service, not part of go-common — only the contract both sides agree on lives here.

## Design notes

- **Pull, not push** — the dashboard calls a platform's `/internal/*` endpoints; a platform never
  calls out to the dashboard. A platform has no dashboard SDK to configure beyond mounting these
  two routes.
- **`RequireAPIKey` is a shared secret, not a JWT** — the caller is a trusted service, not a user
  session, so this avoids needing the dashboard to hold a signing-compatible secret (or the
  platform to trust a cross-service token issuer) for what's fundamentally one API key per
  platform.
- **No `lastActiveAt` by default** — `UserSummary.LastActiveAt` is `nil` until a platform starts
  recording last-login/last-request timestamps; it's optional so adopting this contract doesn't
  require that first.
