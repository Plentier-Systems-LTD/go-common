// Package billing verifies Apple App Store and Google Play purchase
// receipts/tokens and parses their server-to-server webhook notifications.
// It is intentionally storage-agnostic: verification returns a
// PurchaseResult, and persisting subscription/transaction state is left to
// each consuming service's own database.
package billing

import (
	"context"
	"time"
)

type VerifyRequest struct {
	UserID        string
	ReceiptData   string
	PlanId        string
	TransactionID string
}

type PurchaseResult struct {
	TransactionID         string
	OriginalTransactionID string
	ProductID             string
	ExpiresAt             time.Time
	IsRefund              bool
	AccountToken          string
}

type PaymentProvider interface {
	VerifyPurchase(ctx context.Context, req VerifyRequest) (*PurchaseResult, error)
	ParseWebhook(ctx context.Context, payload []byte) (*PurchaseResult, string, error)
}
