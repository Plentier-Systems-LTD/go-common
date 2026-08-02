package billing

import (
	"context"
	"errors"
	"fmt"
	"time"

	"gorm.io/gorm"
)

type Service struct {
	db     *gorm.DB
	apple  PaymentProvider
	google PaymentProvider
}

func NewService(db *gorm.DB, apple, google PaymentProvider) *Service {
	return &Service{db: db, apple: apple, google: google}
}

// VerifyAndProcessPurchase lets a client submit a freshly completed purchase
// (receipt/token) for immediate verification, so the user gets entitlement
// right away instead of waiting on the async webhook.
func (s *Service) VerifyAndProcessPurchase(ctx context.Context, userID string, provider string, req VerifyRequest) (*PurchaseResult, error) {
	var p PaymentProvider
	switch provider {
	case "apple":
		p = s.apple
	case "google":
		p = s.google
	default:
		return nil, fmt.Errorf("billing: unknown provider %q", provider)
	}
	if p == nil {
		return nil, fmt.Errorf("billing: provider %q is not configured", provider)
	}

	req.UserID = userID

	res, err := p.VerifyPurchase(ctx, req)
	if err != nil {
		return nil, fmt.Errorf("billing: failed to verify purchase: %w", err)
	}

	if err := s.ProcessPurchaseResult(ctx, userID, provider, res, "VERIFIED_PURCHASE"); err != nil {
		return nil, fmt.Errorf("billing: failed to process verified purchase: %w", err)
	}

	return res, nil
}

func (s *Service) ProcessPurchaseResult(ctx context.Context, userID string, provider string, res *PurchaseResult, event string) error {
	return s.db.WithContext(ctx).Transaction(func(tx *gorm.DB) error {
		var existing Transaction
		err := tx.Where("transaction_id = ?", res.TransactionID).First(&existing).Error
		if err == nil {
			return nil
		}
		if !errors.Is(err, gorm.ErrRecordNotFound) {
			return err
		}

		var sub Subscription

		// 1. Find or create subscription based on OriginalTransactionID
		err = tx.Where("original_transaction_id = ?", res.OriginalTransactionID).First(&sub).Error
		if errors.Is(err, gorm.ErrRecordNotFound) {
			sub = Subscription{
				UserID:                userID,
				PlanID:                res.ProductID,
				Provider:              ProviderType(provider),
				OriginalTransactionID: res.OriginalTransactionID,
			}
		} else if err != nil {
			return err
		}

		// 2. Update status based on expiration and event type
		if res.IsRefund {
			sub.Status = StatusCanceled
		} else if res.ExpiresAt.After(tx.NowFunc()) {
			sub.Status = StatusActive
		} else {
			sub.Status = StatusExpired
		}
		sub.ExpiresAt = res.ExpiresAt

		if err := tx.Save(&sub).Error; err != nil {
			return err
		}

		// 3. Log historical transaction record
		txLog := Transaction{
			SubscriptionID: sub.ID,
			TransactionID:  res.TransactionID,
			Event:          event,
		}
		return tx.Create(&txLog).Error
	})
}

func (s *Service) FindUserIDByOriginalTransactionID(ctx context.Context, originalTxID string) (string, error) {
	var sub Subscription
	err := s.db.WithContext(ctx).
		Select("user_id").
		Where("original_transaction_id = ?", originalTxID).
		First(&sub).Error

	if err != nil {
		if errors.Is(err, gorm.ErrRecordNotFound) {
			return "", errors.New("no subscription record found for this transaction id")
		}
		return "", err
	}

	return sub.UserID, nil
}

func (s *Service) IsSubscribed(ctx context.Context, userID string) (bool, error) {
	var count int64
	err := s.db.WithContext(ctx).
		Model(&Subscription{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, StatusActive, time.Now()).
		Count(&count).Error

	return count > 0, err
}
