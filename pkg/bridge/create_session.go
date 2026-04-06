// Package bridge — org-scoped HTTP wrappers for the CCR Sessions API (/v1/sessions*).
// Source: src/bridge/createSession.ts
package bridge

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"strings"
	"time"

	"github.com/hashicorp/go-retryablehttp"
)

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// SessionsBetaHeader is the anthropic-beta value for org-scoped session calls.
// Distinct from BetaHeader (environments API).
const SessionsBetaHeader = "ccr-byoc-2025-07-29"

// sessionSource is the constant tag the server routes on.
const sessionSource = "remote-control"

// sessionTimeout is the default HTTP timeout for session API calls.
const sessionTimeout = 10 * time.Second

// ---------------------------------------------------------------------------
// Types — GitSource / GitOutcome / SessionEvent
// ---------------------------------------------------------------------------

// GitSource describes a git repository source for a session.
type GitSource struct {
	Type     string `json:"type"`
	URL      string `json:"url"`
	Revision string `json:"revision,omitempty"`
}

// GitOutcome describes the expected git outcome of a session.
type GitOutcome struct {
	Type    string        `json:"type"`
	GitInfo GitOutcomeInfo `json:"git_info"`
}

// GitOutcomeInfo holds the github-specific info inside a GitOutcome.
type GitOutcomeInfo struct {
	Type     string   `json:"type"`
	Repo     string   `json:"repo"`
	Branches []string `json:"branches"`
}

// SessionEvent wraps an SDK message for the POST /v1/sessions endpoint.
type SessionEvent struct {
	Type string `json:"type"`
	Data any    `json:"data"`
}

// SessionContext is the session_context field in the create-session request.
type SessionContext struct {
	Sources  []GitSource  `json:"sources"`
	Outcomes []GitOutcome `json:"outcomes"`
	Model    string       `json:"model,omitempty"`
}

// ---------------------------------------------------------------------------
// SessionClientConfig — constructor dependencies
// ---------------------------------------------------------------------------

// SessionClientConfig holds dependencies for NewSessionClient.
type SessionClientConfig struct {
	// BaseURL override; empty string falls back to GetBaseAPIURL.
	BaseURL string

	// GetAccessToken returns the OAuth access token. Nil return means not logged in.
	GetAccessToken func() string

	// GetOrgUUID returns the organization UUID. Empty return means unavailable.
	GetOrgUUID func() string

	// GetModel returns the current main-loop model name.
	GetModel func() string

	// OnDebug is called with debug log messages.
	OnDebug func(msg string)
}

// ---------------------------------------------------------------------------
// sessionClient
// ---------------------------------------------------------------------------

type sessionClient struct {
	cfg        SessionClientConfig
	httpClient *retryablehttp.Client
}

// NewSessionClient creates a client for org-scoped session API calls.
func NewSessionClient(cfg SessionClientConfig) *sessionClient {
	rc := retryablehttp.NewClient()
	rc.RetryMax = 2
	rc.Logger = nil
	rc.CheckRetry = retryablehttp.ErrorPropagatedRetryPolicy

	return &sessionClient{cfg: cfg, httpClient: rc}
}

func (c *sessionClient) debug(msg string) {
	if c.cfg.OnDebug != nil {
		c.cfg.OnDebug(msg)
	}
}

func (c *sessionClient) baseURL() string {
	if c.cfg.BaseURL != "" {
		return c.cfg.BaseURL
	}
	return "https://api.anthropic.com"
}

func (c *sessionClient) resolveToken() (string, bool) {
	if c.cfg.GetAccessToken == nil {
		return "", false
	}
	tok := c.cfg.GetAccessToken()
	if tok == "" {
		return "", false
	}
	return tok, true
}

func (c *sessionClient) resolveOrg() (string, bool) {
	if c.cfg.GetOrgUUID == nil {
		return "", false
	}
	org := c.cfg.GetOrgUUID()
	if org == "" {
		return "", false
	}
	return org, true
}

func (c *sessionClient) headers(accessToken, orgUUID string) map[string]string {
	return map[string]string{
		"Authorization":        "Bearer " + accessToken,
		"Content-Type":         "application/json",
		"anthropic-beta":       SessionsBetaHeader,
		"x-organization-uuid":  orgUUID,
	}
}

// doJSON performs a JSON request accepting status < 500 (matching TS validateStatus).
func (c *sessionClient) doJSON(ctx context.Context, method, url string, body any, headers map[string]string, timeout time.Duration) (*http.Response, []byte, error) {
	var bodyReader *strings.Reader
	if body != nil {
		b, err := json.Marshal(body)
		if err != nil {
			return nil, nil, err
		}
		bodyReader = strings.NewReader(string(b))
	}

	var req *retryablehttp.Request
	var err error
	if bodyReader != nil {
		req, err = retryablehttp.NewRequestWithContext(ctx, method, url, bodyReader)
	} else {
		req, err = retryablehttp.NewRequestWithContext(ctx, method, url, nil)
	}
	if err != nil {
		return nil, nil, err
	}
	for k, v := range headers {
		req.Header.Set(k, v)
	}

	tctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()
	req.Request = req.Request.WithContext(tctx)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return nil, nil, err
	}
	defer resp.Body.Close()

	data, err := readBody(resp)
	if err != nil {
		return resp, nil, err
	}
	return resp, data, nil
}

func readBody(resp *http.Response) ([]byte, error) {
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, err
	}
	return data, nil
}

// ---------------------------------------------------------------------------
// extractSessionErrorDetail — same logic as TS debugUtils.ts extractErrorDetail
// ---------------------------------------------------------------------------

func extractSessionErrorDetail(data []byte) string {
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

// ---------------------------------------------------------------------------
// CreateBridgeSession
// ---------------------------------------------------------------------------

// CreateBridgeSessionOpts configures a CreateBridgeSession call.
type CreateBridgeSessionOpts struct {
	EnvironmentID  string
	Title          *string // nil = omit from request
	Events         []SessionEvent
	GitRepoURL     string // empty = no git context
	Branch         string
	PermissionMode string // empty = omit
}

// ParsedGitRemote is a parsed git remote URL.
type ParsedGitRemote struct {
	Host  string
	Owner string
	Name  string
}

// ParseGitRemote parses a git remote URL into host/owner/name.
// Returns nil if the URL cannot be parsed.
func ParseGitRemote(url string) *ParsedGitRemote {
	// Handle SSH format: git@host:owner/name.git
	if strings.HasPrefix(url, "git@") {
		rest := url[len("git@"):]
		colonIdx := strings.Index(rest, ":")
		if colonIdx < 0 {
			return nil
		}
		host := rest[:colonIdx]
		path := rest[colonIdx+1:]
		path = strings.TrimSuffix(path, ".git")
		parts := strings.SplitN(path, "/", 2)
		if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
			return nil
		}
		return &ParsedGitRemote{Host: host, Owner: parts[0], Name: parts[1]}
	}

	// Handle HTTPS format: https://host/owner/name.git
	for _, prefix := range []string{"https://", "http://"} {
		if strings.HasPrefix(url, prefix) {
			rest := url[len(prefix):]
			rest = strings.TrimSuffix(rest, ".git")
			parts := strings.SplitN(rest, "/", 3)
			if len(parts) < 3 || parts[1] == "" || parts[2] == "" {
				return nil
			}
			return &ParsedGitRemote{Host: parts[0], Owner: parts[1], Name: parts[2]}
		}
	}

	return nil
}

// ParseGitHubRepository parses an "owner/repo" string.
// Returns ("owner", "repo", true) on success.
func ParseGitHubRepository(s string) (string, string, bool) {
	parts := strings.SplitN(s, "/", 2)
	if len(parts) != 2 || parts[0] == "" || parts[1] == "" {
		return "", "", false
	}
	// Reject if it looks like a URL
	if strings.Contains(parts[0], ":") || strings.Contains(parts[0], ".") {
		return "", "", false
	}
	return parts[0], parts[1], true
}

// buildGitContext constructs GitSource and GitOutcome from a repo URL and branch.
func buildGitContext(gitRepoURL, branch string) (*GitSource, *GitOutcome) {
	if gitRepoURL == "" {
		return nil, nil
	}

	branchOrTask := branch
	if branchOrTask == "" {
		branchOrTask = "task"
	}

	// Try parsing as a full git remote URL
	parsed := ParseGitRemote(gitRepoURL)
	if parsed != nil {
		revision := branch
		src := &GitSource{
			Type:     "git_repository",
			URL:      fmt.Sprintf("https://%s/%s/%s", parsed.Host, parsed.Owner, parsed.Name),
			Revision: revision,
		}
		out := &GitOutcome{
			Type: "git_repository",
			GitInfo: GitOutcomeInfo{
				Type:     "github",
				Repo:     parsed.Owner + "/" + parsed.Name,
				Branches: []string{"claude/" + branchOrTask},
			},
		}
		return src, out
	}

	// Fallback: try owner/repo format
	owner, name, ok := ParseGitHubRepository(gitRepoURL)
	if ok {
		revision := branch
		src := &GitSource{
			Type:     "git_repository",
			URL:      fmt.Sprintf("https://github.com/%s/%s", owner, name),
			Revision: revision,
		}
		out := &GitOutcome{
			Type: "git_repository",
			GitInfo: GitOutcomeInfo{
				Type:     "github",
				Repo:     owner + "/" + name,
				Branches: []string{"claude/" + branchOrTask},
			},
		}
		return src, out
	}

	return nil, nil
}

// CreateBridgeSession creates a session via POST /v1/sessions.
// Returns the session ID on success, or ("", error).
func (c *sessionClient) CreateBridgeSession(ctx context.Context, opts CreateBridgeSessionOpts) (string, error) {
	accessToken, ok := c.resolveToken()
	if !ok {
		c.debug("[bridge] No access token for session creation")
		return "", fmt.Errorf("[bridge] No access token for session creation")
	}

	orgUUID, ok := c.resolveOrg()
	if !ok {
		c.debug("[bridge] No org UUID for session creation")
		return "", fmt.Errorf("[bridge] No org UUID for session creation")
	}

	gitSource, gitOutcome := buildGitContext(opts.GitRepoURL, opts.Branch)

	var sources []GitSource
	var outcomes []GitOutcome
	if gitSource != nil {
		sources = []GitSource{*gitSource}
	} else {
		sources = []GitSource{}
	}
	if gitOutcome != nil {
		outcomes = []GitOutcome{*gitOutcome}
	} else {
		outcomes = []GitOutcome{}
	}

	model := ""
	if c.cfg.GetModel != nil {
		model = c.cfg.GetModel()
	}

	reqBody := map[string]any{
		"events": opts.Events,
		"session_context": SessionContext{
			Sources:  sources,
			Outcomes: outcomes,
			Model:    model,
		},
		"environment_id": opts.EnvironmentID,
		"source":         sessionSource,
	}
	if opts.Title != nil {
		reqBody["title"] = *opts.Title
	}
	if opts.PermissionMode != "" {
		reqBody["permission_mode"] = opts.PermissionMode
	}

	url := c.baseURL() + "/v1/sessions"
	hdrs := c.headers(accessToken, orgUUID)

	resp, data, err := c.doJSON(ctx, http.MethodPost, url, reqBody, hdrs, sessionTimeout)
	if err != nil {
		c.debug(fmt.Sprintf("[bridge] Session creation request failed: %s", err.Error()))
		return "", fmt.Errorf("[bridge] Session creation request failed: %w", err)
	}

	if resp.StatusCode != 200 && resp.StatusCode != 201 {
		detail := extractSessionErrorDetail(data)
		msg := fmt.Sprintf("[bridge] Session creation failed with status %d", resp.StatusCode)
		if detail != "" {
			msg += ": " + detail
		}
		c.debug(msg)
		return "", fmt.Errorf("%s", msg)
	}

	var result struct {
		ID string `json:"id"`
	}
	if err := json.Unmarshal(data, &result); err != nil || result.ID == "" {
		c.debug("[bridge] No session ID in response")
		return "", fmt.Errorf("[bridge] No session ID in response")
	}

	return result.ID, nil
}

// ---------------------------------------------------------------------------
// GetBridgeSession
// ---------------------------------------------------------------------------

// BridgeSessionInfo is the response from GET /v1/sessions/{id}.
type BridgeSessionInfo struct {
	EnvironmentID string `json:"environment_id,omitempty"`
	Title         string `json:"title,omitempty"`
}

// GetBridgeSession fetches a session via GET /v1/sessions/{id}.
// Returns nil on failure (non-fatal).
func (c *sessionClient) GetBridgeSession(ctx context.Context, sessionID string) (*BridgeSessionInfo, error) {
	accessToken, ok := c.resolveToken()
	if !ok {
		c.debug("[bridge] No access token for session fetch")
		return nil, fmt.Errorf("[bridge] No access token for session fetch")
	}

	orgUUID, ok := c.resolveOrg()
	if !ok {
		c.debug("[bridge] No org UUID for session fetch")
		return nil, fmt.Errorf("[bridge] No org UUID for session fetch")
	}

	hdrs := c.headers(accessToken, orgUUID)
	url := c.baseURL() + "/v1/sessions/" + sessionID
	c.debug(fmt.Sprintf("[bridge] Fetching session %s", sessionID))

	resp, data, err := c.doJSON(ctx, http.MethodGet, url, nil, hdrs, sessionTimeout)
	if err != nil {
		c.debug(fmt.Sprintf("[bridge] Session fetch request failed: %s", err.Error()))
		return nil, fmt.Errorf("[bridge] Session fetch request failed: %w", err)
	}

	if resp.StatusCode != 200 {
		detail := extractSessionErrorDetail(data)
		msg := fmt.Sprintf("[bridge] Session fetch failed with status %d", resp.StatusCode)
		if detail != "" {
			msg += ": " + detail
		}
		c.debug(msg)
		return nil, fmt.Errorf("%s", msg)
	}

	var info BridgeSessionInfo
	if err := json.Unmarshal(data, &info); err != nil {
		return nil, fmt.Errorf("[bridge] invalid session response: %w", err)
	}
	return &info, nil
}

// ---------------------------------------------------------------------------
// ArchiveBridgeSession
// ---------------------------------------------------------------------------

// ArchiveBridgeSession archives a session via POST /v1/sessions/{id}/archive.
// Returns nil on success or 409 (already archived). Returns error on 5xx/network.
func (c *sessionClient) ArchiveBridgeSession(ctx context.Context, sessionID string, timeoutMs *int) error {
	accessToken, ok := c.resolveToken()
	if !ok {
		c.debug("[bridge] No access token for session archive")
		return nil // non-fatal, match TS behavior
	}

	orgUUID, ok := c.resolveOrg()
	if !ok {
		c.debug("[bridge] No org UUID for session archive")
		return nil // non-fatal
	}

	hdrs := c.headers(accessToken, orgUUID)
	timeout := sessionTimeout
	if timeoutMs != nil {
		timeout = time.Duration(*timeoutMs) * time.Millisecond
	}

	url := c.baseURL() + "/v1/sessions/" + sessionID + "/archive"
	c.debug(fmt.Sprintf("[bridge] Archiving session %s", sessionID))

	resp, data, err := c.doJSON(ctx, http.MethodPost, url, map[string]any{}, hdrs, timeout)
	if err != nil {
		return fmt.Errorf("[bridge] Session archive request failed: %w", err)
	}

	if resp.StatusCode == 200 {
		c.debug(fmt.Sprintf("[bridge] Session %s archived successfully", sessionID))
		return nil
	}

	// 409 = already archived (idempotent)
	if resp.StatusCode == 409 {
		return nil
	}

	detail := extractSessionErrorDetail(data)
	msg := fmt.Sprintf("[bridge] Session archive failed with status %d", resp.StatusCode)
	if detail != "" {
		msg += ": " + detail
	}
	c.debug(msg)
	return fmt.Errorf("%s", msg)
}

// ---------------------------------------------------------------------------
// UpdateBridgeSessionTitle
// ---------------------------------------------------------------------------

// UpdateBridgeSessionTitle patches the title via PATCH /v1/sessions/{id}.
// Errors are returned but callers typically ignore them (best-effort).
func (c *sessionClient) UpdateBridgeSessionTitle(ctx context.Context, sessionID, title string) error {
	accessToken, ok := c.resolveToken()
	if !ok {
		c.debug("[bridge] No access token for session title update")
		return nil // non-fatal
	}

	orgUUID, ok := c.resolveOrg()
	if !ok {
		c.debug("[bridge] No org UUID for session title update")
		return nil // non-fatal
	}

	hdrs := c.headers(accessToken, orgUUID)
	compatID := ToCompatSessionID(sessionID)
	url := c.baseURL() + "/v1/sessions/" + compatID
	c.debug(fmt.Sprintf("[bridge] Updating session title: %s → %s", compatID, title))

	resp, data, err := c.doJSON(ctx, http.MethodPatch, url, map[string]any{"title": title}, hdrs, sessionTimeout)
	if err != nil {
		c.debug(fmt.Sprintf("[bridge] Session title update request failed: %s", err.Error()))
		return fmt.Errorf("[bridge] Session title update request failed: %w", err)
	}

	if resp.StatusCode == 200 {
		c.debug("[bridge] Session title updated successfully")
		return nil
	}

	detail := extractSessionErrorDetail(data)
	msg := fmt.Sprintf("[bridge] Session title update failed with status %d", resp.StatusCode)
	if detail != "" {
		msg += ": " + detail
	}
	c.debug(msg)
	return fmt.Errorf("%s", msg)
}

