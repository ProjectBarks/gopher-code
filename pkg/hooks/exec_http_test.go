package hooks

import (
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"
)

// Source: utils/hooks/execHttpHook.ts

func TestDefaultHTTPHookTimeoutMs(t *testing.T) {
	// Source: execHttpHook.ts:12
	if DefaultHTTPHookTimeoutMs != 10*60*1000 {
		t.Errorf("expected 600000, got %d", DefaultHTTPHookTimeoutMs)
	}
}

func TestExecHTTPHook_BasicPOST(t *testing.T) {
	// Source: execHttpHook.ts:201-217 — POST JSON to URL
	var receivedBody string
	var receivedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := readAll(r.Body)
		receivedBody = body
		w.WriteHeader(200)
		fmt.Fprint(w, `{"continue":true}`)
	}))
	defer srv.Close()

	hook := HookCommand{
		Type: HookCommandTypeHTTP,
		URL:  srv.URL,
	}
	result := ExecHTTPHook(context.Background(), hook, `{"hook_event_name":"PreToolUse","tool_name":"Bash"}`)

	if !result.OK {
		t.Errorf("expected OK, got error: %s", result.Error)
	}
	if result.StatusCode != 200 {
		t.Errorf("status = %d, want 200", result.StatusCode)
	}
	if receivedContentType != "application/json" {
		t.Errorf("Content-Type = %q, want application/json", receivedContentType)
	}
	if !strings.Contains(receivedBody, "PreToolUse") {
		t.Errorf("body should contain hook input, got %q", receivedBody)
	}
	if !strings.Contains(result.Body, "continue") {
		t.Errorf("response should contain JSON output, got %q", result.Body)
	}
}

func TestExecHTTPHook_CustomHeaders(t *testing.T) {
	// Source: execHttpHook.ts:158-172 — headers with env var interpolation
	os.Setenv("MY_TOKEN", "secret123")
	defer os.Unsetenv("MY_TOKEN")

	var authHeader string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		authHeader = r.Header.Get("Authorization")
		w.WriteHeader(200)
		fmt.Fprint(w, `{}`)
	}))
	defer srv.Close()

	hook := HookCommand{
		Type:           HookCommandTypeHTTP,
		URL:            srv.URL,
		Headers:        map[string]string{"Authorization": "Bearer $MY_TOKEN"},
		AllowedEnvVars: []string{"MY_TOKEN"},
	}
	result := ExecHTTPHook(context.Background(), hook, `{}`)

	if !result.OK {
		t.Errorf("expected OK, got error: %s", result.Error)
	}
	if authHeader != "Bearer secret123" {
		t.Errorf("Authorization = %q, want 'Bearer secret123'", authHeader)
	}
}

func TestExecHTTPHook_ServerError(t *testing.T) {
	// Source: execHttpHook.ts:226-230 — non-2xx is !ok
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(500)
		fmt.Fprint(w, "internal error")
	}))
	defer srv.Close()

	hook := HookCommand{Type: HookCommandTypeHTTP, URL: srv.URL}
	result := ExecHTTPHook(context.Background(), hook, `{}`)

	if result.OK {
		t.Error("expected !OK for 500")
	}
	if result.StatusCode != 500 {
		t.Errorf("status = %d, want 500", result.StatusCode)
	}
	if result.Body != "internal error" {
		t.Errorf("body = %q", result.Body)
	}
}

func TestExecHTTPHook_EmptyURL(t *testing.T) {
	hook := HookCommand{Type: HookCommandTypeHTTP, URL: ""}
	result := ExecHTTPHook(context.Background(), hook, `{}`)
	if result.OK {
		t.Error("expected error for empty URL")
	}
	if result.Error == "" {
		t.Error("expected error message")
	}
}

func TestExecHTTPHook_Timeout(t *testing.T) {
	// Source: execHttpHook.ts:147-149 — custom timeout
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	hook := HookCommand{
		Type:    HookCommandTypeHTTP,
		URL:     srv.URL,
		Timeout: 1, // 1 second
	}

	start := time.Now()
	result := ExecHTTPHook(context.Background(), hook, `{}`)
	elapsed := time.Since(start)

	if result.OK {
		t.Error("expected failure due to timeout")
	}
	if !result.Aborted && result.Error == "" {
		t.Error("expected aborted or error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("should timeout in ~1s, took %v", elapsed)
	}
}

func TestExecHTTPHook_ContextCancellation(t *testing.T) {
	// Source: execHttpHook.ts:234-235
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		time.Sleep(5 * time.Second)
		w.WriteHeader(200)
	}))
	defer srv.Close()

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // cancel immediately

	hook := HookCommand{Type: HookCommandTypeHTTP, URL: srv.URL}
	result := ExecHTTPHook(ctx, hook, `{}`)
	if result.OK {
		t.Error("expected failure due to cancelled context")
	}
}

func TestExecHTTPHook_NoRedirects(t *testing.T) {
	// Source: execHttpHook.ts:206 — maxRedirects: 0
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Location", "https://evil.com")
		w.WriteHeader(302)
	}))
	defer srv.Close()

	hook := HookCommand{Type: HookCommandTypeHTTP, URL: srv.URL}
	result := ExecHTTPHook(context.Background(), hook, `{}`)

	// 302 is not 2xx, so !OK
	if result.OK {
		t.Error("redirect should not be OK (no auto-follow)")
	}
	if result.StatusCode != 302 {
		t.Errorf("status = %d, want 302", result.StatusCode)
	}
}

func TestInterpolateEnvVars(t *testing.T) {
	// Source: execHttpHook.ts:89-108

	t.Run("dollar_notation", func(t *testing.T) {
		os.Setenv("TEST_TOKEN", "abc")
		defer os.Unsetenv("TEST_TOKEN")
		result := InterpolateEnvVars("Bearer $TEST_TOKEN", map[string]bool{"TEST_TOKEN": true})
		if result != "Bearer abc" {
			t.Errorf("got %q, want 'Bearer abc'", result)
		}
	})

	t.Run("braced_notation", func(t *testing.T) {
		os.Setenv("MY_KEY", "xyz")
		defer os.Unsetenv("MY_KEY")
		result := InterpolateEnvVars("Key: ${MY_KEY}", map[string]bool{"MY_KEY": true})
		if result != "Key: xyz" {
			t.Errorf("got %q, want 'Key: xyz'", result)
		}
	})

	t.Run("not_in_allowlist_becomes_empty", func(t *testing.T) {
		// Source: execHttpHook.ts:98-103 — vars not in allowlist → empty string
		os.Setenv("SECRET", "hidden")
		defer os.Unsetenv("SECRET")
		result := InterpolateEnvVars("Auth: $SECRET", map[string]bool{})
		if result != "Auth: " {
			t.Errorf("got %q, want 'Auth: '", result)
		}
	})

	t.Run("unset_var_becomes_empty", func(t *testing.T) {
		os.Unsetenv("NONEXISTENT_VAR")
		result := InterpolateEnvVars("$NONEXISTENT_VAR", map[string]bool{"NONEXISTENT_VAR": true})
		if result != "" {
			t.Errorf("got %q, want empty", result)
		}
	})

	t.Run("multiple_vars", func(t *testing.T) {
		os.Setenv("USER_ID", "123")
		os.Setenv("API_KEY", "abc")
		defer os.Unsetenv("USER_ID")
		defer os.Unsetenv("API_KEY")
		result := InterpolateEnvVars("$USER_ID:$API_KEY", map[string]bool{"USER_ID": true, "API_KEY": true})
		if result != "123:abc" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("no_vars_passthrough", func(t *testing.T) {
		result := InterpolateEnvVars("no variables here", map[string]bool{})
		if result != "no variables here" {
			t.Errorf("got %q", result)
		}
	})
}

func TestSanitizeHeaderValue(t *testing.T) {
	// Source: execHttpHook.ts:76-79 — strip CR, LF, NUL
	t.Run("strips_cr_lf", func(t *testing.T) {
		result := SanitizeHeaderValue("token\r\nX-Evil: 1")
		if result != "tokenX-Evil: 1" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("strips_nul", func(t *testing.T) {
		result := SanitizeHeaderValue("token\x00value")
		if result != "tokenvalue" {
			t.Errorf("got %q", result)
		}
	})

	t.Run("clean_passthrough", func(t *testing.T) {
		result := SanitizeHeaderValue("Bearer abc123")
		if result != "Bearer abc123" {
			t.Errorf("got %q", result)
		}
	})
}

func TestURLMatchesPattern(t *testing.T) {
	// Source: execHttpHook.ts:64-68

	t.Run("exact_match", func(t *testing.T) {
		if !URLMatchesPattern("https://example.com/hook", "https://example.com/hook") {
			t.Error("exact match should pass")
		}
	})

	t.Run("wildcard_subdomain", func(t *testing.T) {
		if !URLMatchesPattern("https://api.example.com/hook", "https://*.example.com/hook") {
			t.Error("wildcard subdomain should match")
		}
	})

	t.Run("wildcard_path", func(t *testing.T) {
		if !URLMatchesPattern("https://example.com/hooks/v1/pre", "https://example.com/hooks/*") {
			t.Error("wildcard path should match")
		}
	})

	t.Run("no_match", func(t *testing.T) {
		if URLMatchesPattern("https://evil.com/hook", "https://example.com/*") {
			t.Error("different domain should not match")
		}
	})

	t.Run("partial_match_fails", func(t *testing.T) {
		// Anchored — partial match must fail
		if URLMatchesPattern("https://example.com/hook/extra", "https://example.com/hook") {
			t.Error("partial match should fail (anchored)")
		}
	})
}

// readAll is a helper to read a request body.
func readAll(r interface{ Read([]byte) (int, error) }) (string, error) {
	var buf strings.Builder
	b := make([]byte, 1024)
	for {
		n, err := r.Read(b)
		if n > 0 {
			buf.Write(b[:n])
		}
		if err != nil {
			break
		}
	}
	return buf.String(), nil
}
