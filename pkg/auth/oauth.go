package auth

import (
	"bytes"
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

// Source: services/oauth/crypto.ts, services/oauth/index.ts, services/oauth/auth-code-listener.ts

// OAuthConfig holds the OAuth 2.0 configuration.
type OAuthConfig struct {
	AuthURL     string // Authorization endpoint
	TokenURL    string // Token exchange endpoint
	ClientID    string
	RedirectURI string // Set dynamically based on listener port
	Scopes      []string
}

// OAuthTokens holds the token response.
// Source: services/oauth/types.ts
type OAuthTokens struct {
	AccessToken  string `json:"access_token"`
	RefreshToken string `json:"refresh_token,omitempty"`
	ExpiresIn    int    `json:"expires_in,omitempty"`
	TokenType    string `json:"token_type"`
	Scope        string `json:"scope,omitempty"`
}

// PKCEParams holds the PKCE challenge parameters.
type PKCEParams struct {
	CodeVerifier  string
	CodeChallenge string
	State         string
}

// GenerateCodeVerifier creates a random PKCE code verifier.
// Source: services/oauth/crypto.ts:11-13
func GenerateCodeVerifier() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64URLEncode(b)
}

// GenerateCodeChallenge creates a PKCE code challenge from a verifier.
// Source: services/oauth/crypto.ts:15-19
func GenerateCodeChallenge(verifier string) string {
	h := sha256.Sum256([]byte(verifier))
	return base64URLEncode(h[:])
}

// GenerateState creates a random state parameter.
// Source: services/oauth/crypto.ts:21-23
func GenerateState() string {
	b := make([]byte, 32)
	rand.Read(b)
	return base64URLEncode(b)
}

// base64URLEncode encodes bytes as URL-safe base64 without padding.
// Source: services/oauth/crypto.ts:3-9
func base64URLEncode(b []byte) string {
	s := base64.StdEncoding.EncodeToString(b)
	s = strings.ReplaceAll(s, "+", "-")
	s = strings.ReplaceAll(s, "/", "_")
	s = strings.TrimRight(s, "=")
	return s
}

// NewPKCEParams generates a fresh set of PKCE parameters.
func NewPKCEParams() PKCEParams {
	verifier := GenerateCodeVerifier()
	return PKCEParams{
		CodeVerifier:  verifier,
		CodeChallenge: GenerateCodeChallenge(verifier),
		State:         GenerateState(),
	}
}

// BuildAuthURL constructs the OAuth authorization URL.
// Source: services/oauth/client.ts — buildAuthUrl
func BuildAuthURL(cfg OAuthConfig, pkce PKCEParams) string {
	params := url.Values{
		"response_type":         {"code"},
		"client_id":             {cfg.ClientID},
		"redirect_uri":          {cfg.RedirectURI},
		"state":                 {pkce.State},
		"code_challenge":        {pkce.CodeChallenge},
		"code_challenge_method": {"S256"},
	}
	if len(cfg.Scopes) > 0 {
		params.Set("scope", strings.Join(cfg.Scopes, " "))
	}
	return cfg.AuthURL + "?" + params.Encode()
}

// AuthCodeListener listens for the OAuth callback on localhost.
// Source: services/oauth/auth-code-listener.ts
type AuthCodeListener struct {
	server        *http.Server
	listener      net.Listener
	codeCh        chan string
	errCh         chan error
	mu            sync.RWMutex
	expectedState string // State parameter for CSRF protection
}

// NewAuthCodeListener creates a new callback listener.
func NewAuthCodeListener() *AuthCodeListener {
	return &AuthCodeListener{
		codeCh: make(chan string, 1),
		errCh:  make(chan error, 1),
	}
}

// Start begins listening on a random port. Returns the port.
// Source: services/oauth/auth-code-listener.ts — start()
func (l *AuthCodeListener) Start() (int, error) {
	listener, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		return 0, fmt.Errorf("listen: %w", err)
	}
	l.listener = listener
	port := listener.Addr().(*net.TCPAddr).Port

	mux := http.NewServeMux()
	mux.HandleFunc("/callback", l.handleCallbackWithState)

	l.server = &http.Server{Handler: mux}
	go l.server.Serve(listener)

	return port, nil
}

// WaitForCode waits for the authorization code with a timeout.
func (l *AuthCodeListener) WaitForCode(ctx context.Context, timeout time.Duration) (string, error) {
	ctx, cancel := context.WithTimeout(ctx, timeout)
	defer cancel()

	select {
	case code := <-l.codeCh:
		return code, nil
	case err := <-l.errCh:
		return "", err
	case <-ctx.Done():
		return "", fmt.Errorf("OAuth authorization timed out")
	}
}

// Stop shuts down the listener.
func (l *AuthCodeListener) Stop() {
	if l.server != nil {
		l.server.Close()
	}
}

// Port returns the port the listener is running on.
func (l *AuthCodeListener) Port() int {
	if l.listener == nil {
		return 0
	}
	return l.listener.Addr().(*net.TCPAddr).Port
}

// WaitForCodeWithState waits for the authorization code, validating the state parameter.
// Source: services/oauth/auth-code-listener.ts — state validation + CSRF protection
func (l *AuthCodeListener) WaitForCodeWithState(ctx context.Context, timeout time.Duration, expectedState string) (string, error) {
	l.mu.Lock()
	l.expectedState = expectedState
	l.mu.Unlock()
	return l.WaitForCode(ctx, timeout)
}

// handleCallbackWithState processes the OAuth redirect with state validation.
func (l *AuthCodeListener) handleCallbackWithState(w http.ResponseWriter, r *http.Request) {
	code := r.URL.Query().Get("code")
	state := r.URL.Query().Get("state")

	if code == "" {
		errMsg := r.URL.Query().Get("error")
		if errMsg == "" {
			errMsg = "no authorization code received"
		}
		l.errCh <- fmt.Errorf("OAuth error: %s", errMsg)
		http.Error(w, "Authorization failed", http.StatusBadRequest)
		return
	}

	// Validate state if expected state is set
	l.mu.RLock()
	expected := l.expectedState
	l.mu.RUnlock()
	if expected != "" && state != expected {
		l.errCh <- fmt.Errorf("invalid state parameter: expected %q, got %q", expected, state)
		http.Error(w, "Invalid state parameter", http.StatusBadRequest)
		return
	}

	l.codeCh <- code
	w.Header().Set("Content-Type", "text/html")
	fmt.Fprint(w, `<html><body><h1>Authorization successful!</h1><p>You can close this window and return to the terminal.</p></body></html>`)
}

// ShouldUseClaudeAIAuth checks if the user has Claude.ai authentication scope.
// Source: services/oauth/client.ts — shouldUseClaudeAIAuth
func ShouldUseClaudeAIAuth(scopes []string) bool {
	for _, s := range scopes {
		if s == ClaudeAIInferenceScope {
			return true
		}
	}
	return false
}

// ParseScopes splits a space-separated scope string into individual scopes.
// Source: services/oauth/client.ts — parseScopes
func ParseScopes(scopeString string) []string {
	if scopeString == "" {
		return nil
	}
	parts := strings.Split(scopeString, " ")
	var result []string
	for _, p := range parts {
		if p != "" {
			result = append(result, p)
		}
	}
	return result
}

// IsOAuthTokenExpired checks if a token is expired or will expire within the buffer.
// Source: services/oauth/client.ts — isOAuthTokenExpired
// expiresAt is in milliseconds since epoch. 0 means no expiry (null equivalent).
// Buffer: 5 minutes.
func IsOAuthTokenExpired(expiresAtMs int64) bool {
	if expiresAtMs == 0 {
		return false
	}
	const bufferMs = 5 * 60 * 1000 // 5 minutes in milliseconds
	nowMs := time.Now().UnixMilli()
	return (nowMs + bufferMs) >= expiresAtMs
}

// ExchangeCodeParams holds parameters for the authorization code exchange.
type ExchangeCodeParams struct {
	TokenURL     string
	ClientID     string
	Code         string
	CodeVerifier string
	RedirectURI  string
	State        string
	ExpiresIn    int `json:"expires_in,omitempty"` // optional custom expiry
}

// ExchangeCodeForTokens exchanges an authorization code for tokens.
// Source: services/oauth/client.ts — exchangeCodeForTokens
func ExchangeCodeForTokens(params ExchangeCodeParams) (*OAuthTokens, error) {
	body := map[string]interface{}{
		"grant_type":    "authorization_code",
		"code":          params.Code,
		"redirect_uri":  params.RedirectURI,
		"client_id":     params.ClientID,
		"code_verifier": params.CodeVerifier,
		"state":         params.State,
	}
	if params.ExpiresIn > 0 {
		body["expires_in"] = params.ExpiresIn
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, params.TokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token exchange request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode == http.StatusUnauthorized {
		return nil, fmt.Errorf("authentication failed: invalid authorization code")
	}
	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token exchange failed (%d): %s", resp.StatusCode, resp.Status)
	}

	var tokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}
	return &tokens, nil
}

// RefreshTokenParams holds parameters for the token refresh request.
type RefreshTokenParams struct {
	TokenURL     string
	ClientID     string
	RefreshToken string
	Scopes       []string // optional; defaults to ClaudeAIOAuthScopes
}

// RefreshOAuthToken refreshes an OAuth token using a refresh token.
// Source: services/oauth/client.ts — refreshOAuthToken
func RefreshOAuthToken(params RefreshTokenParams) (*OAuthTokens, error) {
	scopes := params.Scopes
	if len(scopes) == 0 {
		scopes = ClaudeAIOAuthScopes
	}

	body := map[string]interface{}{
		"grant_type":    "refresh_token",
		"refresh_token": params.RefreshToken,
		"client_id":     params.ClientID,
		"scope":         strings.Join(scopes, " "),
	}

	jsonBody, err := json.Marshal(body)
	if err != nil {
		return nil, fmt.Errorf("marshal request: %w", err)
	}

	req, err := http.NewRequest(http.MethodPost, params.TokenURL, bytes.NewReader(jsonBody))
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 15 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("token refresh request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("token refresh failed (%d): %s", resp.StatusCode, resp.Status)
	}

	var tokens OAuthTokens
	if err := json.NewDecoder(resp.Body).Decode(&tokens); err != nil {
		return nil, fmt.Errorf("decode token response: %w", err)
	}

	// If server didn't return a new refresh token, keep the old one
	if tokens.RefreshToken == "" {
		tokens.RefreshToken = params.RefreshToken
	}

	return &tokens, nil
}

// OAuthProfileAccount holds account info from the profile response.
// Source: services/oauth/types.ts
type OAuthProfileAccount struct {
	UUID        string `json:"uuid"`
	Email       string `json:"email"`
	DisplayName string `json:"display_name,omitempty"`
	CreatedAt   string `json:"created_at,omitempty"`
}

// OAuthProfileOrganization holds organization info from the profile response.
type OAuthProfileOrganization struct {
	UUID                    string  `json:"uuid"`
	OrganizationType        string  `json:"organization_type,omitempty"`
	RateLimitTier           *string `json:"rate_limit_tier,omitempty"`
	HasExtraUsageEnabled    *bool   `json:"has_extra_usage_enabled,omitempty"`
	BillingType             *string `json:"billing_type,omitempty"`
	SubscriptionCreatedAt   string  `json:"subscription_created_at,omitempty"`
}

// OAuthProfileResponse is the response from the profile endpoints.
// Source: services/oauth/types.ts — OAuthProfileResponse
type OAuthProfileResponse struct {
	Account      OAuthProfileAccount      `json:"account"`
	Organization OAuthProfileOrganization `json:"organization"`
}

// FetchOAuthProfileFromOAuthToken fetches the OAuth profile using a Bearer token.
// Source: services/oauth/getOauthProfile.ts — getOauthProfileFromOauthToken
// Endpoint: ${baseAPIURL}/api/oauth/profile
func FetchOAuthProfileFromOAuthToken(baseAPIURL string, accessToken string) (*OAuthProfileResponse, error) {
	endpoint := baseAPIURL + "/api/oauth/profile"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("Authorization", "Bearer "+accessToken)
	req.Header.Set("Content-Type", "application/json")

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile request failed (%d): %s", resp.StatusCode, resp.Status)
	}

	var profile OAuthProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}
	return &profile, nil
}

// FetchOAuthProfileFromAPIKey fetches the OAuth profile using an API key.
// Source: services/oauth/getOauthProfile.ts — getOauthProfileFromApiKey
// Endpoint: ${baseAPIURL}/api/claude_cli_profile?account_uuid={uuid}
func FetchOAuthProfileFromAPIKey(baseAPIURL string, apiKey string, accountUUID string) (*OAuthProfileResponse, error) {
	endpoint := baseAPIURL + "/api/claude_cli_profile"

	req, err := http.NewRequest(http.MethodGet, endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}
	req.Header.Set("x-api-key", apiKey)
	req.Header.Set("anthropic-beta", OAuthBetaHeader)

	q := req.URL.Query()
	q.Set("account_uuid", accountUUID)
	req.URL.RawQuery = q.Encode()

	client := &http.Client{Timeout: 10 * time.Second}
	resp, err := client.Do(req)
	if err != nil {
		return nil, fmt.Errorf("profile request: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return nil, fmt.Errorf("profile request failed (%d): %s", resp.StatusCode, resp.Status)
	}

	var profile OAuthProfileResponse
	if err := json.NewDecoder(resp.Body).Decode(&profile); err != nil {
		return nil, fmt.Errorf("decode profile response: %w", err)
	}
	return &profile, nil
}

// ResolveSubscriptionType maps an organization_type to a subscription type string.
// Source: services/oauth/client.ts — fetchProfileInfo switch
func ResolveSubscriptionType(orgType string) string {
	switch orgType {
	case "claude_max":
		return "max"
	case "claude_pro":
		return "pro"
	case "claude_enterprise":
		return "enterprise"
	case "claude_team":
		return "team"
	default:
		return ""
	}
}
