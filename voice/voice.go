// Package voice transcribes recorded audio into text via OpenAI's Whisper
// API. Framework- and storage-agnostic: callers pass raw audio bytes and
// get text back.
package voice

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"mime/multipart"
	"net/http"
)

// ErrNoAPIKey is returned by NewWhisperClient when no API key is configured.
var ErrNoAPIKey = errors.New("voice: API key is required")

const (
	defaultModel           = "whisper-1"
	transcriptionsEndpoint = "https://api.openai.com/v1/audio/transcriptions"
)

// Transcriber converts an audio recording into text. Handlers should depend
// on this interface, not on WhisperClient directly, so the provider can be
// swapped (or faked in tests) without touching call sites.
type Transcriber interface {
	Transcribe(ctx context.Context, req TranscribeRequest) (string, error)
}

// TranscribeRequest is a single audio file to transcribe.
type TranscribeRequest struct {
	// Filename is passed through to the provider as a hint for format
	// detection (e.g. "recording.m4a").
	Filename string
	Audio    io.Reader
}

// Config configures a WhisperClient.
type Config struct {
	// APIKey authenticates against the OpenAI API. Required.
	APIKey string
	// Model overrides the default Whisper model.
	Model string
	// HTTPClient overrides the default HTTP client (useful for tests).
	HTTPClient *http.Client
}

// WhisperClient transcribes audio via OpenAI's Whisper API.
type WhisperClient struct {
	apiKey     string
	model      string
	httpClient *http.Client
}

var _ Transcriber = (*WhisperClient)(nil)

// NewWhisperClient builds a WhisperClient. It fails fast if the API key is
// missing so misconfiguration is caught at startup rather than on the first
// request.
func NewWhisperClient(cfg Config) (*WhisperClient, error) {
	if cfg.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	httpClient := cfg.HTTPClient
	if httpClient == nil {
		httpClient = http.DefaultClient
	}

	return &WhisperClient{apiKey: cfg.APIKey, model: model, httpClient: httpClient}, nil
}

// Transcribe uploads the audio to OpenAI and returns the transcribed text.
func (c *WhisperClient) Transcribe(ctx context.Context, req TranscribeRequest) (string, error) {
	var body bytes.Buffer
	writer := multipart.NewWriter(&body)

	part, err := writer.CreateFormFile("file", req.Filename)
	if err != nil {
		return "", fmt.Errorf("voice: creating form file: %w", err)
	}
	if _, err := io.Copy(part, req.Audio); err != nil {
		return "", fmt.Errorf("voice: copying audio: %w", err)
	}
	if err := writer.WriteField("model", c.model); err != nil {
		return "", fmt.Errorf("voice: writing model field: %w", err)
	}
	if err := writer.Close(); err != nil {
		return "", fmt.Errorf("voice: closing multipart writer: %w", err)
	}

	httpReq, err := http.NewRequestWithContext(ctx, http.MethodPost, transcriptionsEndpoint, &body)
	if err != nil {
		return "", fmt.Errorf("voice: building request: %w", err)
	}
	httpReq.Header.Set("Content-Type", writer.FormDataContentType())
	httpReq.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.httpClient.Do(httpReq)
	if err != nil {
		return "", fmt.Errorf("voice: request failed: %w", err)
	}
	defer resp.Body.Close() //nolint:errcheck

	respBody, err := io.ReadAll(resp.Body)
	if err != nil {
		return "", fmt.Errorf("voice: reading response: %w", err)
	}

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("voice: transcription API returned status %d: %s", resp.StatusCode, string(respBody))
	}

	var parsed struct {
		Text string `json:"text"`
	}
	if err := json.Unmarshal(respBody, &parsed); err != nil {
		return "", fmt.Errorf("voice: parsing response: %w", err)
	}

	return parsed.Text, nil
}
