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
		// Tracks whether this exact transaction event was already logged
		// (e.g. the client's verify call and the async webhook both
		// reporting the same event) — that only skips the duplicate log
		// insert below, not the subscription update, since a repeat
		// transaction_id (a plain restore, or StoreKit returning the
		// existing entitlement instead of a new charge) is also the only
		// way a different app account than the one on file re-verifies
		// the same purchase, and that reassignment must still happen.
		var existing Transaction
		err := tx.Where("transaction_id = ?", res.TransactionID).First(&existing).Error
		alreadyLogged := err == nil
		if err != nil && !errors.Is(err, gorm.ErrRecordNotFound) {
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
		} else {
			// A different app account than the one currently on file just
			// verified this same underlying subscription (e.g. bought or
			// restored it while signed into a different account than
			// whichever one verified it first) — the most recent verifier
			// is who should hold entitlement. Webhook-driven calls always
			// pass the existing owner's ID here already (see
			// handlers.go's FindUserIDByOriginalTransactionID fallback),
			// so this is a no-op reassignment for them.
			sub.UserID = userID
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

		if alreadyLogged {
			return nil
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
	err := ActiveSubscriptionQuery(s.db.WithContext(ctx), userID).Count(&count).Error
	return count > 0, err
}

// ActiveSubscriptionQuery scopes db to userID's active, unexpired
// subscriptions, most-recent-first — the one place "what counts as
// active" is defined, so IsSubscribed and gorm/entitlement.ActiveSubscription
// can't drift out of sync.
func ActiveSubscriptionQuery(db *gorm.DB, userID string) *gorm.DB {
	return db.Model(&Subscription{}).
		Where("user_id = ? AND status = ? AND expires_at > ?", userID, StatusActive, time.Now()).
		Order("expires_at DESC")
}
