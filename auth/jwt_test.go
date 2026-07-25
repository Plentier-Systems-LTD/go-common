package auth

import (
	"testing"
	"time"
)

func TestGenerateAndVerifyTokenPair(t *testing.T) {
	cfg := Config{Secret: "test-secret"}

	pair, err := GenerateTokenPair(cfg, "user-1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	claims, err := VerifyToken(cfg, pair.AccessToken)
	if err != nil {
		t.Fatalf("unexpected error verifying access token: %v", err)
	}
	if claims.UserID != "user-1" || claims.Email != "user@example.com" {
		t.Errorf("unexpected claims: %+v", claims)
	}
}

func TestVerifyTokenRejectsWrongSecret(t *testing.T) {
	pair, err := GenerateTokenPair(Config{Secret: "secret-a"}, "user-1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := VerifyToken(Config{Secret: "secret-b"}, pair.AccessToken); err == nil {
		t.Error("expected verification to fail with the wrong secret")
	}
}

func TestVerifyTokenRejectsExpiredToken(t *testing.T) {
	cfg := Config{Secret: "test-secret", AccessTokenTTL: -time.Minute}

	pair, err := GenerateTokenPair(cfg, "user-1", "user@example.com")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	if _, err := VerifyToken(cfg, pair.AccessToken); err == nil {
		t.Error("expected verification to fail for an expired token")
	}
}
