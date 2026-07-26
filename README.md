# go-common

Shared Go libraries for Plentier backend services. Extracted from
[Therapist-api](https://github.com/Plentier-Systems-LTD/Therapist-api) for reuse across projects,
starting with [medbill](https://github.com/Plentier-Systems-LTD/medbill).

## Layout

Framework-agnostic packages live at the top level; anything that depends on a specific web
framework is nested under that framework's name, so consumers only pull in the framework
dependency they actually use.

```
go-common/
├── auth/         JWT issuance/verification — no framework dependency
├── billing/      Apple/Google purchase verification — no framework, no storage dependency
├── chat/         Gemini-backed AI chat replies — no framework, no storage dependency
├── voice/        OpenAI Whisper audio transcription — no framework dependency
└── fiber/
    ├── auth/     Fiber middleware wrapping auth (RequireAuth, OptionalAuth, User)
    └── cors/     Fiber CORS middleware
```

A service on a different framework (chi, net/http, gin, ...) can depend on `auth` and `billing`
directly without ever importing Fiber.

## Packages

### `auth`

JWT issuance/verification. Framework-agnostic.

```go
cfg := auth.Config{Secret: os.Getenv("JWT_SECRET")}
pair, err := auth.GenerateTokenPair(cfg, user.ID, user.Email)
claims, err := auth.VerifyToken(cfg, tokenString)
```

### `fiber/auth`

Fiber middleware built on top of `auth`.

```go
import (
    sharedauth "github.com/Plentier-Systems-LTD/go-common/auth"
    fiberauth "github.com/Plentier-Systems-LTD/go-common/fiber/auth"
)

cfg := sharedauth.Config{Secret: os.Getenv("JWT_SECRET")}

app.Get("/profile", fiberauth.RequireAuth(cfg), func(c *fiber.Ctx) error {
    claims := fiberauth.User(c) // *sharedauth.Claims
    return c.JSON(fiber.Map{"id": claims.UserID, "email": claims.Email})
})
```

### `fiber/cors`

```go
import fibercors "github.com/Plentier-Systems-LTD/go-common/fiber/cors"

app.Use(fibercors.New(fibercors.Config{AllowOrigins: "*"}))
```

### `billing`

Verifies Apple App Store / Google Play purchase receipts and parses their webhook
notifications. Storage- and framework-agnostic — verification returns a `PurchaseResult`;
persisting subscription/transaction state is left to each service's own database.

```go
apple, err := billing.NewAppleProvider(sharedSecret, rootCAPEM)
google, err := billing.NewGoogleProvider(serviceAccountJSON, packageName, pubsubAudience, pubsubServiceAccountEmail)

result, err := apple.VerifyPurchase(ctx, billing.VerifyRequest{
    UserID:      userID,
    ReceiptData: signedTransactionJWS,
})
// result.ExpiresAt, result.IsRefund, result.ProductID, ... -> persist as you see fit
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
