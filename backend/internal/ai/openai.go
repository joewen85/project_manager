package ai

import (
	"bufio"
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
	Stream   bool                 `json:"stream,omitempty"`
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

type chatCompletionStreamResponse struct {
	Choices []struct {
		Delta struct {
			Content string `json:"content"`
		} `json:"delta"`
		Message struct {
			Content string `json:"content"`
		} `json:"message"`
	} `json:"choices"`
	Error *struct {
		Message string `json:"message"`
	} `json:"error"`
}

func (c *openAIClient) newChatRequest(ctx context.Context, messages []Message, stream bool) (*http.Request, error) {
	payload := chatCompletionRequest{Model: c.model, Stream: stream}
	for _, msg := range messages {
		payload.Messages = append(payload.Messages, chatMessagePayload{
			Role:    string(msg.Role),
			Content: msg.Content,
		})
	}
	body, err := json.Marshal(payload)
	if err != nil {
		return nil, fmt.Errorf("ai: marshal request: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodPost, c.baseURL+"/chat/completions", bytes.NewReader(body))
	if err != nil {
		return nil, fmt.Errorf("ai: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Bearer "+c.apiKey)
	if stream {
		req.Header.Set("Accept", "text/event-stream")
	}
	return req, nil
}

func parseChatCompletionResponse(statusCode int, raw []byte) (string, error) {
	var parsed chatCompletionResponse
	if unmarshalErr := json.Unmarshal(raw, &parsed); unmarshalErr != nil {
		return "", fmt.Errorf("ai: decode response (status %d): %w", statusCode, unmarshalErr)
	}
	if statusCode < 200 || statusCode >= 300 {
		if parsed.Error != nil && parsed.Error.Message != "" {
			return "", fmt.Errorf("ai: gateway error (status %d): %s", statusCode, parsed.Error.Message)
		}
		return "", fmt.Errorf("ai: gateway returned status %d", statusCode)
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

func (c *openAIClient) Chat(ctx context.Context, messages []Message) (string, error) {
	req, err := c.newChatRequest(ctx, messages, false)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: call gateway: %w", err)
	}
	defer resp.Body.Close()

	raw, err := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
	if err != nil {
		return "", fmt.Errorf("ai: read response: %w", err)
	}

	return parseChatCompletionResponse(resp.StatusCode, raw)
}

func (c *openAIClient) ChatStream(ctx context.Context, messages []Message, onDelta func(string) error) (string, error) {
	req, err := c.newChatRequest(ctx, messages, true)
	if err != nil {
		return "", err
	}
	resp, err := c.http.Do(req)
	if err != nil {
		return "", fmt.Errorf("ai: call gateway: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return "", fmt.Errorf("ai: read response: %w", readErr)
		}
		return parseChatCompletionResponse(resp.StatusCode, raw)
	}

	if !strings.Contains(strings.ToLower(resp.Header.Get("Content-Type")), "text/event-stream") {
		raw, readErr := io.ReadAll(io.LimitReader(resp.Body, 1<<20))
		if readErr != nil {
			return "", fmt.Errorf("ai: read response: %w", readErr)
		}
		content, parseErr := parseChatCompletionResponse(resp.StatusCode, raw)
		if parseErr != nil {
			return "", parseErr
		}
		if onDelta != nil {
			if callbackErr := onDelta(content); callbackErr != nil {
				return "", fmt.Errorf("ai: stream callback: %w", callbackErr)
			}
		}
		return content, nil
	}

	return readChatCompletionStream(resp.Body, onDelta)
}

func readChatCompletionStream(body io.Reader, onDelta func(string) error) (string, error) {
	scanner := bufio.NewScanner(body)
	scanner.Buffer(make([]byte, 0, 4096), 1024*1024)

	var output strings.Builder
	dataLines := make([]string, 0, 4)
	flushEvent := func() (bool, error) {
		if len(dataLines) == 0 {
			return false, nil
		}
		data := strings.TrimSpace(strings.Join(dataLines, "\n"))
		dataLines = dataLines[:0]
		if data == "" {
			return false, nil
		}
		if data == "[DONE]" {
			return true, nil
		}

		var parsed chatCompletionStreamResponse
		if err := json.Unmarshal([]byte(data), &parsed); err != nil {
			return false, fmt.Errorf("ai: decode stream response: %w", err)
		}
		if parsed.Error != nil && parsed.Error.Message != "" {
			return false, fmt.Errorf("ai: gateway stream error: %s", parsed.Error.Message)
		}
		for _, choice := range parsed.Choices {
			delta := choice.Delta.Content
			if delta == "" {
				delta = choice.Message.Content
			}
			if delta == "" {
				continue
			}
			output.WriteString(delta)
			if onDelta != nil {
				if err := onDelta(delta); err != nil {
					return false, fmt.Errorf("ai: stream callback: %w", err)
				}
			}
		}
		return false, nil
	}

	for scanner.Scan() {
		line := strings.TrimSuffix(scanner.Text(), "\r")
		if line == "" {
			done, err := flushEvent()
			if err != nil {
				return "", err
			}
			if done {
				break
			}
			continue
		}
		if strings.HasPrefix(line, "data:") {
			dataLines = append(dataLines, strings.TrimSpace(strings.TrimPrefix(line, "data:")))
		}
	}
	if err := scanner.Err(); err != nil {
		return "", fmt.Errorf("ai: read stream response: %w", err)
	}
	if len(dataLines) > 0 {
		if _, err := flushEvent(); err != nil {
			return "", err
		}
	}
	content := strings.TrimSpace(output.String())
	if content == "" {
		return "", fmt.Errorf("ai: gateway returned empty completion")
	}
	return content, nil
}
