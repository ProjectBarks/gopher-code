package auth

import (
	"testing"
)

// Source: constants/oauth.ts — scope constants

func TestOAuthScopes(t *testing.T) {
	t.Run("individual_scope_strings", func(t *testing.T) {
		if ClaudeAIInferenceScope != "user:inference" {
			t.Errorf("expected 'user:inference', got %q", ClaudeAIInferenceScope)
		}
		if ClaudeAIProfileScope != "user:profile" {
			t.Errorf("expected 'user:profile', got %q", ClaudeAIProfileScope)
		}
		if consoleScope != "org:create_api_key" {
			t.Errorf("expected 'org:create_api_key', got %q", consoleScope)
		}
	})

	t.Run("console_oauth_scopes", func(t *testing.T) {
		expected := []string{"org:create_api_key", "user:profile"}
		if len(ConsoleOAuthScopes) != len(expected) {
			t.Fatalf("expected %d scopes, got %d", len(expected), len(ConsoleOAuthScopes))
		}
		for i, s := range expected {
			if ConsoleOAuthScopes[i] != s {
				t.Errorf("ConsoleOAuthScopes[%d]: expected %q, got %q", i, s, ConsoleOAuthScopes[i])
			}
		}
	})

	t.Run("claude_ai_oauth_scopes", func(t *testing.T) {
		expected := []string{
			"user:profile",
			"user:inference",
			"user:sessions:claude_code",
			"user:mcp_servers",
			"user:file_upload",
		}
		if len(ClaudeAIOAuthScopes) != len(expected) {
			t.Fatalf("expected %d scopes, got %d", len(expected), len(ClaudeAIOAuthScopes))
		}
		for i, s := range expected {
			if ClaudeAIOAuthScopes[i] != s {
				t.Errorf("ClaudeAIOAuthScopes[%d]: expected %q, got %q", i, s, ClaudeAIOAuthScopes[i])
			}
		}
	})

	t.Run("all_oauth_scopes_is_union", func(t *testing.T) {
		// All scopes = union of Console + ClaudeAI, deduplicated, order preserved.
		// Console: org:create_api_key, user:profile
		// ClaudeAI: user:profile, user:inference, user:sessions:claude_code, user:mcp_servers, user:file_upload
		// Union (order of first appearance): org:create_api_key, user:profile, user:inference, user:sessions:claude_code, user:mcp_servers, user:file_upload
		expected := []string{
			"org:create_api_key",
			"user:profile",
			"user:inference",
			"user:sessions:claude_code",
			"user:mcp_servers",
			"user:file_upload",
		}
		if len(AllOAuthScopes) != len(expected) {
			t.Fatalf("expected %d scopes, got %d", len(expected), len(AllOAuthScopes))
		}
		for i, s := range expected {
			if AllOAuthScopes[i] != s {
				t.Errorf("AllOAuthScopes[%d]: expected %q, got %q", i, s, AllOAuthScopes[i])
			}
		}
	})
}

func TestOAuthBetaHeader(t *testing.T) {
	if OAuthBetaHeader != "oauth-2025-04-20" {
		t.Errorf("expected 'oauth-2025-04-20', got %q", OAuthBetaHeader)
	}
}

func TestMCPClientMetadataURL(t *testing.T) {
	if MCPClientMetadataURL != "https://claude.ai/oauth/claude-code-client-metadata" {
		t.Errorf("expected 'https://claude.ai/oauth/claude-code-client-metadata', got %q", MCPClientMetadataURL)
	}
}

// Source: constants/oauth.ts — PROD_OAUTH_CONFIG

func TestProdOAuthConfig(t *testing.T) {
	cfg := ProdOAuthConfig

	checks := map[string]struct{ got, want string }{
		"BaseAPIURL":          {cfg.BaseAPIURL, "https://api.anthropic.com"},
		"ConsoleAuthorizeURL": {cfg.ConsoleAuthorizeURL, "https://platform.claude.com/oauth/authorize"},
		"ClaudeAIAuthorizeURL": {cfg.ClaudeAIAuthorizeURL, "https://claude.com/cai/oauth/authorize"},
		"ClaudeAIOrigin":      {cfg.ClaudeAIOrigin, "https://claude.ai"},
		"TokenURL":            {cfg.TokenURL, "https://platform.claude.com/v1/oauth/token"},
		"APIKeyURL":           {cfg.APIKeyURL, "https://api.anthropic.com/api/oauth/claude_cli/create_api_key"},
		"RolesURL":            {cfg.RolesURL, "https://api.anthropic.com/api/oauth/claude_cli/roles"},
		"ConsoleSuccessURL":   {cfg.ConsoleSuccessURL, "https://platform.claude.com/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code"},
		"ClaudeAISuccessURL":  {cfg.ClaudeAISuccessURL, "https://platform.claude.com/oauth/code/success?app=claude-code"},
		"ManualRedirectURL":   {cfg.ManualRedirectURL, "https://platform.claude.com/oauth/code/callback"},
		"ClientID":            {cfg.ClientID, "9d1c250a-e61b-44d9-88ed-5944d1962f5e"},
		"OAuthFileSuffix":     {cfg.OAuthFileSuffix, ""},
		"MCPProxyURL":         {cfg.MCPProxyURL, "https://mcp-proxy.anthropic.com"},
		"MCPProxyPath":        {cfg.MCPProxyPath, "/v1/mcp/{server_id}"},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("ProdOAuthConfig.%s: expected %q, got %q", name, c.want, c.got)
		}
	}
}

// Source: constants/oauth.ts — STAGING_OAUTH_CONFIG

func TestStagingOAuthConfig(t *testing.T) {
	cfg := StagingOAuthConfig

	checks := map[string]struct{ got, want string }{
		"BaseAPIURL":           {cfg.BaseAPIURL, "https://api-staging.anthropic.com"},
		"ConsoleAuthorizeURL":  {cfg.ConsoleAuthorizeURL, "https://platform.staging.ant.dev/oauth/authorize"},
		"ClaudeAIAuthorizeURL": {cfg.ClaudeAIAuthorizeURL, "https://claude-ai.staging.ant.dev/oauth/authorize"},
		"ClaudeAIOrigin":       {cfg.ClaudeAIOrigin, "https://claude-ai.staging.ant.dev"},
		"TokenURL":             {cfg.TokenURL, "https://platform.staging.ant.dev/v1/oauth/token"},
		"APIKeyURL":            {cfg.APIKeyURL, "https://api-staging.anthropic.com/api/oauth/claude_cli/create_api_key"},
		"RolesURL":             {cfg.RolesURL, "https://api-staging.anthropic.com/api/oauth/claude_cli/roles"},
		"ConsoleSuccessURL":    {cfg.ConsoleSuccessURL, "https://platform.staging.ant.dev/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code"},
		"ClaudeAISuccessURL":   {cfg.ClaudeAISuccessURL, "https://platform.staging.ant.dev/oauth/code/success?app=claude-code"},
		"ManualRedirectURL":    {cfg.ManualRedirectURL, "https://platform.staging.ant.dev/oauth/code/callback"},
		"ClientID":             {cfg.ClientID, "22422756-60c9-4084-8eb7-27705fd5cf9a"},
		"OAuthFileSuffix":      {cfg.OAuthFileSuffix, "-staging-oauth"},
		"MCPProxyURL":          {cfg.MCPProxyURL, "https://mcp-proxy-staging.anthropic.com"},
		"MCPProxyPath":         {cfg.MCPProxyPath, "/v1/mcp/{server_id}"},
	}
	for name, c := range checks {
		if c.got != c.want {
			t.Errorf("StagingOAuthConfig.%s: expected %q, got %q", name, c.want, c.got)
		}
	}
}

// Source: constants/oauth.ts — getLocalOauthConfig

func TestLocalOAuthConfig(t *testing.T) {
	t.Run("defaults", func(t *testing.T) {
		// Clear any env overrides
		t.Setenv("CLAUDE_LOCAL_OAUTH_API_BASE", "")
		t.Setenv("CLAUDE_LOCAL_OAUTH_APPS_BASE", "")
		t.Setenv("CLAUDE_LOCAL_OAUTH_CONSOLE_BASE", "")

		cfg := GetLocalOAuthConfig()

		checks := map[string]struct{ got, want string }{
			"BaseAPIURL":           {cfg.BaseAPIURL, "http://localhost:8000"},
			"ConsoleAuthorizeURL":  {cfg.ConsoleAuthorizeURL, "http://localhost:3000/oauth/authorize"},
			"ClaudeAIAuthorizeURL": {cfg.ClaudeAIAuthorizeURL, "http://localhost:4000/oauth/authorize"},
			"ClaudeAIOrigin":       {cfg.ClaudeAIOrigin, "http://localhost:4000"},
			"TokenURL":             {cfg.TokenURL, "http://localhost:8000/v1/oauth/token"},
			"APIKeyURL":            {cfg.APIKeyURL, "http://localhost:8000/api/oauth/claude_cli/create_api_key"},
			"RolesURL":             {cfg.RolesURL, "http://localhost:8000/api/oauth/claude_cli/roles"},
			"ConsoleSuccessURL":    {cfg.ConsoleSuccessURL, "http://localhost:3000/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code"},
			"ClaudeAISuccessURL":   {cfg.ClaudeAISuccessURL, "http://localhost:3000/oauth/code/success?app=claude-code"},
			"ManualRedirectURL":    {cfg.ManualRedirectURL, "http://localhost:3000/oauth/code/callback"},
			"ClientID":             {cfg.ClientID, "22422756-60c9-4084-8eb7-27705fd5cf9a"},
			"OAuthFileSuffix":      {cfg.OAuthFileSuffix, "-local-oauth"},
			"MCPProxyURL":          {cfg.MCPProxyURL, "http://localhost:8205"},
			"MCPProxyPath":         {cfg.MCPProxyPath, "/v1/toolbox/shttp/mcp/{server_id}"},
		}
		for name, c := range checks {
			if c.got != c.want {
				t.Errorf("LocalOAuthConfig.%s: expected %q, got %q", name, c.want, c.got)
			}
		}
	})

	t.Run("env_overrides", func(t *testing.T) {
		t.Setenv("CLAUDE_LOCAL_OAUTH_API_BASE", "http://myhost:9000/")
		t.Setenv("CLAUDE_LOCAL_OAUTH_APPS_BASE", "http://myhost:5000/")
		t.Setenv("CLAUDE_LOCAL_OAUTH_CONSOLE_BASE", "http://myhost:7000/")

		cfg := GetLocalOAuthConfig()

		// Trailing slashes should be trimmed
		if cfg.BaseAPIURL != "http://myhost:9000" {
			t.Errorf("BaseAPIURL: expected 'http://myhost:9000', got %q", cfg.BaseAPIURL)
		}
		if cfg.ClaudeAIOrigin != "http://myhost:5000" {
			t.Errorf("ClaudeAIOrigin: expected 'http://myhost:5000', got %q", cfg.ClaudeAIOrigin)
		}
		if cfg.ConsoleAuthorizeURL != "http://myhost:7000/oauth/authorize" {
			t.Errorf("ConsoleAuthorizeURL: expected 'http://myhost:7000/oauth/authorize', got %q", cfg.ConsoleAuthorizeURL)
		}
	})
}

// Source: constants/oauth.ts — ALLOWED_OAUTH_BASE_URLS

func TestAllowedOAuthBaseURLs(t *testing.T) {
	expected := []string{
		"https://beacon.claude-ai.staging.ant.dev",
		"https://claude.fedstart.com",
		"https://claude-staging.fedstart.com",
	}
	if len(AllowedOAuthBaseURLs) != len(expected) {
		t.Fatalf("expected %d allowed URLs, got %d", len(expected), len(AllowedOAuthBaseURLs))
	}
	for i, u := range expected {
		if AllowedOAuthBaseURLs[i] != u {
			t.Errorf("AllowedOAuthBaseURLs[%d]: expected %q, got %q", i, u, AllowedOAuthBaseURLs[i])
		}
	}
}

// Source: constants/oauth.ts — getOauthConfig (env override logic)

func TestGetOAuthConfig(t *testing.T) {
	t.Run("prod_default", func(t *testing.T) {
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "9d1c250a-e61b-44d9-88ed-5944d1962f5e" {
			t.Errorf("expected prod client ID, got %q", cfg.ClientID)
		}
		if cfg.OAuthFileSuffix != "" {
			t.Errorf("expected empty suffix for prod, got %q", cfg.OAuthFileSuffix)
		}
	})

	t.Run("staging_when_ant", func(t *testing.T) {
		t.Setenv("USER_TYPE", "ant")
		t.Setenv("USE_STAGING_OAUTH", "true")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "22422756-60c9-4084-8eb7-27705fd5cf9a" {
			t.Errorf("expected staging client ID, got %q", cfg.ClientID)
		}
		if cfg.BaseAPIURL != "https://api-staging.anthropic.com" {
			t.Errorf("expected staging base API URL, got %q", cfg.BaseAPIURL)
		}
	})

	t.Run("staging_ignored_when_not_ant", func(t *testing.T) {
		t.Setenv("USER_TYPE", "external")
		t.Setenv("USE_STAGING_OAUTH", "true")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should fall back to prod
		if cfg.ClientID != "9d1c250a-e61b-44d9-88ed-5944d1962f5e" {
			t.Errorf("expected prod client ID when not ant, got %q", cfg.ClientID)
		}
	})

	t.Run("local_when_ant", func(t *testing.T) {
		t.Setenv("USER_TYPE", "ant")
		t.Setenv("USE_LOCAL_OAUTH", "1")
		t.Setenv("USE_STAGING_OAUTH", "true") // local takes priority over staging
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")
		t.Setenv("CLAUDE_LOCAL_OAUTH_API_BASE", "")
		t.Setenv("CLAUDE_LOCAL_OAUTH_APPS_BASE", "")
		t.Setenv("CLAUDE_LOCAL_OAUTH_CONSOLE_BASE", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BaseAPIURL != "http://localhost:8000" {
			t.Errorf("expected local base API URL, got %q", cfg.BaseAPIURL)
		}
		if cfg.OAuthFileSuffix != "-local-oauth" {
			t.Errorf("expected '-local-oauth' suffix, got %q", cfg.OAuthFileSuffix)
		}
	})

	t.Run("custom_url_approved", func(t *testing.T) {
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "https://claude.fedstart.com")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BaseAPIURL != "https://claude.fedstart.com" {
			t.Errorf("expected custom base URL, got %q", cfg.BaseAPIURL)
		}
		if cfg.ConsoleAuthorizeURL != "https://claude.fedstart.com/oauth/authorize" {
			t.Errorf("expected custom authorize URL, got %q", cfg.ConsoleAuthorizeURL)
		}
		if cfg.OAuthFileSuffix != "-custom-oauth" {
			t.Errorf("expected '-custom-oauth' suffix, got %q", cfg.OAuthFileSuffix)
		}
		// ClientID and MCPProxyURL should remain from base config (prod)
		if cfg.ClientID != "9d1c250a-e61b-44d9-88ed-5944d1962f5e" {
			t.Errorf("expected prod client ID preserved, got %q", cfg.ClientID)
		}
		if cfg.MCPProxyURL != "https://mcp-proxy.anthropic.com" {
			t.Errorf("expected prod MCP proxy URL preserved, got %q", cfg.MCPProxyURL)
		}
	})

	t.Run("custom_url_trailing_slash_trimmed", func(t *testing.T) {
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "https://claude.fedstart.com/")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.BaseAPIURL != "https://claude.fedstart.com" {
			t.Errorf("trailing slash not trimmed: got %q", cfg.BaseAPIURL)
		}
	})

	t.Run("custom_url_unapproved_error", func(t *testing.T) {
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "https://evil.example.com")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "")

		_, err := GetOAuthConfig()
		if err == nil {
			t.Fatal("expected error for unapproved URL")
		}
		want := "CLAUDE_CODE_CUSTOM_OAUTH_URL is not an approved endpoint."
		if err.Error() != want {
			t.Errorf("expected error %q, got %q", want, err.Error())
		}
	})

	t.Run("client_id_override", func(t *testing.T) {
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("CLAUDE_CODE_OAUTH_CLIENT_ID", "custom-client-id-abc")

		cfg, err := GetOAuthConfig()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if cfg.ClientID != "custom-client-id-abc" {
			t.Errorf("expected custom client ID, got %q", cfg.ClientID)
		}
	})
}

// Source: constants/oauth.ts — fileSuffixForOauthConfig

func TestFileSuffixForOAuthConfig(t *testing.T) {
	t.Run("custom_url_takes_priority", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "https://claude.fedstart.com")
		t.Setenv("USER_TYPE", "ant")
		t.Setenv("USE_LOCAL_OAUTH", "true")

		if s := FileSuffixForOAuthConfig(); s != "-custom-oauth" {
			t.Errorf("expected '-custom-oauth', got %q", s)
		}
	})

	t.Run("local", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("USER_TYPE", "ant")
		t.Setenv("USE_LOCAL_OAUTH", "true")
		t.Setenv("USE_STAGING_OAUTH", "")

		if s := FileSuffixForOAuthConfig(); s != "-local-oauth" {
			t.Errorf("expected '-local-oauth', got %q", s)
		}
	})

	t.Run("staging", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("USER_TYPE", "ant")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "true")

		if s := FileSuffixForOAuthConfig(); s != "-staging-oauth" {
			t.Errorf("expected '-staging-oauth', got %q", s)
		}
	})

	t.Run("prod", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_CUSTOM_OAUTH_URL", "")
		t.Setenv("USER_TYPE", "")
		t.Setenv("USE_LOCAL_OAUTH", "")
		t.Setenv("USE_STAGING_OAUTH", "")

		if s := FileSuffixForOAuthConfig(); s != "" {
			t.Errorf("expected empty string for prod, got %q", s)
		}
	})
}
