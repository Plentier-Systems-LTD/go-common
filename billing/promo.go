package billing

import (
	"context"
	"crypto/rand"
	"errors"
	"fmt"
	"math/big"
	"strings"
	"time"

	"github.com/google/uuid"
	"gorm.io/gorm"
)

var (
	ErrPromoNotFound        = errors.New("billing: promo code not found")
	ErrPromoInactive        = errors.New("billing: promo code is inactive")
	ErrPromoExpired         = errors.New("billing: promo code has expired")
	ErrPromoExhausted       = errors.New("billing: promo code has reached its redemption limit")
	ErrPromoAlreadyRedeemed = errors.New("billing: this user has already redeemed this code")
	ErrPromoWrongPlan       = errors.New("billing: this promo code doesn't apply to that plan")
	ErrPromoNoTarget        = errors.New("billing: this code only discounts an existing subscription, and the user has none on the matching plan")
	ErrPromoInvalidDiscount = errors.New("billing: invalid promo discount")
)

// promoAlphabet excludes visually ambiguous characters (0/O, 1/I/L) so a
// generated code is easy to read back over a phone or a support ticket.
const promoAlphabet = "ABCDEFGHJKLMNPQRSTUVWXYZ23456789"

// GeneratePromoCode returns a random 8-character code from promoAlphabet,
// optionally prefixed (e.g. GeneratePromoCode("LAUNCH") -> "LAUNCH-7K2XQ9AB").
func GeneratePromoCode(prefix string) (string, error) {
	const length = 8
	b := make([]byte, length)
	for i := range b {
		n, err := rand.Int(rand.Reader, big.NewInt(int64(len(promoAlphabet))))
		if err != nil {
			return "", fmt.Errorf("billing: failed to generate promo code: %w", err)
		}
		b[i] = promoAlphabet[n.Int64()]
	}

	code := string(b)
	prefix = strings.ToUpper(strings.TrimSpace(prefix))
	if prefix != "" {
		code = prefix + "-" + code
	}
	return code, nil
}

func normalizePromoCode(code string) string {
	return strings.ToUpper(strings.TrimSpace(code))
}

// CreatePromoCodeParams describes a new PromoCode. Leave Code blank to
// have one generated (retried on the unlikely collision); at least one of
// DiscountType/DiscountValue or FreeDays must be set, or the code would
// redeem to nothing.
type CreatePromoCodeParams struct {
	Code           string
	Description    string
	PlanID         string
	DiscountType   PromoDiscountType
	DiscountValue  int64
	FreeDays       int
	MaxRedemptions int
	ExpiresAt      *time.Time
	CreatedByEmail string
}

func (p CreatePromoCodeParams) validate() error {
	switch p.DiscountType {
	case "":
		if p.DiscountValue != 0 {
			return fmt.Errorf("%w: discountValue set without a discountType", ErrPromoInvalidDiscount)
		}
	case PromoPercent:
		if p.DiscountValue < 1 || p.DiscountValue > 100 {
			return fmt.Errorf("%w: percent discount must be between 1 and 100", ErrPromoInvalidDiscount)
		}
	case PromoFixed:
		if p.DiscountValue <= 0 {
			return fmt.Errorf("%w: fixed discount must be a positive amount of cents", ErrPromoInvalidDiscount)
		}
	default:
		return fmt.Errorf("%w: unknown discount type %q", ErrPromoInvalidDiscount, p.DiscountType)
	}
	if p.FreeDays < 0 {
		return fmt.Errorf("%w: freeDays can't be negative", ErrPromoInvalidDiscount)
	}
	if p.MaxRedemptions < 0 {
		return fmt.Errorf("%w: maxRedemptions can't be negative", ErrPromoInvalidDiscount)
	}
	if p.DiscountType == "" && p.FreeDays == 0 {
		return fmt.Errorf("%w: a promo code needs a discount, free days, or both", ErrPromoInvalidDiscount)
	}
	return nil
}

// CreatePromoCode validates params and inserts a new PromoCode, generating
// a unique Code when params.Code is blank.
func (s *Service) CreatePromoCode(ctx context.Context, params CreatePromoCodeParams) (*PromoCode, error) {
	if err := params.validate(); err != nil {
		return nil, err
	}

	promo := PromoCode{
		Description:    strings.TrimSpace(params.Description),
		PlanID:         strings.TrimSpace(params.PlanID),
		DiscountType:   params.DiscountType,
		DiscountValue:  params.DiscountValue,
		FreeDays:       params.FreeDays,
		MaxRedemptions: params.MaxRedemptions,
		ExpiresAt:      params.ExpiresAt,
		Active:         true,
		CreatedByEmail: params.CreatedByEmail,
	}

	if code := normalizePromoCode(params.Code); code != "" {
		promo.Code = code
		if err := s.db.WithContext(ctx).Create(&promo).Error; err != nil {
			return nil, fmt.Errorf("billing: failed to create promo code: %w", err)
		}
		return &promo, nil
	}

	// Auto-generate, retrying on the unlikely collision (33^8 possible
	// codes, but the unique index is what actually guarantees safety).
	const maxAttempts = 5
	for attempt := 0; attempt < maxAttempts; attempt++ {
		code, err := GeneratePromoCode("")
		if err != nil {
			return nil, err
		}
		promo.Code = code
		err = s.db.WithContext(ctx).Create(&promo).Error
		if err == nil {
			return &promo, nil
		}
		if !isDuplicateKeyErr(err) {
			return nil, fmt.Errorf("billing: failed to create promo code: %w", err)
		}
	}
	return nil, fmt.Errorf("billing: failed to generate a unique promo code after %d attempts", maxAttempts)
}

func isDuplicateKeyErr(err error) bool {
	return errors.Is(err, gorm.ErrDuplicatedKey) || strings.Contains(err.Error(), "duplicate")
}

// ListPromoCodes returns every promo code, most recently created first.
func (s *Service) ListPromoCodes(ctx context.Context) ([]PromoCode, error) {
	var codes []PromoCode
	err := s.db.WithContext(ctx).Order("created_at DESC").Find(&codes).Error
	return codes, err
}

func (s *Service) getPromoCode(ctx context.Context, code string) (*PromoCode, error) {
	var promo PromoCode
	err := s.db.WithContext(ctx).Where("code = ?", normalizePromoCode(code)).First(&promo).Error
	if errors.Is(err, gorm.ErrRecordNotFound) {
		return nil, ErrPromoNotFound
	}
	if err != nil {
		return nil, err
	}
	return &promo, nil
}

// SetPromoCodeActive flips a promo code's Active flag — the reversible
// alternative to DeletePromoCode for "pause this code" without losing its
// redemption history.
func (s *Service) SetPromoCodeActive(ctx context.Context, code string, active bool) (*PromoCode, error) {
	promo, err := s.getPromoCode(ctx, code)
	if err != nil {
		return nil, err
	}
	promo.Active = active
	if err := s.db.WithContext(ctx).Save(promo).Error; err != nil {
		return nil, fmt.Errorf("billing: failed to update promo code: %w", err)
	}
	return promo, nil
}

// DeletePromoCode soft-deletes a promo code (gorm.Model's DeletedAt) so it
// can no longer be redeemed while its PromoRedemption history is kept.
func (s *Service) DeletePromoCode(ctx context.Context, code string) error {
	promo, err := s.getPromoCode(ctx, code)
	if err != nil {
		return err
	}
	return s.db.WithContext(ctx).Delete(promo).Error
}

// PromoRedemptionResult is what redeeming a code actually did.
type PromoRedemptionResult struct {
	Code             string
	PlanID           string
	FreeDaysGranted  int
	DiscountUSDCents int64
	NewExpiresAt     time.Time
}

// RedeemPromoCode applies code to userID: it may extend/create a
// Subscription's ExpiresAt by the code's FreeDays, apply a discount to
// that subscription's contribution to ActiveRevenueUSDCents, or both.
// planID resolves the target plan when the code itself isn't scoped to
// one (PromoCode.PlanID takes precedence). A discount-only code (no
// FreeDays) requires the user to already hold a subscription on the
// resolved plan — there's nothing to discount otherwise
// (ErrPromoNoTarget); a code with FreeDays > 0 will create a comp
// Subscription (Provider "promo") if the user has none yet.
func (s *Service) RedeemPromoCode(ctx context.Context, userID, code, planID string) (*PromoRedemptionResult, error) {
	var result *PromoRedemptionResult

	err := s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var promo PromoCode
		err := tx.Where("code = ?", normalizePromoCode(code)).First(&promo).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return ErrPromoNotFound
		}
		if err != nil {
			return err
		}

		if !promo.Active {
			return ErrPromoInactive
		}
		if promo.ExpiresAt != nil && promo.ExpiresAt.Before(tx.NowFunc()) {
			return ErrPromoExpired
		}
		if promo.MaxRedemptions > 0 && promo.RedemptionCount >= promo.MaxRedemptions {
			return ErrPromoExhausted
		}

		resolvedPlanID := promo.PlanID
		if resolvedPlanID == "" {
			resolvedPlanID = strings.TrimSpace(planID)
		} else if planID != "" && planID != promo.PlanID {
			return ErrPromoWrongPlan
		}

		var sub Subscription
		err = tx.Where("user_id = ? AND plan_id = ?", userID, resolvedPlanID).
			Order("expires_at DESC").First(&sub).Error
		switch {
		case errors.Is(err, gorm.ErrRecordNotFound):
			if promo.FreeDays <= 0 {
				return ErrPromoNoTarget
			}
			if resolvedPlanID == "" {
				return fmt.Errorf("%w: a planID is required to redeem this code", ErrPromoInvalidDiscount)
			}
			sub = Subscription{
				UserID:                userID,
				PlanID:                resolvedPlanID,
				Provider:              "promo",
				OriginalTransactionID: "promo:" + uuid.NewString(),
				Status:                StatusActive,
				ExpiresAt:             tx.NowFunc(),
			}
		case err != nil:
			return err
		}

		if promo.FreeDays > 0 {
			base := sub.ExpiresAt
			if base.Before(tx.NowFunc()) {
				base = tx.NowFunc()
			}
			sub.ExpiresAt = base.AddDate(0, 0, promo.FreeDays)
			sub.Status = StatusActive
		}

		var discountCents int64
		if promo.DiscountType != "" {
			var plan BillingPlan
			if err := tx.First(&plan, "id = ?", resolvedPlanID).Error; err != nil {
				return fmt.Errorf("billing: failed to load plan %q for promo discount: %w", resolvedPlanID, err)
			}
			switch promo.DiscountType {
			case PromoPercent:
				discountCents = plan.PriceUSDCents * promo.DiscountValue / 100
			case PromoFixed:
				discountCents = promo.DiscountValue
			}
			if discountCents > plan.PriceUSDCents {
				discountCents = plan.PriceUSDCents
			}
			sub.PromoDiscountUSDCents = discountCents
		}

		if err := tx.Save(&sub).Error; err != nil {
			return err
		}

		redemption := PromoRedemption{
			PromoCodeID:             promo.ID,
			UserID:                  userID,
			SubscriptionID:          sub.ID,
			AppliedFreeDays:         promo.FreeDays,
			AppliedDiscountUSDCents: discountCents,
		}
		if err := tx.Create(&redemption).Error; err != nil {
			if isDuplicateKeyErr(err) {
				return ErrPromoAlreadyRedeemed
			}
			return err
		}

		promo.RedemptionCount++
		if err := tx.Model(&PromoCode{}).Where("id = ?", promo.ID).
			Update("redemption_count", promo.RedemptionCount).Error; err != nil {
			return err
		}

		result = &PromoRedemptionResult{
			Code:             promo.Code,
			PlanID:           resolvedPlanID,
			FreeDaysGranted:  promo.FreeDays,
			DiscountUSDCents: discountCents,
			NewExpiresAt:     sub.ExpiresAt,
		}
		return nil
	})

	if err != nil {
		return nil, err
	}
	return result, nil
}
