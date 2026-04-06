// Package handlers implements CLI subcommand handlers.
// Source: src/cli/handlers/auth.ts
package handlers

import (
	"encoding/json"
	"fmt"
	"io"
	"os"
	"strings"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/zalando/go-keyring"
)

// AuthMethod represents how the user is authenticated.
// Source: src/cli/handlers/auth.ts — authMethod values
type AuthMethod string

const (
	AuthMethodNone         AuthMethod = "none"
	AuthMethodThirdParty   AuthMethod = "third_party"
	AuthMethodClaudeAI     AuthMethod = "claude.ai"
	AuthMethodAPIKeyHelper AuthMethod = "api_key_helper"
	AuthMethodOAuthToken   AuthMethod = "oauth_token"
	AuthMethodAPIKey       AuthMethod = "api_key"
)

// Keyring keys for OAuth tokens.
const (
	keyringOAuthAccess  = "oauth-access-token"
	keyringOAuthRefresh = "oauth-refresh-token"
	keyringAccountEmail = "account-email"
	keyringAccountUUID  = "account-uuid"
	keyringOrgUUID      = "org-uuid"
	keyringOrgName      = "org-name"
	keyringSubType      = "subscription-type"
)

// AuthStatusOutput is the JSON structure for `auth status --json`.
// Source: src/cli/handlers/auth.ts — authStatus JSON output shape
type AuthStatusOutput struct {
	LoggedIn         bool   `json:"loggedIn"`
	AuthMethod       string `json:"authMethod"`
	APIProvider      string `json:"apiProvider"`
	APIKeySource     string `json:"apiKeySource,omitempty"`
	Email            string `json:"email,omitempty"`
	OrgID            string `json:"orgId,omitempty"`
	OrgName          string `json:"orgName,omitempty"`
	SubscriptionType string `json:"subscriptionType,omitempty"`
}

// AuthLoginOpts holds options for the login subcommand.
type AuthLoginOpts struct {
	Email    string
	SSO      bool
	Console  bool
	ClaudeAI bool
	Output   io.Writer // defaults to os.Stdout
}

// AuthStatusOpts holds options for the status subcommand.
type AuthStatusOpts struct {
	JSON   bool
	Output io.Writer // defaults to os.Stdout
}

// AuthLogoutOpts holds options for the logout subcommand.
type AuthLogoutOpts struct {
	Output io.Writer // defaults to os.Stdout
}

// KeyringStore abstracts keyring operations for testing.
type KeyringStore interface {
	Get(service, key string) (string, error)
	Set(service, key, value string) error
	Delete(service, key string) error
}

// OSKeyringStore uses the real OS keyring via go-keyring.
type OSKeyringStore struct{}

func (OSKeyringStore) Get(service, key string) (string, error) { return keyring.Get(service, key) }
func (OSKeyringStore) Set(service, key, value string) error    { return keyring.Set(service, key, value) }
func (OSKeyringStore) Delete(service, key string) error        { return keyring.Delete(service, key) }

// DefaultKeyring is the keyring store used by auth handlers.
// Override in tests.
var DefaultKeyring KeyringStore = OSKeyringStore{}

// OAuthFlowStarter abstracts the browser-based OAuth flow for testing.
type OAuthFlowStarter interface {
	// StartOAuthFlow opens the browser and waits for the callback.
	// Returns tokens on success.
	StartOAuthFlow(email string, scopes []string, useClaudeAI bool) (*auth.OAuthTokens, error)
}

func output(w io.Writer) io.Writer {
	if w != nil {
		return w
	}
	return os.Stdout
}

// AuthLogin handles `claude auth login`.
// Source: src/cli/handlers/auth.ts — authLogin
func AuthLogin(opts AuthLoginOpts, flow OAuthFlowStarter) int {
	w := output(opts.Output)

	// Mutual exclusion: --console and --claudeai
	if opts.Console && opts.ClaudeAI {
		fmt.Fprint(w, "Error: --console and --claudeai cannot be used together.\n")
		return 1
	}

	// Env var fast-path: CLAUDE_CODE_OAUTH_REFRESH_TOKEN
	if refreshToken := os.Getenv("CLAUDE_CODE_OAUTH_REFRESH_TOKEN"); refreshToken != "" {
		scopeStr := os.Getenv("CLAUDE_CODE_OAUTH_SCOPES")
		if scopeStr == "" {
			fmt.Fprint(w, "CLAUDE_CODE_OAUTH_SCOPES is required when using CLAUDE_CODE_OAUTH_REFRESH_TOKEN.\nSet it to the space-separated scopes the refresh token was issued with\n(e.g. \"user:inference\" or \"user:profile user:inference user:sessions:claude_code user:mcp_servers\").\n")
			return 1
		}
		scopes := auth.ParseScopes(scopeStr)

		oauthCfg, err := auth.GetOAuthConfig()
		if err != nil {
			fmt.Fprintf(w, "Login failed: %v\n", err)
			return 1
		}

		tokens, err := auth.RefreshOAuthToken(auth.RefreshTokenParams{
			TokenURL:     oauthCfg.TokenURL,
			ClientID:     oauthCfg.ClientID,
			RefreshToken: refreshToken,
			Scopes:       scopes,
		})
		if err != nil {
			sslHint := getSSLErrorHint(err)
			msg := fmt.Sprintf("Login failed: %v\n", err)
			if sslHint != "" {
				msg += sslHint + "\n"
			}
			fmt.Fprint(w, msg)
			return 1
		}

		// Store tokens
		storeOAuthTokens(tokens)
		fmt.Fprint(w, "Login successful.\n")
		return 0
	}

	// Determine scopes based on --console / --claudeai
	useClaudeAI := !opts.Console // default to claude.ai unless --console
	if opts.ClaudeAI {
		useClaudeAI = true
	}

	var scopes []string
	if useClaudeAI {
		scopes = auth.ClaudeAIOAuthScopes
	} else {
		scopes = auth.ConsoleOAuthScopes
	}

	fmt.Fprint(w, "Opening browser to sign in\u2026\n")

	tokens, err := flow.StartOAuthFlow(opts.Email, scopes, useClaudeAI)
	if err != nil {
		sslHint := getSSLErrorHint(err)
		msg := fmt.Sprintf("Login failed: %v\n", err)
		if sslHint != "" {
			msg += sslHint + "\n"
		}
		fmt.Fprint(w, msg)
		return 1
	}

	storeOAuthTokens(tokens)
	fmt.Fprint(w, "Login successful.\n")
	return 0
}

// AuthStatus handles `claude auth status`.
// Source: src/cli/handlers/auth.ts — authStatus
func AuthStatus(opts AuthStatusOpts) int {
	w := output(opts.Output)
	ks := DefaultKeyring

	status := resolveAuthStatus(ks)

	if opts.JSON {
		data, _ := json.MarshalIndent(status, "", "  ")
		fmt.Fprintln(w, string(data))
		return 0
	}

	// Text output
	if !status.LoggedIn {
		fmt.Fprint(w, "Not logged in. Run claude auth login to authenticate.\n")
		return 1
	}

	printField(w, "Auth method", status.AuthMethod)
	printField(w, "API provider", status.APIProvider)
	if status.APIKeySource != "" {
		printField(w, "API key source", status.APIKeySource)
	}
	if status.Email != "" {
		printField(w, "Email", status.Email)
	}
	if status.OrgID != "" {
		printField(w, "Org ID", status.OrgID)
	}
	if status.OrgName != "" {
		printField(w, "Org name", status.OrgName)
	}
	if status.SubscriptionType != "" {
		printField(w, "Subscription", status.SubscriptionType)
	}
	return 0
}

// AuthLogout handles `claude auth logout`.
// Source: src/cli/handlers/auth.ts — authLogout
func AuthLogout(opts AuthLogoutOpts) int {
	w := output(opts.Output)

	if err := performLogout(); err != nil {
		fmt.Fprint(w, "Failed to log out.\n")
		return 1
	}

	fmt.Fprint(w, "Successfully logged out from your Anthropic account.\n")
	return 0
}

// resolveAuthStatus builds the AuthStatusOutput by checking multiple auth sources.
// Source: src/cli/handlers/auth.ts — authMethod 6-value decision tree
func resolveAuthStatus(ks KeyringStore) AuthStatusOutput {
	out := AuthStatusOutput{
		APIProvider: "anthropic",
	}

	// Check for ANTHROPIC_API_KEY env var
	if apiKey := os.Getenv("ANTHROPIC_API_KEY"); apiKey != "" {
		out.LoggedIn = true
		out.AuthMethod = string(AuthMethodAPIKey)
		out.APIKeySource = "ANTHROPIC_API_KEY"
		return out
	}

	// Check for OAuth tokens in keyring
	accessToken, err := ks.Get(auth.KeyringService, keyringOAuthAccess)
	if err == nil && accessToken != "" {
		out.LoggedIn = true
		if auth.ShouldUseClaudeAIAuth(auth.ClaudeAIOAuthScopes) {
			out.AuthMethod = string(AuthMethodClaudeAI)
		} else {
			out.AuthMethod = string(AuthMethodOAuthToken)
		}

		// Populate account info from keyring
		if email, err := ks.Get(auth.KeyringService, keyringAccountEmail); err == nil {
			out.Email = email
		}
		if orgID, err := ks.Get(auth.KeyringService, keyringOrgUUID); err == nil {
			out.OrgID = orgID
		}
		if orgName, err := ks.Get(auth.KeyringService, keyringOrgName); err == nil {
			out.OrgName = orgName
		}
		if subType, err := ks.Get(auth.KeyringService, keyringSubType); err == nil {
			out.SubscriptionType = subType
		}
		return out
	}

	// Check for API key in keyring (non-OAuth)
	if apiKey, err := ks.Get(auth.KeyringService, auth.KeyringAPIKey); err == nil && apiKey != "" {
		out.LoggedIn = true
		out.AuthMethod = string(AuthMethodAPIKeyHelper)
		return out
	}

	// Not logged in
	out.AuthMethod = string(AuthMethodNone)
	return out
}

// performLogout clears all auth tokens and account info from the keyring.
// Source: src/cli/handlers/auth.ts — performLogout
func performLogout() error {
	ks := DefaultKeyring
	keys := []string{
		keyringOAuthAccess,
		keyringOAuthRefresh,
		keyringAccountEmail,
		keyringAccountUUID,
		keyringOrgUUID,
		keyringOrgName,
		keyringSubType,
		auth.KeyringAPIKey,
	}

	var lastErr error
	for _, key := range keys {
		if err := ks.Delete(auth.KeyringService, key); err != nil {
			// Ignore "not found" errors — key may not exist
			if !isNotFoundError(err) {
				lastErr = err
			}
		}
	}

	// Also remove plaintext auth file
	auth.DeleteAPIKey()

	return lastErr
}

// storeOAuthTokens saves OAuth tokens and metadata to the keyring.
func storeOAuthTokens(tokens *auth.OAuthTokens) {
	ks := DefaultKeyring
	_ = ks.Set(auth.KeyringService, keyringOAuthAccess, tokens.AccessToken)
	if tokens.RefreshToken != "" {
		_ = ks.Set(auth.KeyringService, keyringOAuthRefresh, tokens.RefreshToken)
	}
}

// StoreAccountInfo saves account metadata to the keyring (exported for use by OAuth flow).
func StoreAccountInfo(email, accountUUID, orgUUID, orgName, subscriptionType string) {
	ks := DefaultKeyring
	if email != "" {
		_ = ks.Set(auth.KeyringService, keyringAccountEmail, email)
	}
	if accountUUID != "" {
		_ = ks.Set(auth.KeyringService, keyringAccountUUID, accountUUID)
	}
	if orgUUID != "" {
		_ = ks.Set(auth.KeyringService, keyringOrgUUID, orgUUID)
	}
	if orgName != "" {
		_ = ks.Set(auth.KeyringService, keyringOrgName, orgName)
	}
	if subscriptionType != "" {
		_ = ks.Set(auth.KeyringService, keyringSubType, subscriptionType)
	}
}

// printField prints a single label: value line for text output.
func printField(w io.Writer, label, value string) {
	fmt.Fprintf(w, "%s: %s\n", label, value)
}

// getSSLErrorHint returns a helpful hint if the error looks SSL-related.
// Source: src/cli/handlers/auth.ts — getSSLErrorHint
func getSSLErrorHint(err error) string {
	msg := err.Error()
	if strings.Contains(msg, "x509") || strings.Contains(msg, "certificate") || strings.Contains(msg, "tls") || strings.Contains(msg, "SSL") {
		return "Hint: This may be caused by a corporate proxy or firewall intercepting SSL connections."
	}
	return ""
}

// isNotFoundError checks if a keyring error indicates a missing key.
func isNotFoundError(err error) bool {
	if err == nil {
		return false
	}
	msg := err.Error()
	return strings.Contains(msg, "not found") ||
		strings.Contains(msg, "does not exist") ||
		strings.Contains(msg, "secret not found")
}
