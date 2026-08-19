// Package campaign lets a platform compose a promotional email, target it at filtered users, and send or schedule it.
package campaign

import (
	"strings"
	"time"

	"gorm.io/gorm"
)

// Status is where a Campaign is in its lifecycle: draft -> scheduled -> sending -> sent, or -> failed from sending.
type Status string

const (
	StatusDraft     Status = "draft"
	StatusScheduled Status = "scheduled"
	StatusSending   Status = "sending"
	StatusSent      Status = "sent"
	StatusFailed    Status = "failed"
)

// Campaign is one promotional email, its audience filter, and its schedule.
type Campaign struct {
	gorm.Model

	Name    string `gorm:"not null"`
	Subject string `gorm:"not null"`
	Body    string `gorm:"type:text;not null"`

	Status Status `gorm:"not null;default:'draft';index:idx_campaign_dispatch"`

	ScheduledAt *time.Time `gorm:"index:idx_campaign_dispatch"`
	SentAt      *time.Time

	RecipientCount int    `gorm:"default:0"`
	FailureReason  string `gorm:"type:text"`

	NeverActive  bool   `gorm:"default:false"`
	InactiveDays int    `gorm:"default:0"` // 0 = no recency filter
	PlanStatuses string // comma-separated; "" = any plan

	CreatedByEmail string
}

// Filter converts Campaign's persisted filter columns to a UserFilter.
func (c Campaign) Filter() UserFilter {
	return UserFilter{
		NeverActive:  c.NeverActive,
		InactiveDays: c.InactiveDays,
		PlanStatuses: ParsePlanStatuses(c.PlanStatuses),
	}
}

// CampaignTemplate is reusable starting content for a new Campaign's Subject/Body, copied in at creation time.
type CampaignTemplate struct {
	gorm.Model

	Name    string `gorm:"not null"`
	Subject string `gorm:"not null"`
	Body    string `gorm:"type:text;not null"`

	CreatedByEmail string
}

// ParsePlanStatuses/JoinPlanStatuses convert Campaign.PlanStatuses's comma-separated storage to/from a []string.
func ParsePlanStatuses(s string) []string {
	if strings.TrimSpace(s) == "" {
		return nil
	}
	var out []string
	for _, part := range strings.Split(s, ",") {
		if trimmed := strings.TrimSpace(part); trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

func JoinPlanStatuses(statuses []string) string {
	var cleaned []string
	for _, s := range statuses {
		if trimmed := strings.TrimSpace(s); trimmed != "" {
			cleaned = append(cleaned, trimmed)
		}
	}
	return strings.Join(cleaned, ",")
}
