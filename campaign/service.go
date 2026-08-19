package campaign

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"gorm.io/gorm"
)

var (
	ErrCampaignNotFound        = errors.New("campaign: campaign not found")
	ErrTemplateNotFound        = errors.New("campaign: template not found")
	ErrInvalidCampaign         = errors.New("campaign: invalid campaign")
	ErrNotEditable             = errors.New("campaign: only draft or scheduled campaigns can be edited")
	ErrSenderNotConfigured     = errors.New("campaign: no email sender configured")
	ErrMatchUsersNotConfigured = errors.New("campaign: no MatchUsersFunc configured")
	ErrServiceNotConfigured    = errors.New("campaign: service not configured")
)

// maxConcurrentSends caps parallel SMTP connections a single SendNow call opens, so a large audience can't blow past the sending IP's rate limits.
const maxConcurrentSends = 8

// Service owns Campaign/CampaignTemplate persistence and sending.
type Service struct {
	db         *gorm.DB
	sender     RawEmailSender
	matchUsers MatchUsersFunc
	onError    func(campaignID uint, err error)
}

func NewService(db *gorm.DB, sender RawEmailSender, matchUsers MatchUsersFunc, onError func(campaignID uint, err error)) *Service {
	return &Service{db: db, sender: sender, matchUsers: matchUsers, onError: onError}
}

// CreateCampaignParams describes a new Campaign; leaving ScheduledAt nil creates a draft.
type CreateCampaignParams struct {
	Name           string
	Subject        string
	Body           string
	NeverActive    bool
	InactiveDays   int
	PlanStatuses   []string
	ScheduledAt    *time.Time
	CreatedByEmail string
}

func (p CreateCampaignParams) validate() error {
	if strings.TrimSpace(p.Name) == "" {
		return fmt.Errorf("%w: name is required", ErrInvalidCampaign)
	}
	if strings.TrimSpace(p.Subject) == "" {
		return fmt.Errorf("%w: subject is required", ErrInvalidCampaign)
	}
	if strings.ContainsAny(p.Subject, "\r\n") {
		return fmt.Errorf("%w: subject can't contain newlines", ErrInvalidCampaign)
	}
	if strings.TrimSpace(p.Body) == "" {
		return fmt.Errorf("%w: body is required", ErrInvalidCampaign)
	}
	if len(p.Name) > 200 {
		return fmt.Errorf("%w: name is too long", ErrInvalidCampaign)
	}
	if len(p.Subject) > 300 {
		return fmt.Errorf("%w: subject is too long", ErrInvalidCampaign)
	}
	if len(p.Body) > 500_000 {
		return fmt.Errorf("%w: body is too long", ErrInvalidCampaign)
	}
	if p.InactiveDays < 0 {
		return fmt.Errorf("%w: inactiveDays can't be negative", ErrInvalidCampaign)
	}
	return nil
}

func (s *Service) CreateCampaign(ctx context.Context, p CreateCampaignParams) (*Campaign, error) {
	if err := p.validate(); err != nil {
		return nil, err
	}

	status := StatusDraft
	if p.ScheduledAt != nil {
		status = StatusScheduled
	}

	c := Campaign{
		Name:           p.Name,
		Subject:        p.Subject,
		Body:           p.Body,
		Status:         status,
		ScheduledAt:    p.ScheduledAt,
		NeverActive:    p.NeverActive,
		InactiveDays:   p.InactiveDays,
		PlanStatuses:   JoinPlanStatuses(p.PlanStatuses),
		CreatedByEmail: p.CreatedByEmail,
	}
	if err := s.db.WithContext(ctx).Create(&c).Error; err != nil {
		return nil, fmt.Errorf("campaign: failed to create campaign: %w", err)
	}
	return &c, nil
}

// ListCampaigns returns every campaign, most recently created first.
func (s *Service) ListCampaigns(ctx context.Context) ([]Campaign, error) {
	var out []Campaign
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (s *Service) getCampaign(ctx context.Context, id uint) (*Campaign, error) {
	var c Campaign
	err := s.db.WithContext(ctx).First(&c, "id = ?", id).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrCampaignNotFound
	}
	if err != nil {
		return nil, err
	}
	return &c, nil
}

// UpdateCampaignParams is a partial edit; ClearSchedule wins over ScheduledAt if both are set.
type UpdateCampaignParams struct {
	Name          *string
	Subject       *string
	Body          *string
	NeverActive   *bool
	InactiveDays  *int
	PlanStatuses  *[]string
	ScheduledAt   *time.Time
	ClearSchedule bool
}

func (s *Service) UpdateCampaign(ctx context.Context, id uint, p UpdateCampaignParams) (*Campaign, error) {
	c, err := s.getCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.Status != StatusDraft && c.Status != StatusScheduled {
		return nil, ErrNotEditable
	}

	if p.Name != nil {
		c.Name = *p.Name
	}
	if p.Subject != nil {
		c.Subject = *p.Subject
	}
	if p.Body != nil {
		c.Body = *p.Body
	}
	if p.NeverActive != nil {
		c.NeverActive = *p.NeverActive
	}
	if p.InactiveDays != nil {
		if *p.InactiveDays < 0 {
			return nil, fmt.Errorf("%w: inactiveDays can't be negative", ErrInvalidCampaign)
		}
		c.InactiveDays = *p.InactiveDays
	}
	if p.PlanStatuses != nil {
		c.PlanStatuses = JoinPlanStatuses(*p.PlanStatuses)
	}

	switch {
	case p.ClearSchedule:
		c.ScheduledAt = nil
		c.Status = StatusDraft
	case p.ScheduledAt != nil:
		c.ScheduledAt = p.ScheduledAt
		c.Status = StatusScheduled
	}

	// Guard the write against a concurrent SendNow/dispatch that already moved the campaign past editable.
	res := s.db.WithContext(ctx).Model(&Campaign{}).
		Where("id = ? AND status IN ?", id, []Status{StatusDraft, StatusScheduled}).
		Updates(map[string]any{
			"name": c.Name, "subject": c.Subject, "body": c.Body,
			"never_active": c.NeverActive, "inactive_days": c.InactiveDays,
			"plan_statuses": c.PlanStatuses, "scheduled_at": c.ScheduledAt, "status": c.Status,
		})
	if res.Error != nil {
		return nil, fmt.Errorf("campaign: failed to update campaign: %w", res.Error)
	}
	if res.RowsAffected == 0 {
		return nil, ErrNotEditable
	}
	return c, nil
}

// DeleteCampaign removes a campaign; sent/sending campaigns can still be deleted.
func (s *Service) DeleteCampaign(ctx context.Context, id uint) error {
	return s.db.WithContext(ctx).Delete(&Campaign{}, "id = ?", id).Error
}

// PreviewAudience reports how many recipients a filter currently matches, without sending anything.
func (s *Service) PreviewAudience(ctx context.Context, filter UserFilter) (int, error) {
	if s.matchUsers == nil {
		return 0, nil
	}
	recipients, err := s.matchUsers(ctx, filter)
	if err != nil {
		return 0, fmt.Errorf("campaign: failed to resolve audience: %w", err)
	}
	return len(recipients), nil
}

// SendNow resolves id's recipients and sends immediately, regardless of any ScheduledAt.
func (s *Service) SendNow(ctx context.Context, id uint) (*Campaign, error) {
	c, err := s.getCampaign(ctx, id)
	if err != nil {
		return nil, err
	}
	if c.Status == StatusSending || c.Status == StatusSent {
		return c, nil
	}
	if s.sender == nil {
		return nil, ErrSenderNotConfigured
	}
	if s.matchUsers == nil {
		return nil, ErrMatchUsersNotConfigured
	}

	// Atomic status flip guards against a concurrent SendNow/dispatch call double-sending.
	res := s.db.WithContext(ctx).Model(&Campaign{}).
		Where("id = ? AND status IN ?", id, []Status{StatusDraft, StatusScheduled}).
		Update("status", StatusSending)
	if res.Error != nil {
		return nil, res.Error
	}
	if res.RowsAffected == 0 {
		return s.getCampaign(ctx, id)
	}
	c.Status = StatusSending

	recipients, err := s.matchUsers(ctx, c.Filter())
	if err != nil {
		s.markFailed(ctx, c, fmt.Errorf("failed to resolve audience: %w", err))
		return c, err
	}

	// Sending itself runs detached from the request's ctx/goroutine so a large audience can't block the caller or the dispatcher.
	go s.sendAndFinalize(context.Background(), c, recipients)
	return c, nil
}

// sendAndFinalize sends to every recipient with bounded concurrency, then persists the final status/counts.
func (s *Service) sendAndFinalize(ctx context.Context, c *Campaign, recipients []Recipient) {
	sem := make(chan struct{}, maxConcurrentSends)
	var wg sync.WaitGroup
	var mu sync.Mutex
	sent := 0
	var lastErr error

	for _, r := range recipients {
		wg.Add(1)
		sem <- struct{}{}
		go func(r Recipient) {
			defer wg.Done()
			defer func() { <-sem }()
			sendErr := s.sender.SendEmail(ctx, r.Email, c.Subject, c.Body, htmlToText(c.Body))
			mu.Lock()
			defer mu.Unlock()
			if sendErr != nil {
				lastErr = sendErr
				return
			}
			sent++
		}(r)
	}
	wg.Wait()

	now := time.Now()
	c.RecipientCount = sent
	c.SentAt = &now
	if lastErr != nil && sent == 0 {
		s.markFailed(ctx, c, fmt.Errorf("failed to send to any of %d recipients: %w", len(recipients), lastErr))
		return
	}
	if lastErr != nil {
		c.FailureReason = fmt.Sprintf("sent to %d of %d recipients; last error: %v", sent, len(recipients), lastErr)
	} else {
		c.FailureReason = ""
	}
	c.Status = StatusSent

	if err := s.db.WithContext(ctx).Save(c).Error; err != nil && s.onError != nil {
		s.onError(c.ID, fmt.Errorf("campaign: failed to save sent campaign: %w", err))
	}
}

func (s *Service) markFailed(ctx context.Context, c *Campaign, err error) {
	c.Status = StatusFailed
	c.FailureReason = err.Error()
	s.db.WithContext(ctx).Model(&Campaign{}).Where("id = ?", c.ID).
		Updates(map[string]any{"status": StatusFailed, "failure_reason": c.FailureReason})
}

// htmlToText is a minimal tag-stripping fallback for the multipart text part.
func htmlToText(html string) string {
	var b strings.Builder
	inTag := false
	for _, r := range html {
		switch {
		case r == '<':
			inTag = true
		case r == '>':
			inTag = false
		case !inTag:
			b.WriteRune(r)
		}
	}
	return b.String()
}

// CreateTemplateParams describes a new CampaignTemplate.
type CreateTemplateParams struct {
	Name           string
	Subject        string
	Body           string
	CreatedByEmail string
}

func (s *Service) CreateTemplate(ctx context.Context, p CreateTemplateParams) (*CampaignTemplate, error) {
	if strings.TrimSpace(p.Name) == "" {
		return nil, fmt.Errorf("%w: name is required", ErrInvalidCampaign)
	}
	if strings.TrimSpace(p.Subject) == "" {
		return nil, fmt.Errorf("%w: subject is required", ErrInvalidCampaign)
	}
	if strings.TrimSpace(p.Body) == "" {
		return nil, fmt.Errorf("%w: body is required", ErrInvalidCampaign)
	}
	t := CampaignTemplate{Name: p.Name, Subject: p.Subject, Body: p.Body, CreatedByEmail: p.CreatedByEmail}
	if err := s.db.WithContext(ctx).Create(&t).Error; err != nil {
		return nil, fmt.Errorf("campaign: failed to create template: %w", err)
	}
	return &t, nil
}

func (s *Service) ListTemplates(ctx context.Context) ([]CampaignTemplate, error) {
	var out []CampaignTemplate
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&out).Error
	return out, err
}

func (s *Service) DeleteTemplate(ctx context.Context, id uint) error {
	res := s.db.WithContext(ctx).Delete(&CampaignTemplate{}, "id = ?", id)
	if res.Error != nil {
		return res.Error
	}
	if res.RowsAffected == 0 {
		return ErrTemplateNotFound
	}
	return nil
}
