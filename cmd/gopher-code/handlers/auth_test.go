package handlers

import (
	"bytes"
	"encoding/json"
	"fmt"
	"os"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/zalando/go-keyring"
)

func TestMain(m *testing.M) {
	keyring.MockInit()
	os.Exit(m.Run())
}

// mockKeyring is a test-only in-memory keyring.
type mockKeyring struct {
	store map[string]string
}

func newMockKeyring() *mockKeyring {
	return &mockKeyring{store: make(map[string]string)}
}

func (m *mockKeyring) Get(service, key string) (string, error) {
	v, ok := m.store[service+"/"+key]
	if !ok {
		return "", fmt.Errorf("secret not found in keyring")
	}
	return v, nil
}

func (m *mockKeyring) Set(service, key, value string) error {
	m.store[service+"/"+key] = value
	return nil
}

func (m *mockKeyring) Delete(service, key string) error {
	k := service + "/" + key
	if _, ok := m.store[k]; !ok {
		return fmt.Errorf("secret not found in keyring")
	}
	delete(m.store, k)
	return nil
}

// mockOAuthFlow is a test-only OAuth flow starter.
type mockOAuthFlow struct {
	tokens *auth.OAuthTokens
	err    error
}

func (m *mockOAuthFlow) StartOAuthFlow(email string, scopes []string, useClaudeAI bool) (*auth.OAuthTokens, error) {
	if m.err != nil {
		return nil, m.err
	}
	return m.tokens, nil
}

// --- Tests ---

func TestAuthStatus_NotLoggedIn(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthStatus(AuthStatusOpts{Output: &buf})

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	got := buf.String()
	if got != "Not logged in. Run claude auth login to authenticate.\n" {
		t.Errorf("unexpected output: %q", got)
	}
}

func TestAuthStatus_NotLoggedIn_JSON(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthStatus(AuthStatusOpts{JSON: true, Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0 for JSON output, got %d", code)
	}

	var out AuthStatusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if out.LoggedIn {
		t.Error("expected loggedIn=false")
	}
	if out.AuthMethod != "none" {
		t.Errorf("expected authMethod=none, got %s", out.AuthMethod)
	}
	if out.APIProvider != "anthropic" {
		t.Errorf("expected apiProvider=anthropic, got %s", out.APIProvider)
	}
}

func TestAuthStatus_APIKeyEnv(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthStatus(AuthStatusOpts{JSON: true, Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	var out AuthStatusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !out.LoggedIn {
		t.Error("expected loggedIn=true")
	}
	if out.AuthMethod != "api_key" {
		t.Errorf("expected authMethod=api_key, got %s", out.AuthMethod)
	}
	if out.APIKeySource != "ANTHROPIC_API_KEY" {
		t.Errorf("expected apiKeySource=ANTHROPIC_API_KEY, got %s", out.APIKeySource)
	}
}

func TestAuthStatus_APIKeyEnv_TextOutput(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "sk-ant-test-key")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthStatus(AuthStatusOpts{Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	got := buf.String()
	if !strings.Contains(got, "API key source: ANTHROPIC_API_KEY") {
		t.Errorf("expected API key source in text output, got: %q", got)
	}
}

func TestAuthStatus_OAuthToken(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	mk := newMockKeyring()
	mk.Set(auth.KeyringService, keyringOAuthAccess, "test-access-token")
	mk.Set(auth.KeyringService, keyringAccountEmail, "user@example.com")
	mk.Set(auth.KeyringService, keyringOrgUUID, "org-123")
	mk.Set(auth.KeyringService, keyringSubType, "pro")
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthStatus(AuthStatusOpts{JSON: true, Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	var out AuthStatusOutput
	if err := json.Unmarshal(buf.Bytes(), &out); err != nil {
		t.Fatalf("failed to parse JSON: %v", err)
	}
	if !out.LoggedIn {
		t.Error("expected loggedIn=true")
	}
	if out.Email != "user@example.com" {
		t.Errorf("expected email user@example.com, got %s", out.Email)
	}
	if out.OrgID != "org-123" {
		t.Errorf("expected orgId org-123, got %s", out.OrgID)
	}
	if out.SubscriptionType != "pro" {
		t.Errorf("expected subscriptionType pro, got %s", out.SubscriptionType)
	}
}

func TestAuthStatus_JSON_TwoSpaceIndent(t *testing.T) {
	t.Setenv("ANTHROPIC_API_KEY", "")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	AuthStatus(AuthStatusOpts{JSON: true, Output: &buf})

	got := buf.String()
	// Verify 2-space indent (not tabs, not 4-space)
	if !strings.Contains(got, "  \"loggedIn\"") {
		t.Errorf("expected 2-space indented JSON, got:\n%s", got)
	}
}

func TestAuthLogout_ClearsKeyring(t *testing.T) {
	mk := newMockKeyring()
	// Populate keyring with auth data
	mk.Set(auth.KeyringService, keyringOAuthAccess, "access-token")
	mk.Set(auth.KeyringService, keyringOAuthRefresh, "refresh-token")
	mk.Set(auth.KeyringService, keyringAccountEmail, "user@example.com")
	mk.Set(auth.KeyringService, keyringAccountUUID, "uuid-123")
	mk.Set(auth.KeyringService, keyringOrgUUID, "org-456")

	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthLogout(AuthLogoutOpts{Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	got := buf.String()
	if got != "Successfully logged out from your Anthropic account.\n" {
		t.Errorf("unexpected output: %q", got)
	}

	// Verify all keys were deleted
	for _, key := range []string{keyringOAuthAccess, keyringOAuthRefresh, keyringAccountEmail, keyringAccountUUID, keyringOrgUUID} {
		if _, err := mk.Get(auth.KeyringService, key); err == nil {
			t.Errorf("key %q should have been deleted", key)
		}
	}
}

func TestAuthLogout_SuccessMessage(t *testing.T) {
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	var buf bytes.Buffer
	code := AuthLogout(AuthLogoutOpts{Output: &buf})

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}
	if buf.String() != "Successfully logged out from your Anthropic account.\n" {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestAuthLogin_MutualExclusion(t *testing.T) {
	var buf bytes.Buffer
	code := AuthLogin(AuthLoginOpts{
		Console:  true,
		ClaudeAI: true,
		Output:   &buf,
	}, nil)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if buf.String() != "Error: --console and --claudeai cannot be used together.\n" {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestAuthLogin_EnvRefreshToken_MissingScopes(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "some-refresh-token")
	t.Setenv("CLAUDE_CODE_OAUTH_SCOPES", "")

	var buf bytes.Buffer
	code := AuthLogin(AuthLoginOpts{Output: &buf}, nil)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}

	expected := "CLAUDE_CODE_OAUTH_SCOPES is required when using CLAUDE_CODE_OAUTH_REFRESH_TOKEN.\nSet it to the space-separated scopes the refresh token was issued with\n(e.g. \"user:inference\" or \"user:profile user:inference user:sessions:claude_code user:mcp_servers\").\n"
	if buf.String() != expected {
		t.Errorf("unexpected output:\ngot:  %q\nwant: %q", buf.String(), expected)
	}
}

func TestAuthLogin_OAuthFlowSuccess(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "")
	mk := newMockKeyring()
	orig := DefaultKeyring
	DefaultKeyring = mk
	defer func() { DefaultKeyring = orig }()

	flow := &mockOAuthFlow{
		tokens: &auth.OAuthTokens{
			AccessToken:  "new-access-token",
			RefreshToken: "new-refresh-token",
		},
	}

	var buf bytes.Buffer
	code := AuthLogin(AuthLoginOpts{Output: &buf}, flow)

	if code != 0 {
		t.Errorf("expected exit code 0, got %d", code)
	}

	got := buf.String()
	if !strings.Contains(got, "Opening browser to sign in\u2026") {
		t.Errorf("expected browser opening message, got: %q", got)
	}
	if !strings.Contains(got, "Login successful.") {
		t.Errorf("expected success message, got: %q", got)
	}

	// Verify tokens stored
	if v, _ := mk.Get(auth.KeyringService, keyringOAuthAccess); v != "new-access-token" {
		t.Errorf("expected access token stored, got %q", v)
	}
	if v, _ := mk.Get(auth.KeyringService, keyringOAuthRefresh); v != "new-refresh-token" {
		t.Errorf("expected refresh token stored, got %q", v)
	}
}

func TestAuthLogin_OAuthFlowFailure(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "")

	flow := &mockOAuthFlow{
		err: fmt.Errorf("connection refused"),
	}

	var buf bytes.Buffer
	code := AuthLogin(AuthLoginOpts{Output: &buf}, flow)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(buf.String(), "Login failed: connection refused") {
		t.Errorf("unexpected output: %q", buf.String())
	}
}

func TestAuthLogin_OAuthFlowSSLError(t *testing.T) {
	t.Setenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN", "")

	flow := &mockOAuthFlow{
		err: fmt.Errorf("x509: certificate signed by unknown authority"),
	}

	var buf bytes.Buffer
	code := AuthLogin(AuthLoginOpts{Output: &buf}, flow)

	if code != 1 {
		t.Errorf("expected exit code 1, got %d", code)
	}
	got := buf.String()
	if !strings.Contains(got, "Login failed:") {
		t.Errorf("expected login failed message, got: %q", got)
	}
	if !strings.Contains(got, "Hint:") {
		t.Errorf("expected SSL hint, got: %q", got)
	}
}

func TestGetSSLErrorHint(t *testing.T) {
	tests := []struct {
		err    string
		hasHint bool
	}{
		{"x509: certificate signed by unknown authority", true},
		{"tls: handshake failure", true},
		{"certificate verify failed", true},
		{"SSL_ERROR_SYSCALL", true},
		{"connection refused", false},
		{"timeout", false},
	}
	for _, tt := range tests {
		hint := getSSLErrorHint(fmt.Errorf("%s", tt.err))
		if tt.hasHint && hint == "" {
			t.Errorf("expected hint for %q", tt.err)
		}
		if !tt.hasHint && hint != "" {
			t.Errorf("unexpected hint for %q: %s", tt.err, hint)
		}
	}
}
