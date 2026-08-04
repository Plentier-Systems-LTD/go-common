// Package entitlement provides a GORM-backed ActiveSubscription lookup on
// top of go-common/billing's Subscription table.
package entitlement

import (
	"context"
	"errors"
	"fmt"

	sharedbilling "github.com/Plentier-Systems-LTD/go-common/billing"
	"gorm.io/gorm"
)

// ActiveSubscription returns userID's current active, unexpired
// subscription, or (nil, nil) if they don't have one. Complements
// Service.IsSubscribed's plain bool with the plan/expiry detail a
// client-facing status endpoint typically needs to show.
func ActiveSubscription(ctx context.Context, db *gorm.DB, userID string) (*sharedbilling.Subscription, error) {
	var sub sharedbilling.Subscription
	err := sharedbilling.ActiveSubscriptionQuery(db.WithContext(ctx), userID).First(&sub).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, nil
	}
	if err != nil {
		return nil, fmt.Errorf("entitlement: active subscription: %w", err)
	}
	return &sub, nil
}
