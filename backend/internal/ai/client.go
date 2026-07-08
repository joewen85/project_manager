// Package ai provides a pluggable client for calling an external LLM gateway.
//
// The client is intentionally minimal: a single Chat call over a list of
// role-tagged messages. Providers implement the Client interface; New selects
// one from configuration. When the gateway is not configured New returns nil,
// and callers are expected to fall back to deterministic behaviour so that no
// project data is ever sent off-box unless explicitly enabled.
package ai

import (
	"context"

	"project-manager/backend/internal/config"
)

// Role identifies the author of a chat message.
type Role string

const (
	// RoleSystem carries instructions and constraints for the model.
	RoleSystem Role = "system"
	// RoleUser carries the user request and any read-only context data.
	RoleUser Role = "user"
	// RoleAssistant carries prior model output (unused today, kept for parity).
	RoleAssistant Role = "assistant"
)

// Message is a single turn in a chat completion request.
type Message struct {
	Role      Role
	Content   string
	ImageURLs []string
}

// Client talks to an LLM gateway. Implementations must be safe for concurrent
// use by multiple goroutines.
type Client interface {
	// Chat sends the messages to the gateway and returns the assistant reply.
	// It returns an error on transport failure, non-2xx responses, or an empty
	// completion so callers can fall back deterministically.
	Chat(ctx context.Context, messages []Message) (string, error)

	// ChatStream sends the messages to the gateway with streaming enabled,
	// calling onDelta for each assistant text delta. It also returns the full
	// assistant reply after the stream completes.
	ChatStream(ctx context.Context, messages []Message, onDelta func(string) error) (string, error)
}

// New builds a Client from configuration. It returns nil when the gateway is
// not fully configured (see config.Config.AIEnabled), signalling callers to
// use their fallback path.
func New(cfg config.Config) Client {
	if !cfg.AIEnabled() {
		return nil
	}
	return newOpenAIClient(cfg)
}
