package hooks

import (
	"bytes"
	"context"
	"fmt"
	"io"
	"net/http"
	"os"
	"regexp"
	"strings"
	"time"
)

// Source: utils/hooks/execHttpHook.ts

// DefaultHTTPHookTimeoutMs is the default timeout for HTTP hooks.
// Source: execHttpHook.ts:12
const DefaultHTTPHookTimeoutMs = 10 * 60 * 1000 // 10 minutes

// HTTPHookResult is the outcome of executing an HTTP hook.
// Source: execHttpHook.ts:128-134
type HTTPHookResult struct {
	OK         bool
	StatusCode int
	Body       string
	Error      string
	Aborted    bool
}

// ExecHTTPHook executes an HTTP hook by POSTing hook input JSON to the configured URL.
// Source: execHttpHook.ts:123-242
func ExecHTTPHook(ctx context.Context, hook HookCommand, jsonInput string) HTTPHookResult {
	if hook.URL == "" {
		return HTTPHookResult{OK: false, Body: "", Error: "HTTP hook URL is empty"}
	}

	// Calculate timeout
	// Source: execHttpHook.ts:147-149
	timeoutMs := DefaultHTTPHookTimeoutMs
	if hook.Timeout > 0 {
		timeoutMs = hook.Timeout * 1000
	}

	// Create context with timeout
	hookCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Build headers with env var interpolation
	// Source: execHttpHook.ts:158-172
	headers := map[string]string{
		"Content-Type": "application/json",
	}
	if hook.Headers != nil {
		allowedEnvVars := buildAllowedEnvVarSet(hook.AllowedEnvVars)
		for name, value := range hook.Headers {
			headers[name] = InterpolateEnvVars(value, allowedEnvVars)
		}
	}

	// Build HTTP request
	req, err := http.NewRequestWithContext(hookCtx, http.MethodPost, hook.URL, bytes.NewBufferString(jsonInput))
	if err != nil {
		return HTTPHookResult{OK: false, Body: "", Error: fmt.Sprintf("create request: %s", err)}
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Execute request
	// Source: execHttpHook.ts:201-217
	client := &http.Client{
		CheckRedirect: func(req *http.Request, via []*http.Request) error {
			// Source: execHttpHook.ts:206 — maxRedirects: 0
			return http.ErrUseLastResponse
		},
	}

	resp, err := client.Do(req)
	if err != nil {
		if hookCtx.Err() != nil {
			return HTTPHookResult{OK: false, Body: "", Aborted: true}
		}
		return HTTPHookResult{OK: false, Body: "", Error: err.Error()}
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return HTTPHookResult{OK: false, Body: "", Error: fmt.Sprintf("read response: %s", err)}
	}

	// Source: execHttpHook.ts:226-230
	return HTTPHookResult{
		OK:         resp.StatusCode >= 200 && resp.StatusCode < 300,
		StatusCode: resp.StatusCode,
		Body:       string(body),
	}
}

// envVarPattern matches $VAR_NAME and ${VAR_NAME} patterns.
// Source: execHttpHook.ts:93-94
var envVarPattern = regexp.MustCompile(`\$\{([A-Z_][A-Z0-9_]*)\}|\$([A-Z_][A-Z0-9_]*)`)

// InterpolateEnvVars replaces $VAR_NAME and ${VAR_NAME} in a string using os.Getenv,
// but only for variables in the allowlist. Unallowed references become empty strings.
// Source: execHttpHook.ts:89-108
func InterpolateEnvVars(value string, allowedEnvVars map[string]bool) string {
	result := envVarPattern.ReplaceAllStringFunc(value, func(match string) string {
		// Extract variable name from either ${VAR} or $VAR
		varName := match
		if strings.HasPrefix(match, "${") {
			varName = match[2 : len(match)-1]
		} else {
			varName = match[1:]
		}

		if !allowedEnvVars[varName] {
			return ""
		}
		return os.Getenv(varName)
	})
	return SanitizeHeaderValue(result)
}

// SanitizeHeaderValue strips CR, LF, and NUL bytes to prevent HTTP header injection.
// Source: execHttpHook.ts:76-79
func SanitizeHeaderValue(value string) string {
	return strings.Map(func(r rune) rune {
		if r == '\r' || r == '\n' || r == 0 {
			return -1 // drop the character
		}
		return r
	}, value)
}

// URLMatchesPattern matches a URL against a wildcard pattern.
// Source: execHttpHook.ts:64-68
func URLMatchesPattern(url, pattern string) bool {
	// Escape regex special chars, then replace * with .*
	escaped := regexp.QuoteMeta(pattern)
	regexStr := strings.ReplaceAll(escaped, `\*`, ".*")
	re, err := regexp.Compile("^" + regexStr + "$")
	if err != nil {
		return false
	}
	return re.MatchString(url)
}

func buildAllowedEnvVarSet(vars []string) map[string]bool {
	set := make(map[string]bool, len(vars))
	for _, v := range vars {
		set[v] = true
	}
	return set
}
