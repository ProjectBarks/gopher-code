package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestWebFetchTool(t *testing.T) {
	tool := &tools.WebFetchTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "WebFetch" {
			t.Errorf("expected 'WebFetch', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("WebFetchTool should be read-only")
		}
	})

	t.Run("input_schema_valid_json", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
		if parsed["type"] != "object" {
			t.Errorf("expected type=object, got %v", parsed["type"])
		}
	})

	t.Run("fetch_html_strips_tags", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><title>Test</title></head><body><h1>Hello</h1><p>World &amp; <b>bold</b></p></body></html>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "extract text"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Hello") {
			t.Errorf("expected 'Hello' in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "World & bold") {
			t.Errorf("expected decoded entities and stripped tags, got %q", out.Content)
		}
		if strings.Contains(out.Content, "<h1>") {
			t.Errorf("HTML tags should be stripped, got %q", out.Content)
		}
	})

	t.Run("fetch_plain_text", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/plain")
			fmt.Fprint(w, "plain text response")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Content != "plain text response" {
			t.Errorf("expected plain text, got %q", out.Content)
		}
	})

	t.Run("truncates_long_content", func(t *testing.T) {
		longText := strings.Repeat("x", 20000)
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			fmt.Fprint(w, longText)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(out.Content, "...[truncated]") {
			t.Errorf("expected truncation suffix, got last 30 chars: %q", out.Content[len(out.Content)-30:])
		}
	})

	t.Run("handles_404", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.WriteHeader(404)
			fmt.Fprint(w, "not found")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for 404")
		}
		if !strings.Contains(out.Content, "404") {
			t.Errorf("expected 404 in error message, got %q", out.Content)
		}
	})

	t.Run("handles_invalid_url", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"url": "not-a-valid-url://???", "prompt": "read"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid URL")
		}
	})

	t.Run("empty_url_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"url": "", "prompt": "read"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty URL")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{bad}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("strips_script_and_style", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<html><head><style>body{color:red}</style></head><body><script>alert("hi")</script><p>visible text</p></body></html>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "alert") {
			t.Errorf("script content should be stripped, got %q", out.Content)
		}
		if strings.Contains(out.Content, "color:red") {
			t.Errorf("style content should be stripped, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "visible text") {
			t.Errorf("visible text should remain, got %q", out.Content)
		}
	})

	t.Run("user_agent_header", func(t *testing.T) {
		var userAgent string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent = r.Header.Get("User-Agent")
			fmt.Fprint(w, "ok")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		tool.Execute(context.Background(), tc, input)
		if userAgent != "gopher-code/0.1" {
			t.Errorf("expected User-Agent 'gopher-code/0.1', got %q", userAgent)
		}
	})
}
