// Package adminapi defines the read-only contract a platform exposes so a
// central stats dashboard can pull its user counts and user list. It is
// deliberately small: a platform's own domain data never crosses this
// contract, only the common shape the dashboard needs. See fiber/adminapi
// for the Fiber handlers that serve this contract, and gorm/auth,
// go-common/billing for the pieces a Provider implementation typically
// reads from.
package adminapi

import (
	"context"
	"time"
)

// PlatformStats summarizes a platform's users at a point in time.
type PlatformStats struct {
	UserCount       int64     `json:"userCount"`
	NewUsersToday   int64     `json:"newUsersToday"`
	NewUsersLast7d  int64     `json:"newUsersLast7d"`
	NewUsersLast30d int64     `json:"newUsersLast30d"`
	SubscribedCount int64     `json:"subscribedCount"`
	GeneratedAt     time.Time `json:"generatedAt"`

	// SubscriptionRevenueUSDCents is billing.ActiveRevenueUSDCents at
	// GeneratedAt — current recurring revenue, in US cents. Zero for a
	// Provider that doesn't wire up billing.
	SubscriptionRevenueUSDCents int64 `json:"subscriptionRevenueUsdCents"`
}

// UserSummary is one user as reported to the dashboard — not a platform's
// full user record, only what's needed to list and search users and show
// their plan status.
type UserSummary struct {
	ID            string     `json:"id"`
	Email         string     `json:"email"`
	FullName      string     `json:"fullName,omitempty"`
	AvatarURL     *string    `json:"avatarUrl,omitempty"`
	Provider      string     `json:"provider"`
	EmailVerified bool       `json:"emailVerified"`
	CreatedAt     time.Time  `json:"createdAt"`
	PlanStatus    string     `json:"planStatus"` // "free" | "active" | "canceled" | "expired" | "grace"
	PlanExpiresAt *time.Time `json:"planExpiresAt,omitempty"`
	// LastActiveAt is nil until a platform tracks last-login/last-request
	// timestamps — optional today, not every platform has this yet.
	LastActiveAt *time.Time `json:"lastActiveAt,omitempty"`
}

// UserPage is one page of a platform's user list.
type UserPage struct {
	Users      []UserSummary `json:"users"`
	Page       int           `json:"page"`
	Limit      int           `json:"limit"`
	TotalCount int64         `json:"totalCount"`
}

// TrendPoint is one day's signup/conversion counts, part of a series
// returned oldest-first.
type TrendPoint struct {
	Date             string `json:"date"` // YYYY-MM-DD, UTC
	NewUsers         int64  `json:"newUsers"`
	NewSubscriptions int64  `json:"newSubscriptions"`
}

// Provider is what a platform implements to expose itself to the
// dashboard. Deliberately small — a platform's own queries stay in the
// platform; this only adapts them into the wire shape above.
type Provider interface {
	PlatformStats(ctx context.Context) (PlatformStats, error)
	ListUsers(ctx context.Context, page, limit int, search string) (UserPage, error)
	// SignupTrend returns one TrendPoint per day for the last days days
	// (oldest first), for charting signups/conversions over time.
	SignupTrend(ctx context.Context, days int) ([]TrendPoint, error)
}
