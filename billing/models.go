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

	// PromoDiscountUSDCents is knocked off the plan's PriceUSDCents when
	// this subscription contributes to ActiveRevenueUSDCents — set by
	// RedeemPromoCode and snapshotted at redemption time, so a later
	// change to the plan's price doesn't retroactively change what a past
	// redemption discounted.
	PromoDiscountUSDCents int64 `gorm:"default:0"`
}

type Transaction struct {
	gorm.Model
	SubscriptionID uint   `gorm:"index"`
	TransactionID  string `gorm:"uniqueIndex;not null"`
	Amount         int64  `gorm:"default:0"`
	RawPayload     string `gorm:"type:text"`
	Event          string `gorm:"not null"`
}

// PromoDiscountType is how a PromoCode's DiscountValue is interpreted.
// The zero value ("") means the code carries no price discount — it may
// still grant FreeDays.
type PromoDiscountType string

const (
	PromoPercent PromoDiscountType = "percent"
	PromoFixed   PromoDiscountType = "fixed"
)

// PromoCode is an admin-issued code redeemable for bonus entitlement
// days, a discounted recurring price, or both — see Service.RedeemPromoCode.
// Code is the human-typed value (e.g. "LAUNCH-7K2XQ9AB", see
// GeneratePromoCode); the internal ID is never exposed over the wire.
type PromoCode struct {
	gorm.Model
	Code        string `gorm:"uniqueIndex;not null"`
	Description string

	// PlanID scopes the code to one billing plan; "" means it applies to
	// any plan the redeeming user names.
	PlanID string `gorm:"index"`

	DiscountType  PromoDiscountType `gorm:"default:''"`
	DiscountValue int64             `gorm:"default:0"`
	FreeDays      int               `gorm:"default:0"`

	// MaxRedemptions caps total redemptions across all users; 0 means
	// unlimited. RedemptionCount is maintained by RedeemPromoCode.
	MaxRedemptions  int `gorm:"default:0"`
	RedemptionCount int `gorm:"default:0"`

	ExpiresAt *time.Time
	Active    bool `gorm:"default:true"`

	CreatedByEmail string
}

// PromoRedemption is one user's use of one PromoCode — the uniqueIndex
// on (PromoCodeID, UserID) is what makes a repeat redemption by the same
// user fail instead of stacking.
type PromoRedemption struct {
	gorm.Model
	PromoCodeID    uint   `gorm:"uniqueIndex:idx_promo_user;not null"`
	UserID         string `gorm:"uniqueIndex:idx_promo_user;not null"`
	SubscriptionID uint   `gorm:"index"`

	AppliedFreeDays         int
	AppliedDiscountUSDCents int64
}
