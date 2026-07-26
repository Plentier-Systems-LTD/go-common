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
