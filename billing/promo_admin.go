package billing

import (
	"context"

	"github.com/Plentier-Systems-LTD/go-common/adminapi"
)

// This file adapts Service's promo code methods to adminapi's wire shapes so a platform's Provider can satisfy adminapi.PromoProvider by delegating.

func toPromoCodeSummary(p PromoCode) adminapi.PromoCodeSummary {
	return adminapi.PromoCodeSummary{
		Code:            p.Code,
		Description:     p.Description,
		PlanID:          p.PlanID,
		DiscountType:    string(p.DiscountType),
		DiscountValue:   p.DiscountValue,
		FreeDays:        p.FreeDays,
		MaxRedemptions:  p.MaxRedemptions,
		RedemptionCount: p.RedemptionCount,
		ExpiresAt:       p.ExpiresAt,
		Active:          p.Active,
		CreatedByEmail:  p.CreatedByEmail,
		CreatedAt:       p.CreatedAt,
	}
}

// ListPromoCodesForAdmin lists every promo code in adminapi's wire shape.
func (s *Service) ListPromoCodesForAdmin(ctx context.Context) ([]adminapi.PromoCodeSummary, error) {
	if s == nil {
		return nil, ErrServiceNotConfigured
	}
	codes, err := s.ListPromoCodes(ctx)
	if err != nil {
		return nil, err
	}
	out := make([]adminapi.PromoCodeSummary, len(codes))
	for i, c := range codes {
		out[i] = toPromoCodeSummary(c)
	}
	return out, nil
}

// CreatePromoCodeForAdmin creates a promo code from an adminapi request.
func (s *Service) CreatePromoCodeForAdmin(ctx context.Context, req adminapi.CreatePromoCodeRequest) (adminapi.PromoCodeSummary, error) {
	if s == nil {
		return adminapi.PromoCodeSummary{}, ErrServiceNotConfigured
	}
	promo, err := s.CreatePromoCode(ctx, CreatePromoCodeParams{
		Code:           req.Code,
		Description:    req.Description,
		PlanID:         req.PlanID,
		DiscountType:   PromoDiscountType(req.DiscountType),
		DiscountValue:  req.DiscountValue,
		FreeDays:       req.FreeDays,
		MaxRedemptions: req.MaxRedemptions,
		ExpiresAt:      req.ExpiresAt,
		CreatedByEmail: req.CreatedByEmail,
	})
	if err != nil {
		return adminapi.PromoCodeSummary{}, err
	}
	return toPromoCodeSummary(*promo), nil
}

// SetPromoCodeActiveForAdmin flips a promo code's Active flag.
func (s *Service) SetPromoCodeActiveForAdmin(ctx context.Context, code string, active bool) (adminapi.PromoCodeSummary, error) {
	if s == nil {
		return adminapi.PromoCodeSummary{}, ErrServiceNotConfigured
	}
	promo, err := s.SetPromoCodeActive(ctx, code, active)
	if err != nil {
		return adminapi.PromoCodeSummary{}, err
	}
	return toPromoCodeSummary(*promo), nil
}

// DeletePromoCodeForAdmin soft-deletes a promo code.
func (s *Service) DeletePromoCodeForAdmin(ctx context.Context, code string) error {
	if s == nil {
		return ErrServiceNotConfigured
	}
	return s.DeletePromoCode(ctx, code)
}
