// Package bridge — BridgeApiClient implementation.
// Source: src/bridge/bridgeApi.ts
package bridge

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"regexp"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// BetaHeader is sent as anthropic-beta on every bridge API request.
const BetaHeader = "environments-2025-11-01"

// AnthropicVersion is sent as anthropic-version on every bridge API request.
const AnthropicVersion = "2023-06-01"

// safeIDPattern is the allowlist for server-provided IDs used in URL path segments.
var safeIDPattern = regexp.MustCompile(`^[a-zA-Z0-9_-]+$`)

// registrationTimeout is the HTTP timeout for registerBridgeEnvironment.
const registrationTimeout = 15 * time.Second

// defaultTimeout is the HTTP timeout for all other bridge API requests.
const defaultTimeout = 10 * time.Second

// emptyPollLogInterval controls how often consecutive empty polls are logged.
const emptyPollLogInterval = 100

// ---------------------------------------------------------------------------
// BridgeFatalError
// ---------------------------------------------------------------------------

// BridgeFatalError represents a fatal bridge error that should not be retried.
type BridgeFatalError struct {
	Status    int
	ErrorType string
	Msg       string
}

func (e *BridgeFatalError) Error() string { return e.Msg }

// IsExpiredErrorType checks whether an error type string indicates expiry.
func IsExpiredErrorType(errorType string) bool {
	if errorType == "" {
		return false
	}
	return contains(errorType, "expired") || contains(errorType, "lifetime")
}

// IsSuppressible403 checks whether a BridgeFatalError is a suppressible 403.
func IsSuppressible403(err *BridgeFatalError) bool {
	if err.Status != 403 {
		return false
	}
	return contains(err.Msg, "external_poll_sessions") || contains(err.Msg, "environments:manage")
}

func contains(s, substr string) bool {
	return len(s) >= len(substr) && (s == substr || len(substr) == 0 ||
		findSubstring(s, substr))
}

func findSubstring(s, sub string) bool {
	for i := 0; i <= len(s)-len(sub); i++ {
		if s[i:i+len(sub)] == sub {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// ValidateBridgeID
// ---------------------------------------------------------------------------

// ValidateBridgeID validates that a server-provided ID is safe for URL path
// interpolation. Returns the ID on success or an error if it contains unsafe
// characters.
func ValidateBridgeID(id, label string) (string, error) {
	if id == "" || !safeIDPattern.MatchString(id) {
		return "", fmt.Errorf("Invalid %s: contains unsafe characters", label)
	}
	return id, nil
}

// ---------------------------------------------------------------------------
// BridgeAPIClientConfig — constructor dependencies
// ---------------------------------------------------------------------------

// BridgeAPIClientConfig holds the dependencies for NewBridgeAPIClient.
type BridgeAPIClientConfig struct {
	BaseURL               string
	RunnerVersion         string
	GetAccessToken        func() string
	OnAuth401             func(staleToken string) (bool, error)
	GetTrustedDeviceToken func() string
	OnDebug               func(msg string)
}

// ---------------------------------------------------------------------------
// bridgeAPIClient
// ---------------------------------------------------------------------------

type bridgeAPIClient struct {
	cfg                  BridgeAPIClientConfig
	httpClient           *retryablehttp.Client
	consecutiveEmptyPolls int
}

// NewBridgeAPIClient creates a BridgeAPIClient backed by go-retryablehttp.
func NewBridgeAPIClient(cfg BridgeAPIClientConfig) BridgeAPIClient {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 2
	rc.Logger = nil // suppress retryablehttp's default logging
	rc.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy

	return &bridgeAPIClient{cfg: cfg, httpClient: rc}
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func (c *bridgeAPIClient) debug(msg string) {
	if c.cfg.OnDebug != nil {
		c.cfg.OnDebug(msg)
	}
}

func (c *bridgeAPIClient) headers(token string) map[string]string {
	h := map[string]string{
		"Authorization":                  "Bearer " + token,
		"Content-Type":                   "application/json",
		"anthropic-version":              AnthropicVersion,
		"anthropic-beta":                 BetaHeader,
		"x-environment-runner-version":   c.cfg.RunnerVersion,
	}
	if c.cfg.GetTrustedDeviceToken != nil {
		if dt := c.cfg.GetTrustedDeviceToken(); dt != "" {
			h["X-Trusted-Device-Token"] = dt
		}
	}
	return h
}

func (c *bridgeAPIClient) resolveAuth() (string, error) {
	tok := c.cfg.GetAccessToken()
	if tok == "" {
		return "", fmt.Errorf("%s", BridgeLoginInstruction)
	}
	return tok, nil
}

// doJSON performs a JSON request and decodes the response. It accepts
// status < 500 (matching TS validateStatus).
func (c *bridgeAPIClient) doJSON(ctx context.Context, method, url string, body any, headers map[string]string, timeout time.Duration) (int, json.RawMessage, error) {
	var bodyReader io.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return 0, nil, err
		}
		bodyReader = bytes.NewReader(b)
	}

	req, err := retryablehttp.NewRequestWithContext(ctx, method, url, bodyReader)
	if err != nil {
		return 0, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	// Per-request timeout via context.
	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req.Request = req.Request.WithContext(tctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return 0, nil, err
	}
	defer resp.Body.Close()

	if resp.StatusCode >= 500 {
		return resp.StatusCode, nil, fmt.Errorf("server error: %d", resp.StatusCode)
	}

	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return resp.StatusCode, nil, err
	}
	return resp.StatusCode, json.RawMessage(data), nil
}

// withOAuthRetry executes fn, and on 401 attempts a single token refresh + retry.
func (c *bridgeAPIClient) withOAuthRetry(fn func(token string) (int, json.RawMessage, error), ctx string) (int, json.RawMessage, error) {
	token, err := c.resolveAuth()
	if err != nil {
		return 0, nil, err
	}

	status, data, err := fn(token)
	if err != nil {
		return status, data, err
	}
	if status != 401 {
		return status, data, nil
	}

	if c.cfg.OnAuth401 == nil {
		c.debug(fmt.Sprintf("[bridge:api] %s: 401 received, no refresh handler", ctx))
		return status, data, nil
	}

	c.debug(fmt.Sprintf("[bridge:api] %s: 401 received, attempting token refresh", ctx))
	refreshed, err := c.cfg.OnAuth401(token)
	if err != nil {
		return status, data, err
	}
	if refreshed {
		c.debug(fmt.Sprintf("[bridge:api] %s: Token refreshed, retrying request", ctx))
		newToken, err := c.resolveAuth()
		if err != nil {
			return 0, nil, err
		}
		retryStatus, retryData, retryErr := fn(newToken)
		if retryErr != nil {
			return retryStatus, retryData, retryErr
		}
		if retryStatus != 401 {
			return retryStatus, retryData, nil
		}
		c.debug(fmt.Sprintf("[bridge:api] %s: Retry after refresh also got 401", ctx))
	} else {
		c.debug(fmt.Sprintf("[bridge:api] %s: Token refresh failed", ctx))
	}
	return status, data, nil
}

// handleErrorStatus maps non-success HTTP status codes to errors.
func handleErrorStatus(status int, data json.RawMessage, ctx string) error {
	if status == 200 || status == 204 {
		return nil
	}
	detail := extractErrorDetail(data)
	errorType := extractErrorType(data)

	switch status {
	case 401:
		msg := fmt.Sprintf("%s: Authentication failed (401)", ctx)
		if detail != "" {
			msg += ": " + detail
		}
		msg += ". " + BridgeLoginInstruction
		return &BridgeFatalError{Msg: msg, Status: 401, ErrorType: errorType}
	case 403:
		var msg string
		if IsExpiredErrorType(errorType) {
			msg = "Remote Control session has expired. Please restart with `claude remote-control` or /remote-control."
		} else {
			msg = fmt.Sprintf("%s: Access denied (403)", ctx)
			if detail != "" {
				msg += ": " + detail
			}
			msg += ". Check your organization permissions."
		}
		return &BridgeFatalError{Msg: msg, Status: 403, ErrorType: errorType}
	case 404:
		msg := detail
		if msg == "" {
			msg = fmt.Sprintf("%s: Not found (404). Remote Control may not be available for this organization.", ctx)
		}
		return &BridgeFatalError{Msg: msg, Status: 404, ErrorType: errorType}
	case 410:
		msg := detail
		if msg == "" {
			msg = "Remote Control session has expired. Please restart with `claude remote-control` or /remote-control."
		}
		et := errorType
		if et == "" {
			et = "environment_expired"
		}
		return &BridgeFatalError{Msg: msg, Status: 410, ErrorType: et}
	case 429:
		return fmt.Errorf("%s: Rate limited (429). Polling too frequently.", ctx)
	default:
		msg := fmt.Sprintf("%s: Failed with status %d", ctx, status)
		if detail != "" {
			msg += ": " + detail
		}
		return fmt.Errorf("%s", msg)
	}
}

// extractErrorDetail pulls a human-readable detail from a JSON error response body.
func extractErrorDetail(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var obj struct {
		Error struct {
			Message string `json:"message"`
		} `json:"error"`
		Message string `json:"message"`
	}
	if json.Unmarshal(data, &obj) != nil {
		return ""
	}
	if obj.Error.Message != "" {
		return obj.Error.Message
	}
	return obj.Message
}

// extractErrorType pulls the error type from a JSON error response body.
func extractErrorType(data json.RawMessage) string {
	if len(data) == 0 {
		return ""
	}
	var obj struct {
		Error struct {
			Type string `json:"type"`
		} `json:"error"`
	}
	if json.Unmarshal(data, &obj) != nil {
		return ""
	}
	return obj.Error.Type
}

// ---------------------------------------------------------------------------
// BridgeAPIClient method implementations
// ---------------------------------------------------------------------------

func (c *bridgeAPIClient) RegisterBridgeEnvironment(config BridgeConfig) (*RegisterEnvironmentResponse, error) {
	c.debug(fmt.Sprintf("[bridge:api] POST /v1/environments/bridge bridgeId=%s", config.BridgeID))

	reqBody := map[string]any{
		"machine_name": config.MachineName,
		"directory":    config.Dir,
		"branch":       config.Branch,
		"git_repo_url": config.GitRepoURL,
		"max_sessions": config.MaxSessions,
		"metadata":     map[string]any{"worker_type": config.WorkerType},
	}
	if config.ReuseEnvironmentID != "" {
		reqBody["environment_id"] = config.ReuseEnvironmentID
	}

	status, data, err := c.withOAuthRetry(func(token string) (int, json.RawMessage, error) {
		url := c.cfg.BaseURL + "/v1/environments/bridge"
		return c.doJSON(context.Background(), http.MethodPost, url, reqBody, c.headers(token), registrationTimeout)
	}, "Registration")
	if err != nil {
		return nil, err
	}

	if err := handleErrorStatus(status, data, "Registration"); err != nil {
		return nil, err
	}

	var resp RegisterEnvironmentResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("Registration: invalid response body: %w", err)
	}
	c.debug(fmt.Sprintf("[bridge:api] POST /v1/environments/bridge -> %d environment_id=%s", status, resp.EnvironmentID))
	return &resp, nil
}

func (c *bridgeAPIClient) PollForWork(environmentID string, environmentSecret string, reclaimOlderThanMS *int) (*WorkResponse, error) {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return nil, err
	}

	prevEmpty := c.consecutiveEmptyPolls
	c.consecutiveEmptyPolls = 0

	url := c.cfg.BaseURL + "/v1/environments/" + environmentID + "/work/poll"
	if reclaimOlderThanMS != nil {
		url += fmt.Sprintf("?reclaim_older_than_ms=%d", *reclaimOlderThanMS)
	}

	status, data, err := c.doJSON(context.Background(), http.MethodGet, url, nil, c.headers(environmentSecret), defaultTimeout)
	if err != nil {
		return nil, err
	}

	if err := handleErrorStatus(status, data, "Poll"); err != nil {
		return nil, err
	}

	// Empty body or "null" = no work available.
	if len(data) == 0 || string(data) == "null" || string(data) == "" {
		c.consecutiveEmptyPolls = prevEmpty + 1
		if c.consecutiveEmptyPolls == 1 || c.consecutiveEmptyPolls%emptyPollLogInterval == 0 {
			c.debug(fmt.Sprintf("[bridge:api] GET .../work/poll -> %d (no work, %d consecutive empty polls)", status, c.consecutiveEmptyPolls))
		}
		return nil, nil
	}

	var resp WorkResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("Poll: invalid response body: %w", err)
	}
	c.debug(fmt.Sprintf("[bridge:api] GET .../work/poll -> %d workId=%s type=%s", status, resp.ID, resp.Data.Type))
	return &resp, nil
}

func (c *bridgeAPIClient) AcknowledgeWork(environmentID string, workID string, sessionToken string) error {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return err
	}
	if _, err := ValidateBridgeID(workID, "workId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/ack", workID))

	url := c.cfg.BaseURL + "/v1/environments/" + environmentID + "/work/" + workID + "/ack"
	status, data, err := c.doJSON(context.Background(), http.MethodPost, url, map[string]any{}, c.headers(sessionToken), defaultTimeout)
	if err != nil {
		return err
	}

	if err := handleErrorStatus(status, data, "Acknowledge"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/ack -> %d", workID, status))
	return nil
}

func (c *bridgeAPIClient) StopWork(environmentID string, workID string, force bool) error {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return err
	}
	if _, err := ValidateBridgeID(workID, "workId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/stop force=%v", workID, force))

	status, data, err := c.withOAuthRetry(func(token string) (int, json.RawMessage, error) {
		url := c.cfg.BaseURL + "/v1/environments/" + environmentID + "/work/" + workID + "/stop"
		return c.doJSON(context.Background(), http.MethodPost, url, map[string]any{"force": force}, c.headers(token), defaultTimeout)
	}, "StopWork")
	if err != nil {
		return err
	}

	if err := handleErrorStatus(status, data, "StopWork"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/stop -> %d", workID, status))
	return nil
}

func (c *bridgeAPIClient) DeregisterEnvironment(environmentID string) error {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] DELETE /v1/environments/bridge/%s", environmentID))

	status, data, err := c.withOAuthRetry(func(token string) (int, json.RawMessage, error) {
		url := c.cfg.BaseURL + "/v1/environments/bridge/" + environmentID
		return c.doJSON(context.Background(), http.MethodDelete, url, nil, c.headers(token), defaultTimeout)
	}, "Deregister")
	if err != nil {
		return err
	}

	if err := handleErrorStatus(status, data, "Deregister"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] DELETE /v1/environments/bridge/%s -> %d", environmentID, status))
	return nil
}

func (c *bridgeAPIClient) ArchiveSession(sessionID string) error {
	if _, err := ValidateBridgeID(sessionID, "sessionId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST /v1/sessions/%s/archive", sessionID))

	status, data, err := c.withOAuthRetry(func(token string) (int, json.RawMessage, error) {
		url := c.cfg.BaseURL + "/v1/sessions/" + sessionID + "/archive"
		return c.doJSON(context.Background(), http.MethodPost, url, map[string]any{}, c.headers(token), defaultTimeout)
	}, "ArchiveSession")
	if err != nil {
		return err
	}

	// 409 = already archived (idempotent).
	if status == 409 {
		c.debug(fmt.Sprintf("[bridge:api] POST /v1/sessions/%s/archive -> 409 (already archived)", sessionID))
		return nil
	}

	if err := handleErrorStatus(status, data, "ArchiveSession"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] POST /v1/sessions/%s/archive -> %d", sessionID, status))
	return nil
}

func (c *bridgeAPIClient) ReconnectSession(environmentID string, sessionID string) error {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return err
	}
	if _, err := ValidateBridgeID(sessionID, "sessionId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST /v1/environments/%s/bridge/reconnect session_id=%s", environmentID, sessionID))

	status, data, err := c.withOAuthRetry(func(token string) (int, json.RawMessage, error) {
		url := c.cfg.BaseURL + "/v1/environments/" + environmentID + "/bridge/reconnect"
		return c.doJSON(context.Background(), http.MethodPost, url, map[string]any{"session_id": sessionID}, c.headers(token), defaultTimeout)
	}, "ReconnectSession")
	if err != nil {
		return err
	}

	if err := handleErrorStatus(status, data, "ReconnectSession"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] POST .../bridge/reconnect -> %d", status))
	return nil
}

func (c *bridgeAPIClient) HeartbeatWork(environmentID string, workID string, sessionToken string) (*HeartbeatResponse, error) {
	if _, err := ValidateBridgeID(environmentID, "environmentId"); err != nil {
		return nil, err
	}
	if _, err := ValidateBridgeID(workID, "workId"); err != nil {
		return nil, err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/heartbeat", workID))

	url := c.cfg.BaseURL + "/v1/environments/" + environmentID + "/work/" + workID + "/heartbeat"
	status, data, err := c.doJSON(context.Background(), http.MethodPost, url, map[string]any{}, c.headers(sessionToken), defaultTimeout)
	if err != nil {
		return nil, err
	}

	if err := handleErrorStatus(status, data, "Heartbeat"); err != nil {
		return nil, err
	}

	var resp HeartbeatResponse
	if err := json.Unmarshal(data, &resp); err != nil {
		return nil, fmt.Errorf("Heartbeat: invalid response body: %w", err)
	}
	c.debug(fmt.Sprintf("[bridge:api] POST .../work/%s/heartbeat -> %d lease_extended=%v state=%s", workID, status, resp.LeaseExtended, resp.State))
	return &resp, nil
}

func (c *bridgeAPIClient) SendPermissionResponseEvent(sessionID string, event PermissionResponseEvent, sessionToken string) error {
	if _, err := ValidateBridgeID(sessionID, "sessionId"); err != nil {
		return err
	}

	c.debug(fmt.Sprintf("[bridge:api] POST /v1/sessions/%s/events type=%s", sessionID, event.Type))

	url := c.cfg.BaseURL + "/v1/sessions/" + sessionID + "/events"
	status, data, err := c.doJSON(context.Background(), http.MethodPost, url, map[string]any{"events": []PermissionResponseEvent{event}}, c.headers(sessionToken), defaultTimeout)
	if err != nil {
		return err
	}

	if err := handleErrorStatus(status, data, "SendPermissionResponseEvent"); err != nil {
		return err
	}
	c.debug(fmt.Sprintf("[bridge:api] POST /v1/sessions/%s/events -> %d", sessionID, status))
	return nil
}

// ---------------------------------------------------------------------------
// NewBridgeAPIClientFromConfig — convenience constructor from BridgeConfig
// ---------------------------------------------------------------------------

// NewBridgeAPIClientFromConfig creates a BridgeAPIClient wired to the
// base URL and bridge ID from a BridgeConfig. The accessToken callback
// and optional debug hook are injected by the caller; the runner version
// defaults to "gopher-code/0.1" when empty.
func NewBridgeAPIClientFromConfig(cfg BridgeConfig, getAccessToken func() string, onDebug func(string)) BridgeAPIClient {
	runnerVersion := "gopher-code/0.1"
	return NewBridgeAPIClient(BridgeAPIClientConfig{
		BaseURL:        cfg.APIBaseURL,
		RunnerVersion:  runnerVersion,
		GetAccessToken: getAccessToken,
		OnDebug:        onDebug,
	})
}
