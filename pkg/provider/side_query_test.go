package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// TestQueryWithModel_ConfigPropagation verifies that QueryWithModel correctly
// propagates model, system prompt, and user prompt to the underlying provider.
// Source: claude.ts:3300-3348
func TestQueryWithModel_ConfigPropagation(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = readAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_qwm","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
		fmt.Fprint(w, sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"response"}}`))
		fmt.Fprint(w, sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
		fmt.Fprint(w, sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":5}}`))
		fmt.Fprint(w, sseEvent("message_stop", `{"type":"message_stop"}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	provider.SetBaseURL(server.URL)

	result, err := QueryWithModel(context.Background(), provider, QueryWithModelRequest{
		SystemPrompt: []string{"You are helpful.", "Be concise."},
		UserPrompt:   "What is 2+2?",
		Options: QueryOptions{
			Model:       "claude-sonnet-4-20250514",
			QuerySource: QuerySourceREPLMainThread,
		},
	})
	if err != nil {
		t.Fatalf("QueryWithModel error: %v", err)
	}

	// Verify the response was received
	if result.Response == nil {
		t.Fatal("QueryWithModel returned nil response")
	}
	if result.Response.ID != "msg_qwm" {
		t.Errorf("Response.ID = %q, want msg_qwm", result.Response.ID)
	}

	// Verify the request body
	var body apiRequest
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	t.Run("model_propagated", func(t *testing.T) {
		if body.Model != "claude-sonnet-4-20250514" {
			t.Errorf("model = %q, want claude-sonnet-4-20250514", body.Model)
		}
	})

	t.Run("system_prompt_joined", func(t *testing.T) {
		want := "You are helpful.\nBe concise."
		if body.System != want {
			t.Errorf("system = %q, want %q", body.System, want)
		}
	})

	t.Run("user_prompt_propagated", func(t *testing.T) {
		if len(body.Messages) != 1 {
			t.Fatalf("messages len = %d, want 1", len(body.Messages))
		}
		if body.Messages[0].Role != "user" {
			t.Errorf("messages[0].role = %q, want user", body.Messages[0].Role)
		}
		if len(body.Messages[0].Content) != 1 {
			t.Fatalf("messages[0].content len = %d, want 1", len(body.Messages[0].Content))
		}
		if body.Messages[0].Content[0].Text != "What is 2+2?" {
			t.Errorf("messages[0].content[0].text = %q, want 'What is 2+2?'", body.Messages[0].Content[0].Text)
		}
	})

	t.Run("max_tokens_default", func(t *testing.T) {
		if body.MaxTokens != MaxNonStreamingTokens {
			t.Errorf("max_tokens = %d, want %d", body.MaxTokens, MaxNonStreamingTokens)
		}
	})

	t.Run("no_tools", func(t *testing.T) {
		if len(body.Tools) != 0 {
			t.Errorf("tools len = %d, want 0 (queryWithModel does not use tools)", len(body.Tools))
		}
	})

	t.Run("stream_enabled", func(t *testing.T) {
		if !body.Stream {
			t.Error("stream should be true")
		}
	})
}

// TestQueryWithModel_EmptyUserPromptError verifies that empty prompts are rejected.
func TestQueryWithModel_EmptyUserPromptError(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	_, err := QueryWithModel(context.Background(), provider, QueryWithModelRequest{
		UserPrompt: "",
		Options: QueryOptions{
			Model:       "claude-sonnet-4-20250514",
			QuerySource: QuerySourceREPLMainThread,
		},
	})
	if err == nil {
		t.Fatal("expected error for empty user prompt")
	}
	if !strings.Contains(err.Error(), "userPrompt is required") {
		t.Errorf("error = %q, want substring 'userPrompt is required'", err.Error())
	}
}

// TestQueryWithModel_MissingModelError verifies that missing model is rejected.
func TestQueryWithModel_MissingModelError(t *testing.T) {
	provider := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	_, err := QueryWithModel(context.Background(), provider, QueryWithModelRequest{
		UserPrompt: "hello",
		Options: QueryOptions{
			Model: "", // missing
		},
	})
	if err == nil {
		t.Fatal("expected error for missing model")
	}
	if !strings.Contains(err.Error(), "model is required") {
		t.Errorf("error = %q, want substring 'model is required'", err.Error())
	}
}

// TestQueryHaiku_UsesSmallFastModel verifies that QueryHaiku dispatches to
// the small fast model (Haiku), not the default Sonnet/Opus.
// Source: claude.ts:3241-3291
func TestQueryHaiku_UsesSmallFastModel(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = readAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_haiku","role":"assistant","model":"claude-haiku-4-5-20251001","usage":{"input_tokens":5,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
		fmt.Fprint(w, sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"A title"}}`))
		fmt.Fprint(w, sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
		fmt.Fprint(w, sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":3}}`))
		fmt.Fprint(w, sseEvent("message_stop", `{"type":"message_stop"}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", "claude-haiku-4-5-20251001")
	provider.SetBaseURL(server.URL)

	result, err := QueryHaiku(context.Background(), provider, QueryHaikuRequest{
		SystemPrompt: []string{"Generate a title."},
		UserPrompt:   "conversation about Go programming",
		QuerySource:  QuerySourceTitle,
	})
	if err != nil {
		t.Fatalf("QueryHaiku error: %v", err)
	}

	if result.Response == nil {
		t.Fatal("QueryHaiku returned nil response")
	}

	// Verify the model used is the small fast model
	var body apiRequest
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	expectedModel := GetSmallFastModel()
	if body.Model != expectedModel {
		t.Errorf("model = %q, want %q (small fast model)", body.Model, expectedModel)
	}

	// Verify response content
	if len(result.Response.Content) != 1 {
		t.Fatalf("Content len = %d, want 1", len(result.Response.Content))
	}
	if result.Response.Content[0].Text != "A title" {
		t.Errorf("Content[0].Text = %q, want 'A title'", result.Response.Content[0].Text)
	}
}

// TestQueryWithModel_CustomMaxOutputTokens verifies that custom max tokens are
// propagated to the API request.
func TestQueryWithModel_CustomMaxOutputTokens(t *testing.T) {
	var capturedBody []byte

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		var err error
		capturedBody, err = readAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}

		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_mt","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":5,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`))
		fmt.Fprint(w, sseEvent("message_stop", `{"type":"message_stop"}`))
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	provider.SetBaseURL(server.URL)

	_, err := QueryWithModel(context.Background(), provider, QueryWithModelRequest{
		UserPrompt: "hello",
		Options: QueryOptions{
			Model:           "claude-sonnet-4-20250514",
			MaxOutputTokens: 8192,
			QuerySource:     QuerySourceREPLMainThread,
		},
	})
	if err != nil {
		t.Fatalf("QueryWithModel error: %v", err)
	}

	var body apiRequest
	if err := json.Unmarshal(capturedBody, &body); err != nil {
		t.Fatalf("unmarshal request body: %v", err)
	}

	if body.MaxTokens != 8192 {
		t.Errorf("max_tokens = %d, want 8192", body.MaxTokens)
	}
}

// TestQueryWithModel_StreamError verifies that stream errors are propagated.
func TestQueryWithModel_StreamError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusTooManyRequests)
		fmt.Fprint(w, `{"type":"error","error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`)
	}))
	defer server.Close()

	provider := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	provider.SetBaseURL(server.URL)

	_, err := QueryWithModel(context.Background(), provider, QueryWithModelRequest{
		UserPrompt: "hello",
		Options: QueryOptions{
			Model:       "claude-sonnet-4-20250514",
			QuerySource: QuerySourceREPLMainThread,
		},
	})
	if err == nil {
		t.Fatal("expected error for rate-limited request")
	}
}

// readAll reads the entire body — a helper to avoid importing io in test.
func readAll(r interface{ Read([]byte) (int, error) }) ([]byte, error) {
	var buf []byte
	tmp := make([]byte, 4096)
	for {
		n, err := r.Read(tmp)
		if n > 0 {
			buf = append(buf, tmp[:n]...)
		}
		if err != nil {
			if err.Error() == "EOF" {
				break
			}
			return buf, err
		}
	}
	return buf, nil
}
