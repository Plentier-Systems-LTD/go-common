# go-common

Shared Go libraries for Plentier backend services (auth, billing, entitlement, adminapi, push,
storage, logging, chat, voice — see README.md for the full package layout). Consumed by
contractlens, medbill, doseline, keeper, and other Plentier platform services.

## Always wire up admin stats on a new platform

Every service built on go-common — new or existing — **must mount the `adminapi` contract** so
the central platform-stats dashboard can see it. This is not an optional add-on to bolt on later;
treat it as part of the standard setup checklist for any new platform, the same as wiring auth or
Postgres:

1. Add `internal/adminstats/provider.go` implementing `adminapi.Provider` (`PlatformStats`,
   `ListUsers`, `SignupTrend`) — and also `adminapi.MutableProvider` (`UpdateUser`, `DeleteUser`)
   so the dashboard can edit/delete users too, not just read them. Query the platform's own `User`
   model plus `billing.Subscription` for plan status — see contractlens, medbill, doseline, or
   keeper's `internal/adminstats` for a working template (doseline/keeper thread `*gorm.DB`
   explicitly; contractlens/medbill use `sqlr.GORM()` — match whichever convention the platform
   already uses for its own queries).
2. Add an `InternalAPIKey` field to `config.Config` (env var `INTERNAL_API_KEY`), documented in
   `.env.example`. No safe default — leaving it unset must disable the integration, never mount it
   with a guessable key.
3. In `server.go`, mount it conditionally, logging (don't crash) when the key is unset:
   ```go
   if cfg.InternalAPIKey == "" {
       log.Warn("admin stats dashboard disabled: INTERNAL_API_KEY not set")
   } else {
       fiberadminapi.Mount(app, adminstats.New(db), cfg.InternalAPIKey) // or adminstats.Provider{} if the platform uses sqlr.GORM() globally
   }
   ```
4. After deploying, register the platform in the dashboard (or set `PLATFORM_<NAME>_URL`/
   `_API_KEY` env vars there) with a base URL reachable from the dashboard and this same key.

Skipping this step is exactly what happened with Doseline and Keeper — both existed and were
reachable in production for a while with no `/internal/*` routes mounted at all, so the dashboard
could never poll them and every request 404'd. Don't let a new platform ship the same gap.

See [`adminapi/README.md`](adminapi/README.md) for the full contract reference.
