# engagement

Records aggregate, non-PII product-usage events — "this kind of thing happened, for this user, at
this time" — the moment it actually happens, so the platform-stats dashboard can chart real usage
instead of a platform inferring it after the fact by counting rows in its own domain tables. That
approach drifts: every platform names and shapes that data differently, some of it isn't even
exported outside the platform's own `models` package, and the numbers only ever reflect whatever
happens to already be in the database — not "this genuinely just happened." This package is the
fix: **plug it in once, call `Record` at the real call sites, done.**

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## Quick start

```go
// server.go, once at startup
store, err := gormengagement.NewStore(db)              // auto-migrates its own table
recorder := engagement.NewRecorder(store, engagement.RecorderConfig{
    OnError: func(err error) { log.Warn("engagement", zap.Error(err)) },
})
defer recorder.Close() // during graceful shutdown

// wherever a real action succeeds — a handler, a model function, anywhere
// with the user ID in scope
recorder.Record(userID, engagement.FileUploaded)
recorder.Record(userID, engagement.AIAnalysis)
recorder.Record(userID, engagement.AIChatMessage)
```

That's the entire integration surface. `Record` never blocks and never fails the request it's
attached to — it hands the event to a small internal queue + worker pool (the same shape
`auth.AsyncEmailSender` uses) and returns immediately.

## Wiring it into `internal/adminstats.Provider`

The three standard event types map straight onto `adminapi.PlatformStats` and `adminapi.TrendPoint`
— once a platform records events, its `Provider` just reads them back instead of writing bespoke
per-domain-table SQL:

```go
type Provider struct {
    store *gormengagement.Store // the same Store the Recorder writes through
}

func (p Provider) PlatformStats(ctx context.Context) (sharedadminapi.PlatformStats, error) {
    ...
    stats.FilesUploadedTotal, _ = p.store.Total(ctx, sharedengagement.FileUploaded)
    stats.AIAnalysesTotal, _ = p.store.Total(ctx, sharedengagement.AIAnalysis)
    stats.AIChatMessagesTotal, _ = p.store.Total(ctx, sharedengagement.AIChatMessage)
    ...
}

func (p Provider) SignupTrend(ctx context.Context, days int) ([]sharedadminapi.TrendPoint, error) {
    ...
    filesByDay, _ := p.store.DailyCounts(ctx, sharedengagement.FileUploaded, since)
    analysesByDay, _ := p.store.DailyCounts(ctx, sharedengagement.AIAnalysis, since)
    chatByDay, _ := p.store.DailyCounts(ctx, sharedengagement.AIChatMessage, since)
    ...
}
```

## What's in here

- **`Event` / `EventType`** — a user ID, a type, a timestamp. Nothing else — no content, no
  request/response bodies, nothing that could reconstruct what a user actually did or said.
- **`FileUploaded` / `AIAnalysis` / `AIChatMessage`** — the three standard event types the
  dashboard already knows how to chart. `EventType` is still just a `string` underneath, so a
  platform can record its own additional types too (they just won't have a dashboard chart yet).
- **`Store`** — the persistence interface (`Record`, `Total`, `DailyCounts`). Implement it
  against whatever database a project uses; see [`gorm/engagement`](../gorm/engagement) for a
  ready-made GORM implementation that auto-migrates its own `engagement_events` table.
- **`Recorder`** — wraps a `Store` with the async queue + worker pool `Record` uses. This is the
  thing every call site actually calls; nothing outside `internal/adminstats` needs to touch
  `Store` directly.

## Design notes

- **Record at the real moment, not after the fact.** `Record` belongs right after a file lands in
  storage or an AI call returns — not in a nightly job, not inferred from a `created_at` column on
  an unrelated table. The whole point is that the count can never silently drift from what actually
  happened.
- **Fire-and-forget by design.** A platform's real feature must never fail or slow down because an
  analytics write failed or the DB was briefly slow. `Record` enqueues and returns; a full queue
  drops the event (via `OnError`) rather than blocking.
- **One event per action, not per artifact.** A multi-file upload in one request is one
  `FileUploaded` event, not one per file — it mirrors "how many times did someone engage with this
  feature," not a raw row count.
