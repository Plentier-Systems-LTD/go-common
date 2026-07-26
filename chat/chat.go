// Package chat generates conversational AI replies via Google's Gemini API.
// Extracted from Therapist-api's services/gemini package and generalized:
// the persona (system prompt) and conversation history are supplied by the
// caller instead of being hardcoded and read from a database, so any
// service can bring its own assistant persona and storage layer.
package chat

import (
	"context"
	"errors"
	"fmt"

	"google.golang.org/genai"
)

// ErrNoAPIKey is returned by New when no API key is configured.
var ErrNoAPIKey = errors.New("chat: API key is required")

// defaultModel matches Therapist-api's current choice at extraction time.
const defaultModel = "gemini-3.1-flash-lite-preview"

// Role identifies who authored a Message.
type Role string

const (
	RoleUser  Role = "user"
	RoleModel Role = "model"
)

// Message is one turn of conversation history. Callers own persistence —
// this package never reads or writes a database.
type Message struct {
	Role    Role
	Content string
}

// Config configures a Client.
type Config struct {
	// APIKey authenticates against the Gemini API. Required.
	APIKey string
	// Model overrides the default Gemini model.
	Model string
}

// Client generates AI chat replies. Safe for concurrent use.
type Client struct {
	genai *genai.Client
	model string
}

// New builds a Client. It fails fast if the API key is missing so
// misconfiguration is caught at startup rather than on the first request.
func New(ctx context.Context, cfg Config) (*Client, error) {
	if cfg.APIKey == "" {
		return nil, ErrNoAPIKey
	}

	model := cfg.Model
	if model == "" {
		model = defaultModel
	}

	genaiClient, err := genai.NewClient(ctx, &genai.ClientConfig{
		APIKey:  cfg.APIKey,
		Backend: genai.BackendGeminiAPI,
	})
	if err != nil {
		return nil, fmt.Errorf("chat: initializing gemini client: %w", err)
	}

	return &Client{genai: genaiClient, model: model}, nil
}

// GenerateReply generates the assistant's next reply given a system prompt
// (the persona/instructions for this app), prior conversation history
// (oldest first), and the new user message.
func (c *Client) GenerateReply(ctx context.Context, systemPrompt string, history []Message, prompt string) (string, error) {
	contents := make([]*genai.Content, 0, len(history)+1)
	for _, msg := range history {
		contents = append(contents, &genai.Content{
			Role:  string(msg.Role),
			Parts: []*genai.Part{{Text: msg.Content}},
		})
	}
	contents = append(contents, &genai.Content{
		Role:  string(RoleUser),
		Parts: []*genai.Part{{Text: prompt}},
	})

	result, err := c.genai.Models.GenerateContent(
		ctx,
		c.model,
		contents,
		&genai.GenerateContentConfig{
			SystemInstruction: &genai.Content{
				Parts: []*genai.Part{{Text: systemPrompt}},
			},
		},
	)
	if err != nil {
		return "", fmt.Errorf("chat: generating content: %w", err)
	}

	return result.Text(), nil
}
