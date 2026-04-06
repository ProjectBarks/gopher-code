// Package bridge — thin HTTP wrappers for the CCR v2 code-session API.
// Source: src/bridge/codeSessionApi.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"math"
	"net/http"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// codeSessionAnthropicVersion is sent as anthropic-version on code-session requests.
const codeSessionAnthropicVersion = "2023-06-01"

// maxPayloadLogLen is the truncation limit for payload dumps in debug logs.
const maxPayloadLogLen = 200

// maxSafeInteger matches JS Number.MAX_SAFE_INTEGER (2^53 - 1).
const maxSafeInteger = 1<<53 - 1

// ---------------------------------------------------------------------------
// RemoteCredentials
// ---------------------------------------------------------------------------

// RemoteCredentials holds the response from POST /v1/code/sessions/{id}/bridge.
// The JWT is opaque -- do not decode.
type RemoteCredentials struct {
	WorkerJWT  string `json:"worker_jwt"`
	APIBaseURL string `json:"api_base_url"`
	ExpiresIn  int    `json:"expires_in"`
	WorkerEpoch int64 `json:"worker_epoch"`
}

// ---------------------------------------------------------------------------
// CodeSessionClient
// ---------------------------------------------------------------------------

// CodeSessionClientConfig holds dependencies for NewCodeSessionClient.
type CodeSessionClientConfig struct {
	// OnDebug is called with debug log messages.
	OnDebug func(msg string)
}

// CodeSessionClient is a thin HTTP client for the code-session API.
type CodeSessionClient struct {
	cfg        CodeSessionClientConfig
	httpClient *http.Client
}

// NewCodeSessionClient creates a client for code-session API calls.
func NewCodeSessionClient(cfg CodeSessionClientConfig) *CodeSessionClient {
	return &CodeSessionClient{
		cfg:        cfg,
		httpClient: &http.Client{},
	}
}

func (c *CodeSessionClient) debug(msg string) {
	if c.cfg.OnDebug != nil {
		c.cfg.OnDebug(msg)
	}
}

func codeSessionOAuthHeaders(accessToken string) map[string]string {
	return map[string]string{
		"Authorization":    "Bearer " + accessToken,
		"Content-Type":     "application/json",
		"anthropic-version": codeSessionAnthropicVersion,
	}
}

// truncatePayload truncates s to maxPayloadLogLen characters.
func truncatePayload(s string) string {
	if len(s) <= maxPayloadLogLen {
		return s
	}
	return s[:maxPayloadLogLen]
}

// ---------------------------------------------------------------------------
// CreateCodeSession — POST /v1/code/sessions
// ---------------------------------------------------------------------------

// CreateCodeSession creates a code session and returns the session ID (cse_*).
// Returns ("", nil) for non-fatal failures (matching TS null-return semantics).
func (c *CodeSessionClient) CreateCodeSession(
	ctx context.Context,
	baseURL string,
	accessToken string,
	title string,
	timeoutMs int,
	tags []string,
) (string, error) {
	url := baseURL + "/v1/code/sessions"

	body := map[string]any{
		"title":  title,
		"bridge": map[string]any{},
	}
	if len(tags) > 0 {
		body["tags"] = tags
	}

	b, err := json.Marshal(body)
	if err != nil {
		return "", nil
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader(b))
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] Session create request failed: %s", err.Error()))
		return "", nil
	}
	for k, v := range codeSessionOAuthHeaders(accessToken) {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] Session create request failed: %s", err.Error()))
		return "", nil
	}
	defer resp.Body.Close()

	// Treat 5xx as transport error.
	if resp.StatusCode >= 500 {
		c.debug(fmt.Sprintf("[code-session] Session create request failed: server error %d", resp.StatusCode))
		return "", nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] Session create request failed: %s", err.Error()))
		return "", nil
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		detail := extractCodeSessionErrorDetail(data)
		msg := fmt.Sprintf("[code-session] Session create failed %d", resp.StatusCode)
		if detail != "" {
			msg += ": " + detail
		}
		c.debug(msg)
		return "", nil
	}

	// Parse response: expecting { session: { id: "cse_..." } }
	var result struct {
		Session struct {
			ID string `json:"id"`
		} `json:"session"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.Session.ID == "" || !strings.HasPrefix(result.Session.ID, "cse_") {
		c.debug(fmt.Sprintf("[code-session] No session.id (cse_*) in response: %s", truncatePayload(string(data))))
		return "", nil
	}

	return result.Session.ID, nil
}

// ---------------------------------------------------------------------------
// FetchRemoteCredentials — POST /v1/code/sessions/{id}/bridge
// ---------------------------------------------------------------------------

// FetchRemoteCredentials fetches worker credentials from the bridge endpoint.
// Returns (nil, nil) for non-fatal failures (matching TS null-return semantics).
func (c *CodeSessionClient) FetchRemoteCredentials(
	ctx context.Context,
	sessionID string,
	baseURL string,
	accessToken string,
	timeoutMs int,
	trustedDeviceToken string,
) (*RemoteCredentials, error) {
	url := baseURL + "/v1/code/sessions/" + sessionID + "/bridge"

	headers := codeSessionOAuthHeaders(accessToken)
	if trustedDeviceToken != "" {
		headers["X-Trusted-Device-Token"] = trustedDeviceToken
	}

	reqCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	req, err := http.NewRequestWithContext(reqCtx, http.MethodPost, url, bytes.NewReader([]byte("{}")))
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] /bridge request failed: %s", err.Error()))
		return nil, nil
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	resp, err := c.httpClient.Do(req)
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] /bridge request failed: %s", err.Error()))
		return nil, nil
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		c.debug(fmt.Sprintf("[code-session] /bridge request failed: server error %d", resp.StatusCode))
		return nil, nil
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] /bridge request failed: %s", err.Error()))
		return nil, nil
	}

	if resp.StatusCode != 200 {
		detail := extractCodeSessionErrorDetail(data)
		msg := fmt.Sprintf("[code-session] /bridge failed %d", resp.StatusCode)
		if detail != "" {
			msg += ": " + detail
		}
		c.debug(msg)
		return nil, nil
	}

	// Parse with json.Number to handle protojson int64-as-string for worker_epoch.
	dec := json.NewDecoder(bytes.NewReader(data))
	dec.UseNumber()
	var raw map[string]any
	if err := dec.Decode(&raw); err != nil {
		c.debug(fmt.Sprintf("[code-session] /bridge response malformed (need worker_jwt, expires_in, api_base_url, worker_epoch): %s", truncatePayload(string(data))))
		return nil, nil
	}

	workerJWT, _ := raw["worker_jwt"].(string)
	apiBaseURL, _ := raw["api_base_url"].(string)
	expiresInNum, expiresInOK := raw["expires_in"].(json.Number)
	rawEpoch, epochExists := raw["worker_epoch"]

	if workerJWT == "" || apiBaseURL == "" || !expiresInOK || !epochExists {
		c.debug(fmt.Sprintf("[code-session] /bridge response malformed (need worker_jwt, expires_in, api_base_url, worker_epoch): %s", truncatePayload(string(data))))
		return nil, nil
	}

	expiresIn, err := expiresInNum.Int64()
	if err != nil {
		c.debug(fmt.Sprintf("[code-session] /bridge response malformed (need worker_jwt, expires_in, api_base_url, worker_epoch): %s", truncatePayload(string(data))))
		return nil, nil
	}

	// worker_epoch: may be number or string (protojson int64-as-string).
	var epoch int64
	switch v := rawEpoch.(type) {
	case json.Number:
		epoch, err = v.Int64()
		if err != nil {
			// Try float for non-integer numbers.
			f, ferr := v.Float64()
			if ferr != nil || math.IsInf(f, 0) || math.IsNaN(f) || f != math.Trunc(f) {
				rawBytes, _ := json.Marshal(rawEpoch)
				c.debug(fmt.Sprintf("[code-session] /bridge worker_epoch invalid: %s", string(rawBytes)))
				return nil, nil
			}
			epoch = int64(f)
		}
	case string:
		// protojson serialises int64 as string.
		n := json.Number(v)
		epoch, err = n.Int64()
		if err != nil {
			rawBytes, _ := json.Marshal(rawEpoch)
			c.debug(fmt.Sprintf("[code-session] /bridge worker_epoch invalid: %s", string(rawBytes)))
			return nil, nil
		}
	default:
		rawBytes, _ := json.Marshal(rawEpoch)
		c.debug(fmt.Sprintf("[code-session] /bridge worker_epoch invalid: %s", string(rawBytes)))
		return nil, nil
	}

	// Reject if not a safe integer (> 2^53 - 1).
	if epoch < -maxSafeInteger || epoch > maxSafeInteger {
		rawBytes, _ := json.Marshal(rawEpoch)
		c.debug(fmt.Sprintf("[code-session] /bridge worker_epoch invalid: %s", string(rawBytes)))
		return nil, nil
	}

	return &RemoteCredentials{
		WorkerJWT:   workerJWT,
		APIBaseURL:  apiBaseURL,
		ExpiresIn:   int(expiresIn),
		WorkerEpoch: epoch,
	}, nil
}

// ---------------------------------------------------------------------------
// extractCodeSessionErrorDetail
// ---------------------------------------------------------------------------

func extractCodeSessionErrorDetail(data []byte) string {
	if len(data) == 0 {
		return ""
	}
	var obj struct {
		Message string `json:"message"`
		Error   struct {
			Message string `json:"message"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &obj) != nil {
		return ""
	}
	if obj.Message != "" {
		return obj.Message
	}
	return obj.Error.Message
}
