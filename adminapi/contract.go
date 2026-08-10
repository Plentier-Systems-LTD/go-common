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

	// ChurnedLast30d counts billing.Subscription rows with Status
	// Canceled or Expired whose UpdatedAt falls in the last 30 days.
	// billing.Service always Saves a Subscription on every status
	// transition, so UpdatedAt is a reasonable (not perfectly exact)
	// proxy for "when this subscription stopped being active." Zero for
	// a Provider that doesn't wire up billing.
	ChurnedLast30d int64 `json:"churnedLast30d"`

	// FilesUploadedTotal/AIAnalysesTotal/AIChatMessagesTotal are
	// aggregate, non-PII engagement counts — how much a platform's AI
	// features actually get used, not who used them. Each is 0 for a
	// platform that doesn't have that particular feature (e.g. a platform
	// with no chat has AIChatMessagesTotal always 0); a platform whose
	// single feature is simultaneously an upload and an AI call (like a
	// scan-and-extract flow) may legitimately report the same count for
	// FilesUploadedTotal and AIAnalysesTotal.
	FilesUploadedTotal  int64 `json:"filesUploadedTotal"`
	AIAnalysesTotal     int64 `json:"aiAnalysesTotal"`
	AIChatMessagesTotal int64 `json:"aiChatMessagesTotal"`
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

// TrendPoint is one day's signup/conversion/engagement counts, part of a
// series returned oldest-first. FilesUploaded/AIAnalyses/AIChatMessages
// mirror PlatformStats's same-named *Total fields — see there for what
// each means and when it's legitimately 0.
type TrendPoint struct {
	Date             string `json:"date"` // YYYY-MM-DD, UTC
	NewUsers         int64  `json:"newUsers"`
	NewSubscriptions int64  `json:"newSubscriptions"`
	FilesUploaded    int64  `json:"filesUploaded"`
	AIAnalyses       int64  `json:"aiAnalyses"`
	AIChatMessages   int64  `json:"aiChatMessages"`
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

// UserPatch is a partial update to one user, as requested by the
// dashboard — nil fields are left unchanged. Deliberately excludes plan
// status/expiry: those are derived from a platform's own billing
// subscriptions (Stripe/Apple/Google), so they're never writable through
// this contract.
type UserPatch struct {
	Email         *string `json:"email,omitempty"`
	FullName      *string `json:"fullName,omitempty"`
	EmailVerified *bool   `json:"emailVerified,omitempty"`
}

// MutableProvider is the optional write half of Provider — a platform
// implements it only if it wants the dashboard to be able to edit/delete
// its users, not just list them. Kept separate from Provider (rather than
// adding these methods to it directly) so existing read-only Providers
// keep compiling unchanged; see fiber/adminapi.Mount, which mounts the
// extra routes only when a Provider also satisfies this interface — the
// same optional-interface pattern as http.Flusher.
type MutableProvider interface {
	Provider
	UpdateUser(ctx context.Context, id string, patch UserPatch) (UserSummary, error)
	DeleteUser(ctx context.Context, id string) error
}
