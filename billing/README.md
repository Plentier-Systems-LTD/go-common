# billing

Verifies Apple App Store / Google Play purchase receipts and parses their webhook
notifications (App Store Server Notifications v2 / Google Play Real-time Developer
Notifications). Framework/storage integration lives in `Initialize`, which is the only place
this package touches Fiber and GORM directly.

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## What's in here

- **`AppleProvider` / `GoogleProvider`** — verify a client-submitted receipt (`VerifyPurchase`)
  and parse a provider webhook payload (`ParseWebhook`) into a normalized `PurchaseResult`. Both
  implement `PaymentProvider`, so `Service` doesn't care which store a purchase came from.
- **`Service`** — persists `Subscription`/`Transaction` rows from a `PurchaseResult`, answers
  `IsSubscribed`, and maps an `OriginalTransactionID` back to a `UserID` for webhooks that don't
  carry the user's ID.
- **`Handler`** — Fiber handlers for the two webhook endpoints (`AppleWebhook`, `GoogleWebhook`).
- **[`fiber/billing.PremiumProtected`](../fiber/billing)** — Fiber middleware gate for
  subscriber-only routes. For free-tier/lifetime-allowance gating instead of a hard subscription
  requirement, see [`fiber/entitlement`](../fiber/entitlement).
- **`Initialize`** — auto-migrates the billing tables and wires a `Service`/`Handler` pair in one
  call. Apple and Google are each independently optional — leaving a provider's credentials unset
  disables just that provider (`InitResult.AppleEnabled`/`GoogleEnabled`) rather than failing
  `Initialize`, since the billing tables (and the free-tier entitlement reads that depend on them)
  still need to exist even when purchase verification itself isn't configured yet.
- **`InitResult.MountWebhooks`** — registers each *enabled* provider's webhook route on a router
  you choose the path/group for.

## Full example

```go
// main.go
package main

import (
    "log"
    "os"

    "github.com/Plentier-Systems-LTD/go-common/billing"
    "github.com/gofiber/fiber/v2"
)

func main() {
    app := fiber.New()
    db := mustOpenDB()

    rootCA, err := os.ReadFile("apple-root-ca.pem")
    if err != nil {
        log.Fatal(err)
    }
    googleCreds, err := billing.GetGoogleCreds("google-service-account.json")
    if err != nil {
        log.Fatal(err)
    }

    result, err := billing.Initialize(db, billing.Config{
        AppleSharedSecret: os.Getenv("APPLE_SHARED_SECRET"),
        AppleRootCAPEM:    rootCA,

        GoogleServiceAccount:           googleCreds,
        GooglePackageName:              "com.myapp.android",
        GooglePubSubAudience:           os.Getenv("GOOGLE_PUBSUB_AUDIENCE"),
        GooglePubSubServiceAccountMail: os.Getenv("GOOGLE_PUBSUB_SA_EMAIL"),
    })
    if err != nil {
        log.Fatal(err)
    }
    result.MountWebhooks(app.Group("/api/v1/billing/webhooks"))
    // Mounted (only for whichever provider was actually configured):
    //   POST /api/v1/billing/webhooks/apple
    //   POST /api/v1/billing/webhooks/google

    registerRoutes(app, result.Service)
    log.Fatal(app.Listen(":8080"))
}
```

### Client-submitted purchase verification

Let a client hand you a freshly completed purchase for immediate entitlement, instead of
waiting on the async webhook:

```go
// handlers/billing.go
func VerifyPurchase(svc *billing.Service) fiber.Handler {
    return func(c *fiber.Ctx) error {
        var req struct {
            Provider      string `json:"provider"` // "apple" | "google"
            ReceiptData   string `json:"receiptData"`
            PlanId        string `json:"planId"`
            TransactionID string `json:"transactionId"`
        }
        if err := c.BodyParser(&req); err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": "invalid request"})
        }

        claims, _ := fiberauth.User(c) // from go-common/fiber/auth, or your own auth
        userID := claims.UserID

        result, err := svc.VerifyAndProcessPurchase(c.UserContext(), userID, req.Provider, billing.VerifyRequest{
            ReceiptData:   req.ReceiptData,
            PlanId:        req.PlanId,
            TransactionID: req.TransactionID,
        })
        if err != nil {
            return c.Status(fiber.StatusBadRequest).JSON(fiber.Map{"error": err.Error()})
        }
        return c.JSON(fiber.Map{"expiresAt": result.ExpiresAt})
    }
}
```

### Gating a route behind an active subscription

```go
import fiberbilling "github.com/Plentier-Systems-LTD/go-common/fiber/billing"

premium := v1.Group("", fiberbilling.PremiumProtected(billingSvc))
premium.Get("/premium-feature", handlers.PremiumFeature)
```

### Checking subscription status directly

```go
active, err := billingSvc.IsSubscribed(ctx, userID)
```

## Design notes

- **Storage- and framework-agnostic verification** — `AppleProvider`/`GoogleProvider` return a
  `PurchaseResult`; they never touch a database. Only `Service` (persistence) and `Handler`
  (HTTP) depend on GORM/Fiber, and only `Initialize` wires them together — so a project on a
  different framework can still use `NewAppleProvider`/`NewGoogleProvider` directly and persist
  results however it likes.
- **Open/closed** — a new store's provider is a new type implementing `PaymentProvider`
  (`VerifyPurchase`, `ParseWebhook`); `Service` and `Handler` don't change.
- Idempotent by transaction ID: `ProcessPurchaseResult` checks for an existing `Transaction`
  before writing, so replayed webhooks are a no-op.
