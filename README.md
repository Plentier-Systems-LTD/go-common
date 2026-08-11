# go-common

Shared Go libraries for Plentier backend services. Extracted from
[Therapist-api](https://github.com/Plentier-Systems-LTD/Therapist-api) for reuse across projects,
now consumed by [medbill](https://github.com/Plentier-Systems-LTD/medbill) and
[contractlens](https://github.com/Plentier-Systems-LTD/contractlens).

## Layout

Framework-agnostic packages live at the top level; anything that depends on a specific web
framework or ORM is nested under that dependency's name, so consumers only pull in what they
actually use. Each package documents itself in its own README (the simpler ones inline below).

```
go-common/
├── auth/            User model, registration/login, Apple/Google login, JWTs
├── billing/         Apple/Google purchase verification
├── entitlement/     Subscription-or-free-tier-allowance gating on top of billing
├── adminapi/        Read-only stats/user-list contract a platform exposes to an admin dashboard
├── engagement/      Records "this happened" usage events (files uploaded, AI calls) for adminapi
├── push/            Push notifications (ExpoSender) to a user's registered devices
├── storage/         S3 object storage (put/url/delete), SSE-S3 encrypted
├── logging/         Structured JSON logging (zap)
├── chat/            Gemini-backed AI chat replies
├── voice/           OpenAI Whisper audio transcription
├── fiber/
│   ├── auth/        Fiber middleware wrapping auth (RequireAuth, OptionalAuth, User)
│   ├── billing/     Fiber middleware wrapping billing (PremiumProtected)
│   ├── entitlement/ Fiber middleware wrapping entitlement (RequireEntitlement, RequireSubscription)
│   ├── adminapi/    Mounts adminapi's contract as Fiber routes (RequireAPIKey, Mount)
│   ├── httpx/       Tiny shared response helpers (SendError)
│   ├── logging/     Fiber request-logging middleware
│   └── cors/        Fiber CORS middleware
└── gorm/
    ├── auth/        GORM UserStore + Bootstrap (wires Service, Google/Apple, SMTP verification)
    ├── entitlement/ GORM-backed ActiveSubscription lookup
    ├── engagement/  GORM-backed engagement.Store (auto-migrates its own table)
    ├── push/        GORM device-token store for push
    └── postgres/    Opens the shared sqlr/GORM Postgres connection
```

A service on a different framework (chi, net/http, gin, ...) or a different ORM can depend on
`auth`, `billing`, `entitlement`, `push`, etc. directly without ever importing Fiber or GORM — the
framework/storage integrations are opt-in, nested packages.

## Packages

- **[`auth`](auth/README.md)** — user model, registration, password/Apple/Google login, JWT
  issuance and verification. Includes the companion `fiber/auth` middleware and `gorm/auth`
  storage packages. `gorm/auth.Bootstrap` wires the whole thing (store, optional Google/Apple,
  optional SMTP email verification) in one call.
- **[`billing`](billing/README.md)** — Apple App Store / Google Play purchase verification and
  webhook parsing, plus a `Service`/`Handler`/`Initialize` for persisting subscriptions.
- **[`entitlement`](entitlement/README.md)** — gates a feature behind "subscribed OR hasn't used
  up a lifetime free-tier allowance", built on `billing`. See `fiber/entitlement` for the
  middleware.
- **[`push`](push/README.md)** — sends push notifications (`ExpoSender` today) to a user's
  registered devices; `gorm/push` persists the device tokens.
- **[`adminapi`](adminapi/README.md)** — the read-only contract (`GET /internal/stats`,
  `GET /internal/users` via `fiber/adminapi.Mount`) a platform implements so a central admin/stats
  dashboard can read its user counts and user list.
- **[`engagement`](engagement/README.md)** — records aggregate, non-PII usage events (a file was
  uploaded, an AI call was made) at the moment they happen; `internal/adminstats.Provider` reads
  them back to fill `adminapi.PlatformStats`'s engagement fields instead of inferring counts from a
  platform's own domain tables after the fact. `gorm/engagement.NewStore` is the ready-made GORM
  backend, auto-migrating its own table.

### `fiber/cors`

```go
import fibercors "github.com/Plentier-Systems-LTD/go-common/fiber/cors"

app.Use(fibercors.New(fibercors.Config{AllowOrigins: "*"}))
```

### `storage`

Puts and removes objects in an S3 bucket, SSE-S3 encrypted. Every write sets a
`Cache-Control: public, max-age=31536000, immutable` header — safe only because it's the caller's
responsibility to write each object under a fresh, never-reused key (e.g. a UUID); nothing is ever
overwritten in place.

```go
client, err := storage.New(ctx, region, accessKeyID, secretAccessKey, bucket)

url, err := client.Put(ctx, uuid.NewString(), "image/png", data)
err = client.Delete(ctx, []string{key1, key2})
```

### `logging` / `fiber/logging`

Structured JSON logging via zap, plus a Fiber request-logging middleware.

```go
log, err := logging.New(cfg.Env, cfg.LogLevel) // human timestamps in "development", JSON otherwise

app.Use(fiberlogging.RequestLogger(log)) // one JSON line per request
```

### `gorm/postgres`

Opens the shared Postgres connection every domain package in a project reaches via
`github.com/ochom/gutils/sqlr`'s package-level singleton.

```go
db, err := postgres.Init(postgres.Config{DSN: cfg.DBUrl, Debug: cfg.LogLevel == "debug"})
// sqlr.GORM() now returns db from anywhere in the project
```

### `fiber/httpx`

```go
return httpx.SendError(c, fiber.StatusBadRequest, "invalid request")
// {"message": "invalid request"}
```

### `chat`

Generates conversational AI replies via Gemini. Extracted from Therapist-api's chat feature and
generalized: the persona (system prompt) and conversation history are supplied by the caller
instead of being hardcoded and read from a database, so each service brings its own assistant
persona and owns its own message storage.

```go
client, err := chat.New(ctx, chat.Config{APIKey: os.Getenv("GEMINI_API_KEY")})

reply, err := client.GenerateReply(ctx, systemPrompt, history, "What does this charge cover?")
// history is []chat.Message{{Role: chat.RoleUser, ...}, {Role: chat.RoleModel, ...}, ...},
// oldest first — fetch it from your own chat_messages table before calling.
```

### `voice`

Transcribes recorded audio to text via OpenAI's Whisper API. Framework- and storage-agnostic.
Handlers should depend on the `Transcriber` interface, not `WhisperClient` directly, so tests can
fake it.

```go
client, err := voice.NewWhisperClient(voice.Config{APIKey: os.Getenv("OPENAI_API_KEY")})

text, err := client.Transcribe(ctx, voice.TranscribeRequest{
    Filename: "recording.m4a",
    Audio:    audioReader,
})
```

## Versioning

Tag releases (`git tag vX.Y.Z`) and consume with `go get github.com/Plentier-Systems-LTD/go-common@vX.Y.Z`.
