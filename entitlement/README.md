# entitlement

Gates a feature behind "has an active subscription OR hasn't used up a lifetime free-tier
allowance yet" (or, more simply, "has an active subscription" with no free tier at all). Built on
top of [`billing`](../billing)'s `Service.IsSubscribed`/`Subscription`. Framework-agnostic; see
[`fiber/entitlement`](../fiber/entitlement) for the Fiber middleware.

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## What's in here

- **`Kind`** / **`Limits`** — a project defines its own `Kind` values (e.g. `"document"`,
  `"chat"`) and how many free lifetime uses each gets.
- **`Counter`** — the interface a project implements once, switching on `Kind` to call its own
  per-feature count queries (how many documents/messages/etc. has this user ever used).
- **`ActiveSubscription`** — reads `billing.Subscription` directly for the plan/expiry detail a
  client-facing status endpoint typically needs, complementing `Service.IsSubscribed`'s plain bool.
- **`fiber/entitlement.RequireEntitlement`** / **`RequireSubscription`** — the Fiber middleware.

## Example

```go
type Counter struct{}

func (Counter) Count(ctx context.Context, kind entitlement.Kind, userID string) (int64, error) {
    switch kind {
    case KindDocument:
        return models.CountUserDocuments(ctx, userID)
    case KindChat:
        return models.CountUserChatMessages(ctx, userID)
    default:
        return 0, fmt.Errorf("unknown kind %q", kind)
    }
}

limits := entitlement.Limits{KindDocument: cfg.FreeDocumentLimit, KindChat: cfg.FreeChatLimit}

app.Post("/documents", fiberentitlement.RequireEntitlement(billingSvc, limits, KindDocument, Counter{},
    func(kind entitlement.Kind, limit int) fiber.Map {
        return fiber.Map{"message": "free limit reached", "code": "free_limit_reached", "limitType": string(kind)}
    },
), handlers.UploadDocument)

// premium-only, no free tier at all
app.Post("/compare", fiberentitlement.RequireSubscription(billingSvc, fiber.Map{
    "message": "This feature requires a Premium subscription.",
    "code":    "subscription_required",
}), handlers.CompareDocuments)
```

## Design notes

- **`billingSvc` may be `nil`** — both middlewares treat a `nil` `*billing.Service` as "never
  subscribed", so free-tier limits alone govern access in environments where billing isn't
  configured (e.g. local dev), the same way every other optional integration degrades rather than
  hard-failing.
- **Counting logic stays in your app** — only you know your own tables/columns; this package only
  defines the shape (`Counter`) the middleware needs to call it generically.
