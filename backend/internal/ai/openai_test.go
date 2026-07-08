package ai

import (
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"project-manager/backend/internal/config"
)

func TestNewReturnsNilWhenUnconfigured(t *testing.T) {
	cases := []config.Config{
		{},
		{AIBaseURL: "https://gw.example/v1"},
		{AIBaseURL: "https://gw.example/v1", AIAPIKey: "sk-test"},
	}
	for i, cfg := range cases {
		if client := New(cfg); client != nil {
			t.Fatalf("case %d: expected nil client for incomplete config", i)
		}
	}
}

func TestNewReturnsClientWhenConfigured(t *testing.T) {
	cfg := config.Config{AIBaseURL: "https://gw.example/v1", AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"}
	if client := New(cfg); client == nil {
		t.Fatal("expected non-nil client when fully configured")
	}
}

func TestChatSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.URL.Path != "/chat/completions" {
			t.Errorf("unexpected path %q", r.URL.Path)
		}
		if got := r.Header.Get("Authorization"); got != "Bearer sk-test" {
			t.Errorf("missing/incorrect auth header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload chatCompletionRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if payload.Model != "gpt-4o-mini" || len(payload.Messages) != 2 {
			t.Errorf("unexpected payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"  周报正文  "}}]}`)
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	out, err := client.Chat(context.Background(), []Message{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "context"},
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "周报正文" {
		t.Fatalf("expected trimmed content, got %q", out)
	}
}

func TestChatSendsImageContentParts(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		var payload chatCompletionRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if len(payload.Messages) != 1 {
			t.Fatalf("expected one message, got %+v", payload.Messages)
		}
		parts, ok := payload.Messages[0].Content.([]any)
		if !ok || len(parts) != 2 {
			t.Fatalf("expected text and image content parts, got %#v", payload.Messages[0].Content)
		}
		textPart, _ := parts[0].(map[string]any)
		if textPart["type"] != "text" || textPart["text"] != "描述图片" {
			t.Fatalf("unexpected text part: %#v", textPart)
		}
		imagePart, _ := parts[1].(map[string]any)
		imageURL, _ := imagePart["image_url"].(map[string]any)
		if imagePart["type"] != "image_url" || imageURL["url"] != "data:image/png;base64,abc" {
			t.Fatalf("unexpected image part: %#v", imagePart)
		}
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"图片说明"}}]}`)
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	out, err := client.Chat(context.Background(), []Message{{
		Role:      RoleUser,
		Content:   "描述图片",
		ImageURLs: []string{"data:image/png;base64,abc"},
	}})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "图片说明" {
		t.Fatalf("expected image description, got %q", out)
	}
}

func TestChatStreamSuccess(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if got := r.Header.Get("Accept"); got != "text/event-stream" {
			t.Errorf("missing/incorrect accept header: %q", got)
		}
		body, _ := io.ReadAll(r.Body)
		var payload chatCompletionRequest
		if err := json.Unmarshal(body, &payload); err != nil {
			t.Fatalf("decode request: %v", err)
		}
		if !payload.Stream {
			t.Fatalf("expected stream=true payload: %+v", payload)
		}
		w.Header().Set("Content-Type", "text/event-stream")
		flusher, _ := w.(http.Flusher)
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"周\"}}]}\n\n")
		flusher.Flush()
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"报\"}}]}\n\n")
		flusher.Flush()
		io.WriteString(w, "data: {\"choices\":[{\"delta\":{\"content\":\"正文\"}}]}\n\n")
		io.WriteString(w, "data: [DONE]\n\n")
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	var deltas []string
	out, err := client.ChatStream(context.Background(), []Message{
		{Role: RoleSystem, Content: "system"},
		{Role: RoleUser, Content: "context"},
	}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "周报正文" {
		t.Fatalf("expected streamed content, got %q", out)
	}
	if strings.Join(deltas, "") != "周报正文" || len(deltas) != 3 {
		t.Fatalf("unexpected deltas: %#v", deltas)
	}
}

func TestChatStreamAcceptsNonStreamingJSONResponse(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[{"message":{"content":"  完整正文  "}}]}`)
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	var deltas []string
	out, err := client.ChatStream(context.Background(), []Message{{Role: RoleUser, Content: "x"}}, func(delta string) error {
		deltas = append(deltas, delta)
		return nil
	})
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out != "完整正文" {
		t.Fatalf("expected trimmed content, got %q", out)
	}
	if len(deltas) != 1 || deltas[0] != "完整正文" {
		t.Fatalf("unexpected fallback delta: %#v", deltas)
	}
}

func TestChatGatewayError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		w.WriteHeader(http.StatusTooManyRequests)
		io.WriteString(w, `{"error":{"message":"rate limited"}}`)
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	_, err := client.Chat(context.Background(), []Message{{Role: RoleUser, Content: "x"}})
	if err == nil || !strings.Contains(err.Error(), "rate limited") {
		t.Fatalf("expected gateway error surfaced, got %v", err)
	}
}

func TestChatEmptyCompletion(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		io.WriteString(w, `{"choices":[]}`)
	}))
	defer server.Close()

	client := newOpenAIClient(config.Config{AIBaseURL: server.URL, AIAPIKey: "sk-test", AIModel: "gpt-4o-mini"})
	if _, err := client.Chat(context.Background(), []Message{{Role: RoleUser, Content: "x"}}); err == nil {
		t.Fatal("expected error on empty choices")
	}
}
