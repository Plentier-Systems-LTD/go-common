package billing

import (
	"context"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type AppleProvider struct {
	sharedSecret string
	rootCAPool   *x509.CertPool
}

func NewAppleProvider(sharedSecret string, rootCAPEM []byte) (*AppleProvider, error) {
	if len(rootCAPEM) == 0 {
		return nil, errors.New("apple provider: rootCAPEM must not be empty")
	}

	pool := x509.NewCertPool()
	if ok := pool.AppendCertsFromPEM(rootCAPEM); !ok {
		return nil, errors.New("apple provider: failed to parse any certificates from rootCAPEM")
	}

	return &AppleProvider{sharedSecret: sharedSecret, rootCAPool: pool}, nil
}

type appleWebhookRequest struct {
	SignedPayload string `json:"signedPayload"`
}

type NotificationClaims struct {
	NotificationType string `json:"notificationType"`
	Subtype          string `json:"subtype"`
	NotificationUUID string `json:"notificationUUID"`
	Version          string `json:"version"`
	SignedDate       int64  `json:"signedDate"`
	Data             struct {
		AppAppleID            int64  `json:"appAppleId"`
		BundleID              string `json:"bundleId"`
		BundleVersion         string `json:"bundleVersion"`
		Environment           string `json:"environment"`
		SignedTransactionInfo string `json:"signedTransactionInfo"`
		SignedRenewalInfo     string `json:"signedRenewalInfo"`
	} `json:"data"`
	jwt.RegisteredClaims
}

type TransactionClaims struct {
	TransactionID               string `json:"transactionId"`
	OriginalTransactionID       string `json:"originalTransactionId"`
	WebOrderLineItemID          string `json:"webOrderLineItemId"`
	BundleID                    string `json:"bundleId"`
	ProductID                   string `json:"productId"`
	SubscriptionGroupIdentifier string `json:"subscriptionGroupIdentifier"`
	PurchaseDate                int64  `json:"purchaseDate"`
	OriginalPurchaseDate        int64  `json:"originalPurchaseDate"`
	ExpiresDate                 int64  `json:"expiresDate"`
	Quantity                    int    `json:"quantity"`
	Type                        string `json:"type"`
	InAppOwnershipType          string `json:"inAppOwnershipType"`
	SignedDate                  int64  `json:"signedDate"`
	Environment                 string `json:"environment"`
	AppAccountToken             string `json:"appAccountToken"`
	RevocationDate              int64  `json:"revocationDate"`
	RevocationReason            int    `json:"revocationReason"`
	jwt.RegisteredClaims
}

func extractAndVerifyPublicKey(token *jwt.Token, rootPool *x509.CertPool) (interface{}, error) {
	rawChain, ok := token.Header["x5c"].([]interface{})
	if !ok || len(rawChain) == 0 {
		return nil, errors.New("token header is missing the x5c certificate chain")
	}

	certs := make([]*x509.Certificate, 0, len(rawChain))
	for i, entry := range rawChain {
		encoded, ok := entry.(string)
		if !ok {
			return nil, fmt.Errorf("x5c[%d] is not a string", i)
		}
		der, err := base64.StdEncoding.DecodeString(encoded)
		if err != nil {
			return nil, fmt.Errorf("x5c[%d]: failed to base64-decode certificate: %w", i, err)
		}
		cert, err := x509.ParseCertificate(der)
		if err != nil {
			return nil, fmt.Errorf("x5c[%d]: failed to parse certificate: %w", i, err)
		}
		certs = append(certs, cert)
	}

	leaf := certs[0]
	intermediates := x509.NewCertPool()
	for _, c := range certs[1:] {
		intermediates.AddCert(c)
	}

	if _, err := leaf.Verify(x509.VerifyOptions{
		Roots:         rootPool,
		Intermediates: intermediates,
		CurrentTime:   time.Now(),
		KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageAny},
	}); err != nil {
		return nil, fmt.Errorf("x5c chain does not chain to a trusted Apple root: %w", err)
	}

	return leaf.PublicKey, nil
}

func (a *AppleProvider) verifyAndParseClaims(tokenString string, claims jwt.Claims) error {
	parser := jwt.NewParser(jwt.WithValidMethods([]string{"ES256"}))
	_, err := parser.ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		return extractAndVerifyPublicKey(token, a.rootCAPool)
	})
	return err
}

func (a *AppleProvider) ParseWebhook(ctx context.Context, payload []byte) (*PurchaseResult, string, error) {
	var container appleWebhookRequest
	if err := json.Unmarshal(payload, &container); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal JSON container payload: %w", err)
	}

	if container.SignedPayload == "" {
		return nil, "", errors.New("invalid webhook request: missing structural signedPayload field")
	}

	var notifClaims NotificationClaims
	if err := a.verifyAndParseClaims(container.SignedPayload, &notifClaims); err != nil {
		return nil, "", fmt.Errorf("failed to verify signed notification: %w", err)
	}

	if notifClaims.NotificationType == "TEST" || notifClaims.Data.SignedTransactionInfo == "" {
		testResult := &PurchaseResult{
			TransactionID:         "TEST_" + notifClaims.NotificationUUID,
			OriginalTransactionID: "TEST_" + notifClaims.NotificationUUID,
			ProductID:             "SANDBOX_HEARTBEAT",
			ExpiresAt:             time.Now(),
			IsRefund:              false,
			AccountToken:          "SYSTEM_TEST",
		}
		return testResult, notifClaims.NotificationType, nil
	}

	var txClaims TransactionClaims
	if err := a.verifyAndParseClaims(notifClaims.Data.SignedTransactionInfo, &txClaims); err != nil {
		return nil, "", fmt.Errorf("failed to verify nested transaction: %w", err)
	}

	expiresTime := time.UnixMilli(txClaims.ExpiresDate)
	isRefundedTransaction := txClaims.RevocationDate > 0

	normalizedResult := &PurchaseResult{
		TransactionID:         txClaims.TransactionID,
		OriginalTransactionID: txClaims.OriginalTransactionID,
		ProductID:             txClaims.ProductID,
		ExpiresAt:             expiresTime,
		IsRefund:              isRefundedTransaction,
		AccountToken:          txClaims.AppAccountToken,
	}

	return normalizedResult, notifClaims.NotificationType, nil
}

func (a *AppleProvider) VerifyPurchase(ctx context.Context, req VerifyRequest) (*PurchaseResult, error) {
	if req.ReceiptData == "" {
		return nil, errors.New("missing receipt data: expected a signed StoreKit2 transaction JWS")
	}

	var txClaims TransactionClaims
	if err := a.verifyAndParseClaims(req.ReceiptData, &txClaims); err != nil {
		return nil, fmt.Errorf("failed to verify Apple transaction signature: %w", err)
	}

	return &PurchaseResult{
		TransactionID:         txClaims.TransactionID,
		OriginalTransactionID: txClaims.OriginalTransactionID,
		ProductID:             txClaims.ProductID,
		ExpiresAt:             time.UnixMilli(txClaims.ExpiresDate),
		IsRefund:              txClaims.RevocationDate > 0,
		AccountToken:          txClaims.AppAccountToken,
	}, nil
}
