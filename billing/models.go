package billing

import (
	"time"

	"gorm.io/gorm"
)

type SubscriptionStatus string
type BillingPlanType string
type ProviderType string

const (
	StatusActive   SubscriptionStatus = "active"
	StatusCanceled SubscriptionStatus = "canceled"
	StatusExpired  SubscriptionStatus = "expired"
	StatusGrace    SubscriptionStatus = "grace"
)

const (
	SubscriptionType BillingPlanType = "subscription"
	ConsumableType   BillingPlanType = "consumable"
)

const (
	GoogleType ProviderType = "google"
	AppleType  ProviderType = "apple"
)

type BillingPlan struct {
	ID   string          `gorm:"primaryKey"`
	Name string          `gorm:"not null"`
	Type BillingPlanType `gorm:"not null"`

	// PriceUSDCents is this plan's price in US cents, set by whoever
	// configures the plan. Zero means "not priced" — such a plan simply
	// contributes nothing to ActiveRevenueUSDCents, rather than erroring.
	PriceUSDCents int64 `gorm:"default:0"`
}

type Subscription struct {
	gorm.Model
	UserID                string             `gorm:"index;not null" json:"userId"`
	PlanID                string             `gorm:"index;not null"`
	Provider              ProviderType       `gorm:"not null"`
	OriginalTransactionID string             `gorm:"uniqueIndex;not null"`
	Status                SubscriptionStatus `gorm:"not null"`
	ExpiresAt             time.Time          `gorm:"index"`
	AutoRenew             bool               `gorm:"default:true"`
}

type Transaction struct {
	gorm.Model
	SubscriptionID uint   `gorm:"index"`
	TransactionID  string `gorm:"uniqueIndex;not null"`
	Amount         int64  `gorm:"default:0"`
	RawPayload     string `gorm:"type:text"`
	Event          string `gorm:"not null"`
}
