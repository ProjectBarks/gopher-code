package provider

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"
)

// sseWriter is a helper to build SSE response payloads.
func sseLines(events ...string) string {
	var sb strings.Builder
	for _, e := range events {
		sb.WriteString(e)
		sb.WriteString("\n")
	}
	return sb.String()
}

func sseEvent(eventType, data string) string {
	return fmt.Sprintf("event: %s\ndata: %s\n", eventType, data)
}

// collectResults drains a StreamResult channel and returns all results.
func collectResults(ch <-chan StreamResult) []StreamResult {
	var results []StreamResult
	for r := range ch {
		results = append(results, r)
	}
	return results
}

// textOnlySSE returns a minimal SSE response with just text content.
func textOnlySSE() string {
	return sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_test1","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":25,"output_tokens":0},"stop_reason":null}}`) +
		sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`) +
		sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hello"}}`) +
		sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":" world"}}`) +
		sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`) +
		sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":10}}`) +
		sseEvent("message_stop", `{"type":"message_stop"}`)
}

// toolUseSSE returns a SSE response with text + tool_use content.
func toolUseSSE() string {
	return sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_test2","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":50,"output_tokens":0},"stop_reason":null}}`) +
		sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`) +
		sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Let me check."}}`) +
		sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`) +
		sseEvent("content_block_start", `{"type":"content_block_start","index":1,"content_block":{"type":"tool_use","id":"toolu_01XYZ","name":"Bash","input":{}}}`) +
		sseEvent("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"{\"command\":"}}`) +
		sseEvent("content_block_delta", `{"type":"content_block_delta","index":1,"delta":{"type":"input_json_delta","partial_json":"\"ls -la\"}"}}`) +
		sseEvent("content_block_stop", `{"type":"content_block_stop","index":1}`) +
		sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"tool_use"},"usage":{"output_tokens":30}}`) +
		sseEvent("message_stop", `{"type":"message_stop"}`)
}

func TestAnthropicProvider_Name(t *testing.T) {
	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	if p.Name() != "anthropic" {
		t.Errorf("Name() = %q, want %q", p.Name(), "anthropic")
	}
}

func TestAnthropicProvider_TextOnlyStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, textOnlySSE())
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	results := collectResults(ch)

	// Verify we got the expected events.
	t.Run("has_content_block_start", func(t *testing.T) {
		found := false
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventContentBlockStart {
				found = true
				if r.Event.Content == nil {
					t.Error("ContentBlockStart missing Content")
				} else if r.Event.Content.Type != "text" {
					t.Errorf("ContentBlockStart type=%q, want text", r.Event.Content.Type)
				}
			}
		}
		if !found {
			t.Error("no ContentBlockStart event found")
		}
	})

	t.Run("has_text_deltas", func(t *testing.T) {
		var texts []string
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventTextDelta {
				texts = append(texts, r.Event.Text)
			}
		}
		joined := strings.Join(texts, "")
		if joined != "Hello world" {
			t.Errorf("text deltas = %q, want %q", joined, "Hello world")
		}
	})

	t.Run("has_content_block_stop", func(t *testing.T) {
		found := false
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventContentBlockStop {
				found = true
			}
		}
		if !found {
			t.Error("no ContentBlockStop event found")
		}
	})

	t.Run("has_message_done", func(t *testing.T) {
		found := false
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventMessageDone {
				found = true
				if r.Event.Response == nil {
					t.Fatal("MessageDone missing Response")
				}
				resp := r.Event.Response
				if resp.ID != "msg_test1" {
					t.Errorf("ID = %q, want msg_test1", resp.ID)
				}
				if resp.StopReason == nil || *resp.StopReason != StopReasonEndTurn {
					t.Errorf("StopReason = %v, want end_turn", resp.StopReason)
				}
				if resp.Usage.InputTokens != 25 {
					t.Errorf("InputTokens = %d, want 25", resp.Usage.InputTokens)
				}
				if resp.Usage.OutputTokens != 10 {
					t.Errorf("OutputTokens = %d, want 10", resp.Usage.OutputTokens)
				}
				if len(resp.Content) != 1 {
					t.Fatalf("Content len = %d, want 1", len(resp.Content))
				}
				if resp.Content[0].Type != "text" {
					t.Errorf("Content[0].Type = %q, want text", resp.Content[0].Type)
				}
				if resp.Content[0].Text != "Hello world" {
					t.Errorf("Content[0].Text = %q, want %q", resp.Content[0].Text, "Hello world")
				}
			}
		}
		if !found {
			t.Error("no MessageDone event found")
		}
	})

	t.Run("no_errors", func(t *testing.T) {
		for _, r := range results {
			if r.Err != nil {
				t.Errorf("unexpected error: %v", r.Err)
			}
		}
	})
}

func TestAnthropicProvider_ToolUseStream(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, toolUseSSE())
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "list files"}}},
		},
		Tools: []ToolDefinition{
			{
				Name:        "Bash",
				Description: "Run a bash command",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}},"required":["command"],"additionalProperties":false}`),
			},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	results := collectResults(ch)

	t.Run("has_text_block_start", func(t *testing.T) {
		found := false
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventContentBlockStart && r.Event.Content != nil && r.Event.Content.Type == "text" {
				found = true
			}
		}
		if !found {
			t.Error("no text ContentBlockStart found")
		}
	})

	t.Run("has_tool_use_block_start", func(t *testing.T) {
		found := false
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventContentBlockStart && r.Event.Content != nil && r.Event.Content.Type == "tool_use" {
				found = true
				if r.Event.Content.ID != "toolu_01XYZ" {
					t.Errorf("tool_use ID = %q, want toolu_01XYZ", r.Event.Content.ID)
				}
				if r.Event.Content.Name != "Bash" {
					t.Errorf("tool_use Name = %q, want Bash", r.Event.Content.Name)
				}
			}
		}
		if !found {
			t.Error("no tool_use ContentBlockStart found")
		}
	})

	t.Run("has_input_json_deltas", func(t *testing.T) {
		var parts []string
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventInputJsonDelta {
				parts = append(parts, r.Event.PartialJSON)
			}
		}
		if len(parts) != 2 {
			t.Fatalf("expected 2 json deltas, got %d", len(parts))
		}
		full := strings.Join(parts, "")
		var parsed map[string]interface{}
		if err := json.Unmarshal([]byte(full), &parsed); err != nil {
			t.Fatalf("assembled JSON invalid: %v", err)
		}
		if parsed["command"] != "ls -la" {
			t.Errorf("command = %v, want ls -la", parsed["command"])
		}
	})

	t.Run("message_done_has_tool_use", func(t *testing.T) {
		for _, r := range results {
			if r.Event != nil && r.Event.Type == EventMessageDone {
				resp := r.Event.Response
				if resp == nil {
					t.Fatal("MessageDone missing Response")
				}
				if resp.StopReason == nil || *resp.StopReason != StopReasonToolUse {
					t.Errorf("StopReason = %v, want tool_use", resp.StopReason)
				}
				if len(resp.Content) != 2 {
					t.Fatalf("Content len = %d, want 2", len(resp.Content))
				}
				if resp.Content[0].Type != "text" {
					t.Errorf("Content[0].Type = %q, want text", resp.Content[0].Type)
				}
				if resp.Content[1].Type != "tool_use" {
					t.Errorf("Content[1].Type = %q, want tool_use", resp.Content[1].Type)
				}
				if resp.Content[1].Name != "Bash" {
					t.Errorf("Content[1].Name = %q, want Bash", resp.Content[1].Name)
				}
				if resp.Content[1].ID != "toolu_01XYZ" {
					t.Errorf("Content[1].ID = %q, want toolu_01XYZ", resp.Content[1].ID)
				}

				var input map[string]interface{}
				if err := json.Unmarshal(resp.Content[1].Input, &input); err != nil {
					t.Fatalf("parse tool input: %v", err)
				}
				if input["command"] != "ls -la" {
					t.Errorf("tool input command = %v, want ls -la", input["command"])
				}
				return
			}
		}
		t.Error("no MessageDone event found")
	})

	t.Run("no_errors", func(t *testing.T) {
		for _, r := range results {
			if r.Err != nil {
				t.Errorf("unexpected error: %v", r.Err)
			}
		}
	})
}

func TestAnthropicProvider_ErrorResponse(t *testing.T) {
	tests := []struct {
		name       string
		statusCode int
		body       string
		wantSubstr string
	}{
		{
			name:       "rate_limit_429",
			statusCode: 429,
			body:       `{"type":"error","error":{"type":"rate_limit_error","message":"Rate limit exceeded"}}`,
			wantSubstr: "rate limit",
		},
		{
			name:       "server_error_500",
			statusCode: 500,
			body:       `{"type":"error","error":{"type":"api_error","message":"Internal server error"}}`,
			wantSubstr: "server error",
		},
		{
			name:       "overloaded_529",
			statusCode: 529,
			body:       `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`,
			wantSubstr: "server_overload",
		},
	}

	for _, tc := range tests {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(tc.statusCode)
				fmt.Fprint(w, tc.body)
			}))
			defer server.Close()

			p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
			p.baseURL = server.URL

			req := ModelRequest{
				Model:     "claude-sonnet-4-20250514",
				MaxTokens: 1024,
				Messages: []RequestMessage{
					{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
				},
			}

			_, err := p.Stream(context.Background(), req)
			if err == nil {
				t.Fatal("expected error, got nil")
			}
			if !strings.Contains(strings.ToLower(err.Error()), tc.wantSubstr) {
				t.Errorf("error = %q, want substring %q", err.Error(), tc.wantSubstr)
			}
		})
	}
}

func TestAnthropicProvider_AuthError(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusUnauthorized)
		fmt.Fprint(w, `{"type":"error","error":{"type":"authentication_error","message":"Invalid API key"}}`)
	}))
	defer server.Close()

	p := NewAnthropicProvider("bad-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
	}

	_, err := p.Stream(context.Background(), req)
	if err == nil {
		t.Fatal("expected error, got nil")
	}
	if !strings.Contains(strings.ToLower(err.Error()), "authentication") {
		t.Errorf("error = %q, want substring 'authentication'", err.Error())
	}
}

func TestAnthropicProvider_ContextCancellation(t *testing.T) {
	// Create a server that sends events slowly.
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)

		flusher, ok := w.(http.Flusher)
		if !ok {
			t.Log("ResponseWriter does not support Flusher")
			return
		}

		// Send the first event.
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_cancel","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":0},"stop_reason":null}}`))
		flusher.Flush()

		fmt.Fprint(w, sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
		flusher.Flush()

		// Wait for the request to be cancelled, simulating a slow stream.
		<-r.Context().Done()
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	ctx, cancel := context.WithCancel(context.Background())

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
	}

	ch, err := p.Stream(ctx, req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	// Read at least one event to prove the stream started.
	var gotEvent bool
	timeout := time.After(5 * time.Second)
	for !gotEvent {
		select {
		case r, ok := <-ch:
			if !ok {
				t.Fatal("channel closed before any events")
			}
			if r.Event != nil {
				gotEvent = true
			}
		case <-timeout:
			t.Fatal("timeout waiting for first event")
		}
	}

	// Cancel the context.
	cancel()

	// Drain the channel; it should close.
	drained := false
	drainTimeout := time.After(5 * time.Second)
	for !drained {
		select {
		case _, ok := <-ch:
			if !ok {
				drained = true
			}
		case <-drainTimeout:
			t.Fatal("timeout waiting for channel to close after cancellation")
		}
	}
}

func TestAnthropicProvider_RequestFormat(t *testing.T) {
	var (
		capturedHeaders http.Header
		capturedBody    []byte
		mu              sync.Mutex
	)

	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		mu.Lock()
		capturedHeaders = r.Header.Clone()
		var err error
		capturedBody, err = io.ReadAll(r.Body)
		if err != nil {
			t.Errorf("reading request body: %v", err)
		}
		mu.Unlock()

		// Return a minimal valid SSE response.
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_fmt","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":5,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`))
		fmt.Fprint(w, sseEvent("message_stop", `{"type":"message_stop"}`))
	}))
	defer server.Close()

	p := NewAnthropicProvider("sk-test-123", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	temp := 0.7
	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 4096,
		System:    "You are a helpful assistant.",
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
		Tools: []ToolDefinition{
			{
				Name:        "Bash",
				Description: "Run a bash command",
				InputSchema: json.RawMessage(`{"type":"object","properties":{"command":{"type":"string"}}}`),
			},
		},
		Temperature: &temp,
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}
	// Drain the channel.
	collectResults(ch)

	mu.Lock()
	defer mu.Unlock()

	t.Run("has_api_key_header", func(t *testing.T) {
		if capturedHeaders.Get("X-Api-Key") != "sk-test-123" {
			t.Errorf("x-api-key = %q, want sk-test-123", capturedHeaders.Get("X-Api-Key"))
		}
	})

	t.Run("has_anthropic_version_header", func(t *testing.T) {
		if capturedHeaders.Get("Anthropic-Version") != "2023-06-01" {
			t.Errorf("anthropic-version = %q, want 2023-06-01", capturedHeaders.Get("Anthropic-Version"))
		}
	})

	t.Run("has_content_type_header", func(t *testing.T) {
		ct := capturedHeaders.Get("Content-Type")
		if ct != "application/json" {
			t.Errorf("content-type = %q, want application/json", ct)
		}
	})

	t.Run("request_body_format", func(t *testing.T) {
		var body apiRequest
		if err := json.Unmarshal(capturedBody, &body); err != nil {
			t.Fatalf("unmarshal request body: %v", err)
		}

		if body.Model != "claude-sonnet-4-20250514" {
			t.Errorf("model = %q, want claude-sonnet-4-20250514", body.Model)
		}
		if body.MaxTokens != 4096 {
			t.Errorf("max_tokens = %d, want 4096", body.MaxTokens)
		}
		if body.System != "You are a helpful assistant." {
			t.Errorf("system = %q, want 'You are a helpful assistant.'", body.System)
		}
		if !body.Stream {
			t.Error("stream should be true")
		}
		if body.Temperature == nil || *body.Temperature != 0.7 {
			t.Errorf("temperature = %v, want 0.7", body.Temperature)
		}
		if len(body.Messages) != 1 {
			t.Errorf("messages len = %d, want 1", len(body.Messages))
		}
		if len(body.Tools) != 1 {
			t.Errorf("tools len = %d, want 1", len(body.Tools))
		}
		if body.Tools[0].Name != "Bash" {
			t.Errorf("tools[0].name = %q, want Bash", body.Tools[0].Name)
		}
	})
}

func TestAnthropicProvider_StreamErrorEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_err","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":10,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("error", `{"type":"error","error":{"type":"overloaded_error","message":"Overloaded"}}`))
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	results := collectResults(ch)
	foundErr := false
	for _, r := range results {
		if r.Err != nil {
			foundErr = true
			if !strings.Contains(r.Err.Error(), "Overloaded") {
				t.Errorf("error = %q, want substring 'Overloaded'", r.Err.Error())
			}
		}
	}
	if !foundErr {
		t.Error("expected an error result from stream error event")
	}
}

func TestAnthropicProvider_PingEvent(t *testing.T) {
	server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		w.WriteHeader(http.StatusOK)
		fmt.Fprint(w, sseEvent("message_start", `{"type":"message_start","message":{"id":"msg_ping","role":"assistant","model":"claude-sonnet-4-20250514","usage":{"input_tokens":5,"output_tokens":0},"stop_reason":null}}`))
		fmt.Fprint(w, sseEvent("ping", `{"type":"ping"}`))
		fmt.Fprint(w, sseEvent("content_block_start", `{"type":"content_block_start","index":0,"content_block":{"type":"text","text":""}}`))
		fmt.Fprint(w, sseEvent("content_block_delta", `{"type":"content_block_delta","index":0,"delta":{"type":"text_delta","text":"Hi"}}`))
		fmt.Fprint(w, sseEvent("content_block_stop", `{"type":"content_block_stop","index":0}`))
		fmt.Fprint(w, sseEvent("message_delta", `{"type":"message_delta","delta":{"stop_reason":"end_turn"},"usage":{"output_tokens":1}}`))
		fmt.Fprint(w, sseEvent("message_stop", `{"type":"message_stop"}`))
	}))
	defer server.Close()

	p := NewAnthropicProvider("test-key", "claude-sonnet-4-20250514")
	p.baseURL = server.URL

	req := ModelRequest{
		Model:     "claude-sonnet-4-20250514",
		MaxTokens: 1024,
		Messages: []RequestMessage{
			{Role: "user", Content: []RequestContent{{Type: "text", Text: "Hello"}}},
		},
	}

	ch, err := p.Stream(context.Background(), req)
	if err != nil {
		t.Fatalf("Stream() error: %v", err)
	}

	results := collectResults(ch)

	// Ping events should be silently skipped; verify no errors and we got text.
	for _, r := range results {
		if r.Err != nil {
			t.Errorf("unexpected error: %v", r.Err)
		}
	}

	foundText := false
	for _, r := range results {
		if r.Event != nil && r.Event.Type == EventTextDelta && r.Event.Text == "Hi" {
			foundText = true
		}
	}
	if !foundText {
		t.Error("expected text delta 'Hi' after ping event")
	}
}
