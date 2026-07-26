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
├── chat/         Gemini-backed AI chat replies — no framework, no storage dependency
├── voice/        OpenAI Whisper audio transcription — no framework dependency
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
