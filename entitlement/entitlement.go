// Package entitlement helps gate features behind "has an active
// subscription OR hasn't used up a lifetime free-tier allowance yet".
// Framework-agnostic; see fiber/entitlement for the Fiber middleware built
// on top of it.
package entitlement

import (
	"context"
	"errors"
	"fmt"
	"time"

	sharedbilling "github.com/Plentier-Systems-LTD/go-common/billing"
	"gorm.io/gorm"
)

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

// ActiveSubscription returns userID's current active, unexpired
// subscription from go-common/billing's own Subscription table, or
// (nil, nil) if they don't have one. Complements Service.IsSubscribed's
// plain bool with the plan/expiry detail a client-facing status endpoint
// typically needs to show.
func ActiveSubscription(ctx context.Context, db *gorm.DB, userID string) (*sharedbilling.Subscription, error) {
	var sub sharedbilling.Subscription
	err := db.WithContext(ctx).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, sharedbilling.StatusActive, time.Now()).
		Order("expires_at DESC").
		First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entitlement: active subscription: %w", err)
	}
	return &sub, nil
}
