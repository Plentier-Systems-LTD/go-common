package chat

import (
	"context"
	"testing"
)

func TestNewRequiresAPIKey(t *testing.T) {
	if _, err := New(context.Background(), Config{}); err != ErrNoAPIKey {
		t.Fatalf("expected ErrNoAPIKey, got %v", err)
	}
}
