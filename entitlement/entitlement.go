// Package entitlement helps gate features behind "has an active
// subscription OR hasn't used up a lifetime free-tier allowance yet".
// Framework- and storage-agnostic; see fiber/entitlement for the Fiber
// middleware built on top of it, and gorm/entitlement for a GORM-backed
// ActiveSubscription lookup.
package entitlement

import "context"

// Kind identifies which free-tier allowance a feature spends (e.g.
// "document", "chat"). Each project defines its own Kind values.
type Kind string

// Limits holds a project's lifetime free-tier allowance per Kind.
type Limits map[Kind]int

// Counter reports how much of a Kind's lifetime allowance userID has used.
// Implement one per project, switching on kind to call your own
// per-feature count queries — the counting logic itself is inherently
// app-specific (which table, which columns), so this package only defines
// the shape the middleware needs.
type Counter interface {
	Count(ctx context.Context, kind Kind, userID string) (int64, error)
}
