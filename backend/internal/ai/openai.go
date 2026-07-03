package ai

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"project-manager/backend/internal/config"
)

// openAIClient talks to any OpenAI-compatible chat-completions endpoint
// (OpenAI, Azure OpenAI gateways, or self-hosted proxies that mirror the API).
type openAIClient struct {
	baseURL string
	apiKey  string
	model   string
	http    *http.Client
}

func newOpenAIClient(cfg config.Config) *openAIClient {
	timeout := time.Duration(cfg.AITimeoutSeconds) * time.Second
	if timeout <= 0 {
		timeout = 30 * time.Second
	}
	return &openAIClient{
		baseURL: cfg.AIBaseURL,
		apiKey:  cfg.AIAPIKey,
		model:   cfg.AIModel,
		http:    &http.Client{Timeout: timeout},
	}
}

type chatCompletionRequest struct {
	Model    string               `json:"model"`
	Messages []chatMessagePayload `json:"messages"`
}

type chatMessagePayload struct {
	Role    string `json:"role"`
	Content string `json:"content"`
}

type chatCompletionResponse struct {
	Choices []struct {
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openAIClient) Chat(ctx context.Context, messages []Message) (string, error) {
	payload := chatCompletionRequest{Model: c.model}
	for _, msg := range messages {
		payload.Messages = append(payload.Messages, chatMessagePayload{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return "", fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return "", fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)

	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: call gateway: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("ai: read response: %w", err)
	}

	var parsed chatCompletionResponse
	if unmarshalErr := json.Unmarshal(raw, &parsed); unmarshalErr != nil {
		return "", fmt.Errorf("ai: decode response (status %d): %w", resp.StatusCode, unmarshalErr)
	}
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("ai: gateway error (status %d): %s", resp.StatusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("ai: gateway returned status %d", resp.StatusCode)
	}
	if len(parsed.Choices) == 0 {
		return "", fmt.Errorf("ai: gateway returned no choices")
	}
	content := strings.TrimSpace(parsed.Choices[0].Message.Content)
	if content == "" {
		return "", fmt.Errorf("ai: gateway returned empty completion")
	}
	return content, nil
}
