package campaign

import "context"

// Recipient is one person a Campaign gets sent to.
type Recipient struct {
	Email string
	Name  string
}

// UserFilter is a Campaign's audience, in the shape MatchUsersFunc reads.
type UserFilter struct {
	// NeverActive matches users with no recorded login activity at all.
	NeverActive bool
	// InactiveDays, when > 0, matches users inactive at least this many days (combines with NeverActive via OR).
	InactiveDays int
	// PlanStatuses, when non-empty, restricts to these plan statuses; empty means any plan.
	PlanStatuses []string
}

// MatchUsersFunc resolves a UserFilter to matching recipients — supplied by the integrating platform, which owns its own user table.
type MatchUsersFunc func(ctx context.Context, filter UserFilter) ([]Recipient, error)
