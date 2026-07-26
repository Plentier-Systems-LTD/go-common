package auth

import (
	"context"
	"crypto/rsa"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"math/big"
	"net/http"
	"sync"
	"time"
)

// jwksCache fetches and caches RSA public keys from a JWK Set endpoint.
// AppleProvider and GoogleProvider each hold one, pointed at their
// respective provider's JWKS URL.
type jwksCache struct {
	url        string
	httpClient *http.Client

	mu     sync.Mutex
	keys   map[string]*rsa.PublicKey
	expiry time.Time
}

func newJWKSCache(url string) *jwksCache {
	return &jwksCache{
		url:        url,
		httpClient: &http.Client{Timeout: 15 * time.Second},
		keys:       map[string]*rsa.PublicKey{},
	}
}

func (c *jwksCache) key(ctx context.Context, kid string) (*rsa.PublicKey, error) {
	c.mu.Lock()
	defer c.mu.Unlock()

	if key, ok := c.keys[kid]; ok && time.Now().Before(c.expiry) {
		return key, nil
	}

	if err := c.refresh(ctx); err != nil {
		return nil, err
	}

	key, ok := c.keys[kid]
	if !ok {
		return nil, fmt.Errorf("auth: no jwks key found for kid %q", kid)
	}
	return key, nil
}

func (c *jwksCache) refresh(ctx context.Context) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.url, nil)
	if err != nil {
		return err
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("auth: failed to fetch jwks: %w", err)
	}
	defer func() { _ = resp.Body.Close() }()

	body, _ := io.ReadAll(resp.Body)
	if resp.StatusCode != http.StatusOK {
		return fmt.Errorf("auth: jwks endpoint returned %d", resp.StatusCode)
	}

	var doc struct {
		Keys []struct {
			Kid string `json:"kid"`
			N   string `json:"n"`
			E   string `json:"e"`
		} `json:"keys"`
	}
	if err := json.Unmarshal(body, &doc); err != nil {
		return fmt.Errorf("auth: failed to decode jwks: %w", err)
	}

	keys := make(map[string]*rsa.PublicKey, len(doc.Keys))
	for _, k := range doc.Keys {
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
		keys[k.Kid] = &rsa.PublicKey{N: new(big.Int).SetBytes(nBytes), E: e}
	}

	c.keys = keys
	c.expiry = time.Now().Add(time.Hour)
	return nil
}
