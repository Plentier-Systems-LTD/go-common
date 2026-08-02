// Package push sends push notifications to a user's registered devices.
// Framework- and storage-agnostic: Sender only knows how to deliver a
// message to a set of device tokens, not how those tokens are persisted
// (see gorm/push for that) or which of a user's notification categories
// they've opted into (that's app-specific policy, not this package's job).
package push

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"time"
)

// Message is the content of a push notification.
type Message struct {
	Title string
	Body  string
	// Sound defaults to "default" when empty.
	Sound string
}

// Sender delivers a push to a set of device tokens. Implementations are
// expected to be best-effort — callers use this fire-and-forget, never in
// the request path a client is waiting on, so an error is safe to log and
// otherwise ignore.
type Sender interface {
	Send(ctx context.Context, tokens []string, msg Message) error
}

const expoPushURL = "https://exp.host/--/api/v2/push/send"

// ExpoSender delivers pushes via Expo's push notification service
// (https://exp.host), for apps built with Expo/React Native.
type ExpoSender struct {
	httpClient *http.Client
}

// NewExpoSender builds an ExpoSender with a 10s request timeout.
func NewExpoSender() *ExpoSender {
	return &ExpoSender{httpClient: &http.Client{Timeout: 10 * time.Second}}
}

type expoPushMessage struct {
	To    string `json:"to"`
	Title string `json:"title"`
	Body  string `json:"body"`
	Sound string `json:"sound"`
}

// Send fires a single batched request to Expo's push API for every token
// given. Delivery failures for individual tokens (uninstalled app, expired
// token, ...) aren't reported here — Expo surfaces those asynchronously
// via its own receipt API, which callers needing delivery confirmation
// should poll separately.
func (s *ExpoSender) Send(ctx context.Context, tokens []string, msg Message) error {
	if len(tokens) == 0 {
		return nil
	}

	sound := msg.Sound
	if sound == "" {
		sound = "default"
	}

	messages := make([]expoPushMessage, len(tokens))
	for i, token := range tokens {
		messages[i] = expoPushMessage{To: token, Title: msg.Title, Body: msg.Body, Sound: sound}
	}

	payload, err := json.Marshal(messages)
	if err != nil {
		return fmt.Errorf("push: encoding payload: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, expoPushURL, bytes.NewReader(payload))
	if err != nil {
		return fmt.Errorf("push: building request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Accept", "application/json")

	resp, err := s.httpClient.Do(req)
	if err != nil {
		return fmt.Errorf("push: sending: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	if resp.StatusCode >= 300 {
		return fmt.Errorf("push: expo push service returned status %d", resp.StatusCode)
	}
	return nil
}
