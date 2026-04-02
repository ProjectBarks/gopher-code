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

// Source: tools/WebFetchTool/utils.ts, tools/WebFetchTool/preapproved.ts

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

	t.Run("fetch_html_to_markdown", func(t *testing.T) {
		// Source: utils.ts:456-458 — HTML content type triggers Turndown
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
		if !strings.Contains(out.Content, "bold") {
			t.Errorf("expected 'bold' in output, got %q", out.Content)
		}
		if strings.Contains(out.Content, "<h1>") {
			t.Errorf("HTML tags should be converted, got %q", out.Content)
		}
		// Should produce Markdown headings
		if !strings.Contains(out.Content, "#") {
			t.Errorf("expected Markdown heading marker, got %q", out.Content)
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
		if !strings.Contains(out.Content, "plain text response") {
			t.Errorf("expected plain text, got %q", out.Content)
		}
	})

	t.Run("truncates_long_content", func(t *testing.T) {
		longText := strings.Repeat("x", 200000)
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
		// Source: utils.ts:128 — MAX_MARKDOWN_LENGTH = 100_000
		if !strings.Contains(out.Content, "[Content truncated due to length...]") {
			t.Errorf("expected truncation message, got last 50 chars: %q", out.Content[len(out.Content)-50:])
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
		// Source: utils/http.ts:56-58
		var userAgent string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			userAgent = r.Header.Get("User-Agent")
			fmt.Fprint(w, "ok")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		tool.Execute(context.Background(), tc, input)
		if userAgent != tools.WebFetchUserAgent {
			t.Errorf("expected User-Agent %q, got %q", tools.WebFetchUserAgent, userAgent)
		}
	})

	t.Run("accept_header", func(t *testing.T) {
		// Source: utils.ts:279
		var accept string
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			accept = r.Header.Get("Accept")
			fmt.Fprint(w, "ok")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		tool.Execute(context.Background(), tc, input)
		if !strings.Contains(accept, "text/markdown") {
			t.Errorf("Accept header should prefer markdown, got %q", accept)
		}
	})

	t.Run("cross_host_redirect_reported", func(t *testing.T) {
		// Source: utils.ts:305-312 — cross-host redirects return info
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Location", "https://evil.example.com/malware")
			w.WriteHeader(302)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("cross-host redirect should not be an error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "redirected") {
			t.Errorf("expected redirect message, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "evil.example.com") {
			t.Errorf("expected redirect URL in message, got %q", out.Content)
		}
	})

	t.Run("same_host_redirect_followed", func(t *testing.T) {
		// Source: utils.ts:297-302 — same-host redirects auto-followed
		var hits int
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			hits++
			if r.URL.Path == "/redirect" {
				w.Header().Set("Location", "/final")
				w.WriteHeader(301)
				return
			}
			fmt.Fprint(w, "final content")
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL+"/redirect"))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "final content") {
			t.Errorf("expected final content after redirect, got %q", out.Content)
		}
		if hits != 2 {
			t.Errorf("expected 2 requests (redirect + final), got %d", hits)
		}
	})

	t.Run("html_to_markdown_headings", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<h1>Title</h1><h2>Subtitle</h2><p>Paragraph text</p>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, _ := tool.Execute(context.Background(), tc, input)
		if !strings.Contains(out.Content, "# Title") {
			t.Errorf("expected '# Title', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "## Subtitle") {
			t.Errorf("expected '## Subtitle', got %q", out.Content)
		}
	})

	t.Run("html_to_markdown_links", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<p>Click <a href="https://example.com">here</a></p>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, _ := tool.Execute(context.Background(), tc, input)
		if !strings.Contains(out.Content, "[here](https://example.com)") {
			t.Errorf("expected markdown link, got %q", out.Content)
		}
	})

	t.Run("html_to_markdown_bold_italic", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<p><strong>bold</strong> and <em>italic</em></p>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, _ := tool.Execute(context.Background(), tc, input)
		if !strings.Contains(out.Content, "**bold**") {
			t.Errorf("expected **bold**, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "*italic*") {
			t.Errorf("expected *italic*, got %q", out.Content)
		}
	})

	t.Run("html_to_markdown_code", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<p>Use <code>fmt.Println</code> function</p>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, _ := tool.Execute(context.Background(), tc, input)
		if !strings.Contains(out.Content, "`fmt.Println`") {
			t.Errorf("expected inline code, got %q", out.Content)
		}
	})

	t.Run("html_to_markdown_list", func(t *testing.T) {
		srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Content-Type", "text/html")
			fmt.Fprint(w, `<ul><li>Item one</li><li>Item two</li></ul>`)
		}))
		defer srv.Close()

		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(fmt.Sprintf(`{"url": %q, "prompt": "read"}`, srv.URL))
		out, _ := tool.Execute(context.Background(), tc, input)
		if !strings.Contains(out.Content, "- Item one") {
			t.Errorf("expected '- Item one', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "- Item two") {
			t.Errorf("expected '- Item two', got %q", out.Content)
		}
	})
}

func TestValidateURL(t *testing.T) {
	// Source: utils.ts:139-169
	t.Run("valid_url", func(t *testing.T) {
		if !tools.ValidateURL("https://example.com/page") {
			t.Error("should accept valid URL")
		}
	})

	t.Run("too_long", func(t *testing.T) {
		// Source: utils.ts:106 — MAX_URL_LENGTH = 2000
		long := "https://example.com/" + strings.Repeat("x", 2000)
		if tools.ValidateURL(long) {
			t.Error("should reject URL > 2000 chars")
		}
	})

	t.Run("with_credentials", func(t *testing.T) {
		// Source: utils.ts:156-157
		if tools.ValidateURL("https://user:pass@example.com") {
			t.Error("should reject URL with credentials")
		}
	})

	t.Run("single_part_hostname", func(t *testing.T) {
		// Source: utils.ts:164-166
		if tools.ValidateURL("https://intranet/page") {
			t.Error("should reject single-part hostname")
		}
	})

	t.Run("empty_invalid", func(t *testing.T) {
		if tools.ValidateURL("") {
			t.Error("should reject empty URL")
		}
	})
}

func TestIsPermittedRedirect(t *testing.T) {
	// Source: utils.ts:212-243
	t.Run("same_host_allowed", func(t *testing.T) {
		if !tools.IsPermittedRedirect("https://example.com/a", "https://example.com/b") {
			t.Error("same host redirect should be allowed")
		}
	})

	t.Run("add_www_allowed", func(t *testing.T) {
		// Source: utils.ts:237 — adding www. is allowed
		if !tools.IsPermittedRedirect("https://example.com", "https://www.example.com") {
			t.Error("adding www. should be allowed")
		}
	})

	t.Run("remove_www_allowed", func(t *testing.T) {
		// Source: utils.ts:238 — removing www. is allowed
		if !tools.IsPermittedRedirect("https://www.example.com", "https://example.com") {
			t.Error("removing www. should be allowed")
		}
	})

	t.Run("cross_host_blocked", func(t *testing.T) {
		// Source: utils.ts:239 — different domain blocked
		if tools.IsPermittedRedirect("https://example.com", "https://evil.com") {
			t.Error("cross-host redirect should be blocked")
		}
	})

	t.Run("protocol_change_blocked", func(t *testing.T) {
		// Source: utils.ts:220-222
		if tools.IsPermittedRedirect("https://example.com", "http://example.com") {
			t.Error("protocol change should be blocked")
		}
	})

	t.Run("port_change_blocked", func(t *testing.T) {
		// Source: utils.ts:224-226
		if tools.IsPermittedRedirect("https://example.com", "https://example.com:8080") {
			t.Error("port change should be blocked")
		}
	})

	t.Run("credentials_in_redirect_blocked", func(t *testing.T) {
		// Source: utils.ts:228-230
		if tools.IsPermittedRedirect("https://example.com", "https://user:pass@example.com/page") {
			t.Error("redirect with credentials should be blocked")
		}
	})

	t.Run("invalid_urls_blocked", func(t *testing.T) {
		if tools.IsPermittedRedirect("not-a-url", "https://example.com") {
			t.Error("invalid original URL should return false")
		}
	})
}

func TestPreapprovedHosts(t *testing.T) {
	// Source: preapproved.ts:14-131

	t.Run("hostname_only", func(t *testing.T) {
		approved := []string{
			"docs.python.org", "go.dev", "pkg.go.dev",
			"developer.mozilla.org", "react.dev", "kubernetes.io",
			"platform.claude.com", "modelcontextprotocol.io",
		}
		for _, h := range approved {
			if !tools.IsPreapprovedHost(h, "/") {
				t.Errorf("%q should be preapproved", h)
			}
		}
	})

	t.Run("not_preapproved", func(t *testing.T) {
		notApproved := []string{
			"example.com", "evil.com", "google.com", "facebook.com",
		}
		for _, h := range notApproved {
			if tools.IsPreapprovedHost(h, "/") {
				t.Errorf("%q should NOT be preapproved", h)
			}
		}
	})

	t.Run("path_scoped_github_anthropics", func(t *testing.T) {
		// Source: preapproved.ts:19 — github.com/anthropics
		if !tools.IsPreapprovedHost("github.com", "/anthropics") {
			t.Error("github.com/anthropics should be preapproved")
		}
		if !tools.IsPreapprovedHost("github.com", "/anthropics/claude-code") {
			t.Error("github.com/anthropics/claude-code should be preapproved")
		}
	})

	t.Run("path_scoped_boundary_enforcement", func(t *testing.T) {
		// Source: preapproved.ts:159-161 — segment boundary check
		if tools.IsPreapprovedHost("github.com", "/anthropics-evil") {
			t.Error("github.com/anthropics-evil should NOT match /anthropics prefix")
		}
	})

	t.Run("path_scoped_vercel_docs", func(t *testing.T) {
		// Source: preapproved.ts:117 — vercel.com/docs
		if !tools.IsPreapprovedHost("vercel.com", "/docs") {
			t.Error("vercel.com/docs should be preapproved")
		}
		if !tools.IsPreapprovedHost("vercel.com", "/docs/some-page") {
			t.Error("vercel.com/docs/some-page should be preapproved")
		}
		if tools.IsPreapprovedHost("vercel.com", "/pricing") {
			t.Error("vercel.com/pricing should NOT be preapproved")
		}
	})

	t.Run("github_root_not_preapproved", func(t *testing.T) {
		// github.com itself is NOT preapproved, only specific paths
		if tools.IsPreapprovedHost("github.com", "/") {
			t.Error("github.com root should NOT be preapproved")
		}
	})
}

func TestWebFetchConstants(t *testing.T) {
	// Source: utils.ts:106-128
	if tools.MaxURLLength != 2000 {
		t.Errorf("MaxURLLength = %d, want 2000", tools.MaxURLLength)
	}
	if tools.MaxHTTPContentLength != 10*1024*1024 {
		t.Errorf("MaxHTTPContentLength = %d, want 10MB", tools.MaxHTTPContentLength)
	}
	if tools.FetchTimeoutMs != 60_000 {
		t.Errorf("FetchTimeoutMs = %d, want 60000", tools.FetchTimeoutMs)
	}
	if tools.MaxRedirects != 10 {
		t.Errorf("MaxRedirects = %d, want 10", tools.MaxRedirects)
	}
	if tools.MaxMarkdownLength != 100_000 {
		t.Errorf("MaxMarkdownLength = %d, want 100000", tools.MaxMarkdownLength)
	}
}
