// Source: src/bridge/workSecret.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// DecodeWorkSecret — base64url → JSON → validate
// ---------------------------------------------------------------------------

// DecodeWorkSecret decodes a base64url-encoded work secret string and
// validates the version, session_ingress_token, and api_base_url fields.
func DecodeWorkSecret(secret string) (WorkSecret, error) {
	raw, err := base64.RawURLEncoding.DecodeString(secret)
	if err != nil {
		// Also try standard base64url (with padding).
		raw, err = base64.URLEncoding.DecodeString(secret)
		if err != nil {
			return WorkSecret{}, fmt.Errorf("invalid base64url encoding: %w", err)
		}
	}

	var ws WorkSecret
	if err := json.Unmarshal(raw, &ws); err != nil {
		return WorkSecret{}, fmt.Errorf("invalid work secret JSON: %w", err)
	}

	if ws.Version != 1 {
		return WorkSecret{}, fmt.Errorf("Unsupported work secret version: %d", ws.Version)
	}
	if ws.SessionIngressToken == "" {
		return WorkSecret{}, fmt.Errorf("Invalid work secret: missing or empty session_ingress_token")
	}
	if ws.APIBaseURL == "" {
		return WorkSecret{}, fmt.Errorf("Invalid work secret: missing api_base_url")
	}

	return ws, nil
}

// ---------------------------------------------------------------------------
// BuildSdkUrl — WebSocket ingress URL
// ---------------------------------------------------------------------------

// BuildSdkUrl constructs a WebSocket SDK URL from the API base URL and
// session ID. Uses ws:///v2/ for localhost, wss:///v1/ for production.
func BuildSdkUrl(apiBaseURL, sessionID string) string {
	isLocalhost := strings.Contains(apiBaseURL, "localhost") ||
		strings.Contains(apiBaseURL, "127.0.0.1")

	protocol := "wss"
	version := "v1"
	if isLocalhost {
		protocol = "ws"
		version = "v2"
	}

	host := apiBaseURL
	// Strip http(s):// prefix.
	host = strings.TrimPrefix(host, "https://")
	host = strings.TrimPrefix(host, "http://")
	// Strip trailing slashes.
	host = strings.TrimRight(host, "/")

	return fmt.Sprintf("%s://%s/%s/session_ingress/ws/%s", protocol, host, version, sessionID)
}

// ---------------------------------------------------------------------------
// BuildCCRv2SdkUrl — HTTP session URL
// ---------------------------------------------------------------------------

// BuildCCRv2SdkUrl constructs an HTTP(S) session URL for CCR v2.
func BuildCCRv2SdkUrl(apiBaseURL, sessionID string) string {
	base := strings.TrimRight(apiBaseURL, "/")
	return fmt.Sprintf("%s/v1/code/sessions/%s", base, sessionID)
}

// ---------------------------------------------------------------------------
// SameSessionId — tagged-ID equivalence
// ---------------------------------------------------------------------------

// SameSessionId compares two session IDs regardless of their tagged-ID
// prefix. Tagged IDs have the form {tag}_{body} or {tag}_staging_{body};
// CCR v2 compat returns session_* while infrastructure uses cse_*.
func SameSessionId(a, b string) bool {
	if a == b {
		return true
	}
	aBody := a[strings.LastIndex(a, "_")+1:]
	bBody := b[strings.LastIndex(b, "_")+1:]
	// Require minimum length of 4 to avoid accidental matches.
	return len(aBody) >= 4 && aBody == bBody
}

// ---------------------------------------------------------------------------
// RegisterWorker — POST /worker/register
// ---------------------------------------------------------------------------

// RegisterWorker registers this bridge as the worker for a CCR v2 session
// and returns the worker_epoch.
func RegisterWorker(ctx context.Context, sessionURL, accessToken string) (int64, error) {
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()

	url := sessionURL + "/worker/register"
	req, err := http.NewRequestWithContext(ctx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		return 0, fmt.Errorf("registerWorker: failed to create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("anthropic-version", "2023-06-01")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return 0, fmt.Errorf("registerWorker: request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return 0, fmt.Errorf("registerWorker: failed to read response: %w", err)
	}

	// Use json.Number to handle protojson int64 (string or number).
	var result map[string]json.Number
	if err := json.Unmarshal(body, &result); err != nil {
		return 0, fmt.Errorf("registerWorker: invalid worker_epoch in response: %s", string(body))
	}

	epochNum, ok := result["worker_epoch"]
	if !ok {
		return 0, fmt.Errorf("registerWorker: invalid worker_epoch in response: %s", string(body))
	}

	epoch, err := epochNum.Int64()
	if err != nil {
		return 0, fmt.Errorf("registerWorker: invalid worker_epoch in response: %s", string(body))
	}

	// Match JS Number.isSafeInteger: -(2^53-1) to (2^53-1).
	const maxSafeInt = 1<<53 - 1
	if epoch < -maxSafeInt || epoch > maxSafeInt {
		return 0, fmt.Errorf("registerWorker: invalid worker_epoch in response: %s", string(body))
	}

	return epoch, nil
}
