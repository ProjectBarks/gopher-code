package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/auth"
)

// TestOAuthConfig_IntegrationProdDefault verifies that the OAuth config is
// reachable from the binary and returns the production configuration when no
// env overrides are set.
func TestOAuthConfig_IntegrationProdDefault(t *testing.T) {
	// Clear all env vars that could change the config.
	t.Setenv("USER_TYPE", "")
	t.Setenv("USE_LOCAL_OAUTH", "")
	t.Setenv("USE_STAGING_OAUTH", "")
	t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
	t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

	cfg, err := auth.GetOAuthConfig()
	if err != nil {
		t.Fatalf("GetOAuthConfig() returned error: %v", err)
	}

	// Verify all 14 fields match the production config.
	checks := []struct {
		field string
		got   string
		want  string
	}{
		{"BaseAPIURL", cfg.BaseAPIURL, "https://api.anthropic.com"},
		{"ConsoleAuthorizeURL", cfg.ConsoleAuthorizeURL, "https://platform.claude.com/oauth/authorize"},
		{"ClaudeAIAuthorizeURL", cfg.ClaudeAIAuthorizeURL, "https://claude.com/cai/oauth/authorize"},
		{"ClaudeAIOrigin", cfg.ClaudeAIOrigin, "https://claude.ai"},
		{"TokenURL", cfg.TokenURL, "https://platform.claude.com/v1/oauth/token"},
		{"APIKeyURL", cfg.APIKeyURL, "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"},
		{"RolesURL", cfg.RolesURL, "https://api.anthropic.com/api/oauth/claude_cli/roles"},
		{"ConsoleSuccessURL", cfg.ConsoleSuccessURL, "https://platform.claude.com/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code"},
		{"ClaudeAISuccessURL", cfg.ClaudeAISuccessURL, "https://platform.claude.com/oauth/code/success?app=claude-code"},
		{"ManualRedirectURL", cfg.ManualRedirectURL, "https://platform.claude.com/oauth/code/callback"},
		{"ClientID", cfg.ClientID, "9d1c250a-e61b-44d9-88ed-5944d1962f5e"},
		{"OAuthFileSuffix", cfg.OAuthFileSuffix, ""},
		{"MCPProxyURL", cfg.MCPProxyURL, "https://mcp-proxy.anthropic.com"},
		{"MCPProxyPath", cfg.MCPProxyPath, "/v1/mcp/{server_id}"},
	}
	for _, c := range checks {
		if c.got != c.want {
			t.Errorf("%s: got %q, want %q", c.field, c.got, c.want)
		}
	}
}

// TestOAuthConfig_IntegrationScopesAndConstants verifies that OAuth scope
// slices and singleton constants are reachable from the binary with the
// correct values.
func TestOAuthConfig_IntegrationScopesAndConstants(t *testing.T) {
	// Scopes
	if len(auth.ConsoleOAuthScopes) != 2 {
		t.Errorf("ConsoleOAuthScopes: expected 2 scopes, got %d", len(auth.ConsoleOAuthScopes))
	}
	if len(auth.ClaudeAIOAuthScopes) != 5 {
		t.Errorf("ClaudeAIOAuthScopes: expected 5 scopes, got %d", len(auth.ClaudeAIOAuthScopes))
	}
	if len(auth.AllOAuthScopes) != 6 {
		t.Errorf("AllOAuthScopes: expected 6 scopes (deduplicated union), got %d", len(auth.AllOAuthScopes))
	}

	// Constants
	if auth.OAuthBetaHeader != "oauth-2025-04-20" {
		t.Errorf("OAuthBetaHeader: got %q, want %q", auth.OAuthBetaHeader, "oauth-2025-04-20")
	}
	if auth.MCPClientMetadataURL != "https://claude.ai/oauth/claude-code-client-metadata" {
		t.Errorf("MCPClientMetadataURL: got %q, want %q", auth.MCPClientMetadataURL, "https://claude.ai/oauth/claude-code-client-metadata")
	}
}

// TestOAuthConfig_IntegrationCustomURLRejected verifies that unapproved
// custom OAuth URLs are rejected through the real code path.
func TestOAuthConfig_IntegrationCustomURLRejected(t *testing.T) {
	t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "https://evil.example.com")
	t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")
	t.Setenv("USER_TYPE", "")
	t.Setenv("USE_LOCAL_OAUTH", "")
	t.Setenv("USE_STAGING_OAUTH", "")

	_, err := auth.GetOAuthConfig()
	if err == nil {
		t.Fatal("expected error for unapproved custom OAuth URL")
	}
	if err != auth.ErrUnapprovedOAuthEndpoint {
		t.Errorf("expected ErrUnapprovedOAuthEndpoint, got: %v", err)
	}
}

// TestOAuthConfig_IntegrationFileSuffix verifies FileSuffixForOAuthConfig
// returns empty string for default prod env.
func TestOAuthConfig_IntegrationFileSuffix(t *testing.T) {
	t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
	t.Setenv("USER_TYPE", "")
	t.Setenv("USE_LOCAL_OAUTH", "")
	t.Setenv("USE_STAGING_OAUTH", "")

	suffix := auth.FileSuffixForOAuthConfig()
	if suffix != "" {
		t.Errorf("expected empty suffix for prod, got %q", suffix)
	}
}
