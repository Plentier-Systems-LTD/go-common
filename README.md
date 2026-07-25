# plentier-go-common

Shared Go libraries for Plentier backend services (Fiber-based). Extracted from
[Therapist-api](https://github.com/Plentier-Systems-LTD/Therapist-api) for reuse across projects,
starting with [medbill](https://github.com/Plentier-Systems-LTD/medbill).

## Packages

### `auth`

JWT issuance/verification and a Fiber middleware.

```go
cfg := auth.Config{Secret: os.Getenv("JWT_SECRET")}

pair, err := auth.GenerateTokenPair(cfg, user.ID, user.Email)

app.Get("/profile", auth.RequireAuth(cfg), func(c *fiber.Ctx) error {
    claims := auth.User(c) // *auth.Claims
    return c.JSON(fiber.Map{"id": claims.UserID, "email": claims.Email})
})
```

### `cors`

```go
app.Use(cors.New(cors.Config{AllowOrigins: "*"}))
```

### `billing`

Verifies Apple App Store / Google Play purchase receipts and parses their webhook
notifications. Storage-agnostic — verification returns a `PurchaseResult`; persisting
subscription/transaction state is left to each service's own database.

```go
apple, err := billing.NewAppleProvider(sharedSecret, rootCAPEM)
google, err := billing.NewGoogleProvider(serviceAccountJSON, packageName, pubsubAudience, pubsubServiceAccountEmail)

result, err := apple.VerifyPurchase(ctx, billing.VerifyRequest{
    UserID:      userID,
    ReceiptData: signedTransactionJWS,
})
// result.ExpiresAt, result.IsRefund, result.ProductID, ... -> persist as you see fit
```

## Versioning

Tag releases (`git tag vX.Y.Z`) and consume with `go get github.com/Plentier-Systems-LTD/plentier-go-common@vX.Y.Z`.
Until the first tag, consumers can pin a commit: `go get github.com/Plentier-Systems-LTD/plentier-go-common@<sha>`.
