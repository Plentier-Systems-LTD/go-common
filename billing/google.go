package billing

import (
	"context"
	"crypto/rsa"
	"crypto/x509"
	"encoding/base64"
	"encoding/json"
	"encoding/pem"
	"errors"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/golang-jwt/jwt/v5"
)

type GoogleProvider struct {
	packageName string
	key         *googleServiceAccountKey
	privateKey  *rsa.PrivateKey
	httpClient  *http.Client

	pubsubAudience           string
	pubsubServiceAccountMail string

	tokenMu     sync.Mutex
	cachedToken string
	tokenExpiry time.Time

	jwksMu     sync.Mutex
	jwksCache  map[string]*rsa.PublicKey
	jwksExpiry time.Time
}

type googleServiceAccountKey struct {
	Type        string `json:"type"`
	ProjectID   string `json:"project_id"`
	PrivateKey  string `json:"private_key"`
	ClientEmail string `json:"client_email"`
	TokenURI    string `json:"token_uri"`
}

func NewGoogleProvider(serviceAccountJSON []byte, packageName, pubsubAudience, pubsubServiceAccountEmail string) (*GoogleProvider, error) {
	if packageName == "" {
		return nil, errors.New("google provider: packageName must not be empty")
	}

	var key googleServiceAccountKey
	if err := json.Unmarshal(serviceAccountJSON, &key); err != nil {
		return nil, fmt.Errorf("google provider: failed to parse service account JSON: %w", err)
	}
	if key.ClientEmail == "" || key.PrivateKey == "" {
		return nil, errors.New("google provider: service account JSON is missing client_email or private_key")
	}
	if key.TokenURI == "" {
		key.TokenURI = "https://oauth2.googleapis.com/token"
	}

	block, _ := pem.Decode([]byte(key.PrivateKey))
	if block == nil {
		return nil, errors.New("google provider: failed to decode PEM block from service account private_key")
	}
	parsed, err := x509.ParsePKCS8PrivateKey(block.Bytes)
	if err != nil {
		return nil, fmt.Errorf("google provider: failed to parse private key: %w", err)
	}
	rsaKey, ok := parsed.(*rsa.PrivateKey)
	if !ok {
		return nil, errors.New("google provider: service account private key is not RSA")
	}

	return &GoogleProvider{
		packageName:              packageName,
		key:                      &key,
		privateKey:               rsaKey,
		httpClient:               &http.Client{Timeout: 15 * time.Second},
		pubsubAudience:           pubsubAudience,
		pubsubServiceAccountMail: pubsubServiceAccountEmail,
		jwksCache:                map[string]*rsa.PublicKey{},
	}, nil
}

func (g *GoogleProvider) accessToken(ctx context.Context) (string, error) {
	g.tokenMu.Lock()
	defer g.tokenMu.Unlock()

	if g.cachedToken != "" && time.Now().Before(g.tokenExpiry) {
		return g.cachedToken, nil
	}

	now := time.Now()
	assertion := jwt.NewWithClaims(jwt.SigningMethodRS256, jwt.MapClaims{
		"iss":   g.key.ClientEmail,
		"scope": "https://www.googleapis.com/auth/androidpublisher",
		"aud":   g.key.TokenURI,
		"iat":   now.Unix(),
		"exp":   now.Add(time.Hour).Unix(),
	})
	signedAssertion, err := assertion.SignedString(g.privateKey)
	if err != nil {
		return "", fmt.Errorf("failed to sign OAuth2 JWT assertion: %w", err)
	}

	form := url.Values{}
	form.Set("grant_type", "urn:ietf:params:oauth:grant-type:jwt-bearer")
	form.Set("assertion", signedAssertion)

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, g.key.TokenURI, strings.NewReader(form.Encode()))
	if err != nil {
		return "", err
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("token exchange request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, string(body))
	}

	var tokenResp struct {
		AccessToken string `json:"access_token"`
		ExpiresIn   int    `json:"expires_in"`
	}
	if err := json.Unmarshal(body, &tokenResp); err != nil {
		return "", fmt.Errorf("failed to decode token response: %w", err)
	}

	g.cachedToken = tokenResp.AccessToken
	g.tokenExpiry = now.Add(time.Duration(tokenResp.ExpiresIn-60) * time.Second)
	return g.cachedToken, nil
}

type subscriptionPurchaseV2 struct {
	SubscriptionState string `json:"subscriptionState"`
	LatestOrderID     string `json:"latestOrderId"`
	LineItems         []struct {
		ProductID  string `json:"productId"`
		ExpiryTime string `json:"expiryTime"`
	} `json:"lineItems"`
	ExternalAccountIdentifiers struct {
		ObfuscatedExternalAccountID string `json:"obfuscatedExternalAccountId"`
	} `json:"externalAccountIdentifiers"`
}

func (g *GoogleProvider) fetchSubscriptionV2(ctx context.Context, purchaseToken string) (*subscriptionPurchaseV2, error) {
	token, err := g.accessToken(ctx)
	if err != nil {
		return nil, fmt.Errorf("failed to obtain access token: %w", err)
	}

	endpoint := fmt.Sprintf(
		"https://androidpublisher.googleapis.com/androidpublisher/v3/applications/%s/purchases/subscriptionsv2/tokens/%s",
		url.PathEscape(g.packageName), url.PathEscape(purchaseToken),
	)

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Authorization", "Bearer "+token)

	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("play developer api request failed: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("play developer api returned %d: %s", resp.StatusCode, string(body))
	}

	var sub subscriptionPurchaseV2
	if err := json.Unmarshal(body, &sub); err != nil {
		return nil, fmt.Errorf("failed to decode subscription purchase: %w", err)
	}
	return &sub, nil
}

func (sub *subscriptionPurchaseV2) toPurchaseResult(fallbackTransactionID string) (*PurchaseResult, error) {
	if len(sub.LineItems) == 0 {
		return nil, errors.New("subscription purchase has no line items")
	}
	item := sub.LineItems[0]

	expiresAt, err := time.Parse(time.RFC3339, item.ExpiryTime)
	if err != nil {
		return nil, fmt.Errorf("failed to parse expiryTime %q: %w", item.ExpiryTime, err)
	}

	txID := sub.LatestOrderID
	if txID == "" {
		txID = fallbackTransactionID
	}

	return &PurchaseResult{
		TransactionID:         txID,
		OriginalTransactionID: txID,
		ProductID:             item.ProductID,
		ExpiresAt:             expiresAt,
		IsRefund:              sub.SubscriptionState == "SUBSCRIPTION_STATE_REVOKED",
		AccountToken:          sub.ExternalAccountIdentifiers.ObfuscatedExternalAccountID,
	}, nil
}

func (g *GoogleProvider) VerifyPurchase(ctx context.Context, req VerifyRequest) (*PurchaseResult, error) {
	if req.ReceiptData == "" {
		return nil, errors.New("missing receipt data: expected a Google Play purchase token")
	}

	sub, err := g.fetchSubscriptionV2(ctx, req.ReceiptData)
	if err != nil {
		return nil, err
	}
	return sub.toPurchaseResult(req.TransactionID)
}

type pubSubPushEnvelope struct {
	Message struct {
		Data      string `json:"data"`
		MessageID string `json:"messageId"`
	} `json:"message"`
	Subscription string `json:"subscription"`
}

type developerNotification struct {
	PackageName              string `json:"packageName"`
	EventTimeMillis          string `json:"eventTimeMillis"`
	SubscriptionNotification *struct {
		Version          string `json:"version"`
		NotificationType int    `json:"notificationType"`
		PurchaseToken    string `json:"purchaseToken"`
		SubscriptionID   string `json:"subscriptionId"`
	} `json:"subscriptionNotification"`
	TestNotification *struct {
		Version string `json:"version"`
	} `json:"testNotification"`
}

var googleNotificationTypeNames = map[int]string{
	1: "SUBSCRIPTION_RECOVERED", 2: "SUBSCRIPTION_RENEWED", 3: "SUBSCRIPTION_CANCELED",
	4: "SUBSCRIPTION_PURCHASED", 5: "SUBSCRIPTION_ON_HOLD", 6: "SUBSCRIPTION_IN_GRACE_PERIOD",
	7: "SUBSCRIPTION_RESTARTED", 8: "SUBSCRIPTION_PRICE_CHANGE_CONFIRMED", 9: "SUBSCRIPTION_DEFERRED",
	10: "SUBSCRIPTION_PAUSED", 11: "SUBSCRIPTION_PAUSE_SCHEDULE_CHANGED", 12: "SUBSCRIPTION_REVOKED",
	13: "SUBSCRIPTION_EXPIRED",
}

func (g *GoogleProvider) ParseWebhook(ctx context.Context, payload []byte) (*PurchaseResult, string, error) {
	if err := g.verifyPubSubRequest(ctx); err != nil {
		return nil, "", fmt.Errorf("pub/sub push authentication failed: %w", err)
	}

	var envelope pubSubPushEnvelope
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal pub/sub push envelope: %w", err)
	}
	if envelope.Message.Data == "" {
		return nil, "", errors.New("pub/sub push envelope missing message.data")
	}

	decoded, err := base64.StdEncoding.DecodeString(envelope.Message.Data)
	if err != nil {
		return nil, "", fmt.Errorf("failed to base64-decode message.data: %w", err)
	}

	var notification developerNotification
	if err := json.Unmarshal(decoded, &notification); err != nil {
		return nil, "", fmt.Errorf("failed to unmarshal developer notification: %w", err)
	}

	if notification.TestNotification != nil {
		testResult := &PurchaseResult{
			TransactionID:         "TEST_" + envelope.Message.MessageID,
			OriginalTransactionID: "TEST_" + envelope.Message.MessageID,
			ProductID:             "SANDBOX_HEARTBEAT",
			ExpiresAt:             time.Now(),
			IsRefund:              false,
			AccountToken:          "SYSTEM_TEST",
		}
		return testResult, "TEST", nil
	}

	if notification.SubscriptionNotification == nil {
		return nil, "", errors.New("developer notification contains no subscriptionNotification (one-time product notifications are not handled)")
	}

	sn := notification.SubscriptionNotification
	eventType, ok := googleNotificationTypeNames[sn.NotificationType]
	if !ok {
		eventType = fmt.Sprintf("UNKNOWN_%d", sn.NotificationType)
	}

	sub, err := g.fetchSubscriptionV2(ctx, sn.PurchaseToken)
	if err != nil {
		return nil, "", err
	}

	result, err := sub.toPurchaseResult(sn.PurchaseToken)
	if err != nil {
		return nil, "", err
	}
	return result, eventType, nil
}

type contextKey string

const googleAuthHeaderKey contextKey = "google-pubsub-authorization"

func WithGoogleAuthHeader(ctx context.Context, authorizationHeader string) context.Context {
	return context.WithValue(ctx, googleAuthHeaderKey, authorizationHeader)
}

func (g *GoogleProvider) verifyPubSubRequest(ctx context.Context) error {
	if g.pubsubAudience == "" || g.pubsubServiceAccountMail == "" {
		return nil
	}

	raw, _ := ctx.Value(googleAuthHeaderKey).(string)
	if raw == "" {
		return errors.New("missing Authorization header on push request")
	}
	tokenString := strings.TrimPrefix(raw, "Bearer ")
	if tokenString == raw {
		return errors.New("authorization header is not a Bearer token")
	}

	claims := jwt.MapClaims{}
	_, err := jwt.NewParser(jwt.WithValidMethods([]string{"RS256"})).ParseWithClaims(tokenString, claims, func(token *jwt.Token) (interface{}, error) {
		kid, _ := token.Header["kid"].(string)
		if kid == "" {
			return nil, errors.New("token header missing kid")
		}
		return g.googlePublicKey(ctx, kid)
	})
	if err != nil {
		return fmt.Errorf("invalid OIDC token: %w", err)
	}

	if iss, _ := claims["iss"].(string); iss != "https://accounts.google.com" {
		return fmt.Errorf("unexpected issuer: %q", iss)
	}
	if aud, _ := claims["aud"].(string); aud != g.pubsubAudience {
		return fmt.Errorf("unexpected audience: %q", aud)
	}
	if email, _ := claims["email"].(string); email != g.pubsubServiceAccountMail {
		return fmt.Errorf("unexpected token subject: %q", email)
	}
	if verified, _ := claims["email_verified"].(bool); !verified {
		return errors.New("token email_verified is false")
	}

	return nil
}

func (g *GoogleProvider) googlePublicKey(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	g.jwksMu.Lock()
	defer g.jwksMu.Unlock()

	if key, ok := g.jwksCache[kid]; ok && time.Now().Before(g.jwksExpiry) {
		return key, nil
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, "https://www.googleapis.com/oauth2/v3/certs", nil)
	if err != nil {
		return nil, err
	}
	resp, err := g.httpClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch google jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("google jwks endpoint returned %d", resp.StatusCode)
	}

	var jwks struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &jwks); err != nil {
		return nil, fmt.Errorf("failed to decode google jwks: %w", err)
	}

	g.jwksCache = map[string]*rsa.PublicKey{}
	for _, k := range jwks.Keys {
		nBytes, err := base64.RawURLEncoding.DecodeString(k.N)
		if err != nil {
			continue
		}
		eBytes, err := base64.RawURLEncoding.DecodeString(k.E)
		if err != nil {
			continue
		}
		e := 0
		for _, b := range eBytes {
			e = e<<8 | int(b)
		}
		g.jwksCache[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
	}
	g.jwksExpiry = time.Now().Add(time.Hour)

	key, ok := g.jwksCache[kid]
	if !ok {
		return nil, fmt.Errorf("no google jwks key found for kid %q", kid)
	}
	return key, nil
}
