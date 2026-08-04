# push

Sends push notifications to a user's registered devices. Framework- and storage-agnostic —
`Sender` only knows how to deliver a message to a set of device tokens; persisting those tokens is
[`gorm/push`](../gorm/push)'s job, and deciding *whether* a user wants a given notification
(per-category opt-in/opt-out) is app-specific policy that stays in your own code.

```sh
go get github.com/Plentier-Systems-LTD/go-common@latest
```

## What's in here

- **`Sender`** — the interface: `Send(ctx, tokens, Message) error`. Implement it to add a new
  push provider without touching anything else.
- **`ExpoSender`** — the zero-dependency default, for apps built with Expo/React Native. Sends a
  single batched request to Expo's push API.
- **`gorm/push.Store`** — GORM-backed device-token persistence (register/unregister/lookup),
  auto-migrating its own table.

## Example

```go
// internal/push/push.go
var sender = push.NewExpoSender()

func NotifyUser(ctx context.Context, store *gormpush.Store, userID string, title, body string) {
    // Check the user's own notification settings/category opt-in here —
    // this package doesn't know about either.
    if !userWantsNotifications(ctx, userID) {
        return
    }

    tokens, err := store.TokensForUser(ctx, userID)
    if err != nil {
        log.Printf("push: loading tokens for %s: %v", userID, err)
        return
    }

    if err := sender.Send(ctx, tokens, push.Message{Title: title, Body: body}); err != nil {
        log.Printf("push: notify %s: %v", userID, err)
    }
}
```

```go
// registering a device token, e.g. from a mobile client after Expo push permission is granted
store, err := gormpush.NewStore(db) // auto-migrates device_tokens

err = store.Register(ctx, userID, expoPushToken, "ios")
```

## Design notes

- **Fire-and-forget by design** — `Sender.Send` is meant to be called without the caller waiting
  on or propagating its error into the request/response cycle; log it and move on.
- **No per-category logic here** — which notification categories exist and whether a user has
  opted into a given one is app-specific and belongs in your own settings model, not this package.
