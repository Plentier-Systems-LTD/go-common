package campaign

import (
	"context"

	"github.com/Plentier-Systems-LTD/go-common/adminapi"
)

// This file adapts Service's methods to adminapi's wire shapes so a platform's Provider can satisfy adminapi.CampaignProvider by delegating.

func toCampaignSummary(c Campaign) adminapi.CampaignSummary {
	return adminapi.CampaignSummary{
		ID:             c.ID,
		Name:           c.Name,
		Subject:        c.Subject,
		Body:           c.Body,
		Status:         string(c.Status),
		ScheduledAt:    c.ScheduledAt,
		SentAt:         c.SentAt,
		RecipientCount: c.RecipientCount,
		FailureReason:  c.FailureReason,
		NeverActive:    c.NeverActive,
		InactiveDays:   c.InactiveDays,
		PlanStatuses:   ParsePlanStatuses(c.PlanStatuses),
		CreatedByEmail: c.CreatedByEmail,
		CreatedAt:      c.CreatedAt,
	}
}

func toTemplateSummary(t CampaignTemplate) adminapi.TemplateSummary {
	return adminapi.TemplateSummary{
		ID:             t.ID,
		Name:           t.Name,
		Subject:        t.Subject,
		Body:           t.Body,
		CreatedByEmail: t.CreatedByEmail,
		CreatedAt:      t.CreatedAt,
	}
}

// ListCampaignsForAdmin lists every campaign in adminapi's wire shape.
func (s *Service) ListCampaignsForAdmin(ctx context.Context) ([]adminapi.CampaignSummary, error) {
	if s == nil {
		return nil, ErrServiceNotConfigured
	}
	campaigns, err := s.ListCampaigns(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adminapi.CampaignSummary, len(campaigns))
	for i, c := range campaigns {
		out[i] = toCampaignSummary(c)
	}
	return out, nil
}

// CreateCampaignForAdmin creates a campaign from an adminapi request.
func (s *Service) CreateCampaignForAdmin(ctx context.Context, req adminapi.CreateCampaignRequest) (adminapi.CampaignSummary, error) {
	if s == nil {
		return adminapi.CampaignSummary{}, ErrServiceNotConfigured
	}
	c, err := s.CreateCampaign(ctx, CreateCampaignParams{
		Name:           req.Name,
		Subject:        req.Subject,
		Body:           req.Body,
		NeverActive:    req.NeverActive,
		InactiveDays:   req.InactiveDays,
		PlanStatuses:   req.PlanStatuses,
		ScheduledAt:    req.ScheduledAt,
		CreatedByEmail: req.CreatedByEmail,
	})
	if err != nil {
		return adminapi.CampaignSummary{}, err
	}
	return toCampaignSummary(*c), nil
}

// UpdateCampaignForAdmin edits a campaign from an adminapi request.
func (s *Service) UpdateCampaignForAdmin(ctx context.Context, id uint, req adminapi.UpdateCampaignRequest) (adminapi.CampaignSummary, error) {
	if s == nil {
		return adminapi.CampaignSummary{}, ErrServiceNotConfigured
	}
	c, err := s.UpdateCampaign(ctx, id, UpdateCampaignParams{
		Name:          req.Name,
		Subject:       req.Subject,
		Body:          req.Body,
		NeverActive:   req.NeverActive,
		InactiveDays:  req.InactiveDays,
		PlanStatuses:  req.PlanStatuses,
		ScheduledAt:   req.ScheduledAt,
		ClearSchedule: req.ClearSchedule,
	})
	if err != nil {
		return adminapi.CampaignSummary{}, err
	}
	return toCampaignSummary(*c), nil
}

// DeleteCampaignForAdmin deletes a campaign.
func (s *Service) DeleteCampaignForAdmin(ctx context.Context, id uint) error {
	if s == nil {
		return ErrServiceNotConfigured
	}
	return s.DeleteCampaign(ctx, id)
}

// SendCampaignNowForAdmin sends a campaign immediately.
func (s *Service) SendCampaignNowForAdmin(ctx context.Context, id uint) (adminapi.CampaignSummary, error) {
	if s == nil {
		return adminapi.CampaignSummary{}, ErrServiceNotConfigured
	}
	c, err := s.SendNow(ctx, id)
	if err != nil {
		return adminapi.CampaignSummary{}, err
	}
	return toCampaignSummary(*c), nil
}

// PreviewAudienceForAdmin reports how many recipients a would-be campaign's filter currently matches.
func (s *Service) PreviewAudienceForAdmin(ctx context.Context, req adminapi.CreateCampaignRequest) (adminapi.AudiencePreview, error) {
	if s == nil {
		return adminapi.AudiencePreview{}, ErrServiceNotConfigured
	}
	count, err := s.PreviewAudience(ctx, UserFilter{
		NeverActive:  req.NeverActive,
		InactiveDays: req.InactiveDays,
		PlanStatuses: req.PlanStatuses,
	})
	if err != nil {
		return adminapi.AudiencePreview{}, err
	}
	return adminapi.AudiencePreview{Count: count}, nil
}

// ListTemplatesForAdmin lists every reusable template in adminapi's wire shape.
func (s *Service) ListTemplatesForAdmin(ctx context.Context) ([]adminapi.TemplateSummary, error) {
	if s == nil {
		return nil, ErrServiceNotConfigured
	}
	templates, err := s.ListTemplates(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adminapi.TemplateSummary, len(templates))
	for i, t := range templates {
		out[i] = toTemplateSummary(t)
	}
	return out, nil
}

// CreateTemplateForAdmin creates a reusable template from an adminapi request.
func (s *Service) CreateTemplateForAdmin(ctx context.Context, req adminapi.CreateTemplateRequest) (adminapi.TemplateSummary, error) {
	if s == nil {
		return adminapi.TemplateSummary{}, ErrServiceNotConfigured
	}
	t, err := s.CreateTemplate(ctx, CreateTemplateParams{
		Name:           req.Name,
		Subject:        req.Subject,
		Body:           req.Body,
		CreatedByEmail: req.CreatedByEmail,
	})
	if err != nil {
		return adminapi.TemplateSummary{}, err
	}
	return toTemplateSummary(*t), nil
}

// DeleteTemplateForAdmin deletes a reusable template.
func (s *Service) DeleteTemplateForAdmin(ctx context.Context, id uint) error {
	if s == nil {
		return ErrServiceNotConfigured
	}
	return s.DeleteTemplate(ctx, id)
}
