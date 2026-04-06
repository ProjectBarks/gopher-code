package auth

// Source: constants/oauth.ts — OAuth URL/scope/client-id configuration.

import (
	"errors"
	"os"
	"strings"
)

// OAuthConfigType represents the environment variant.
type OAuthConfigType string

const (
	OAuthConfigTypeProd    OAuthConfigType = "prod"
	OAuthConfigTypeStaging OAuthConfigType = "staging"
	OAuthConfigTypeLocal   OAuthConfigType = "local"
)

// EnvironmentOAuthConfig holds the full set of OAuth endpoints and identifiers
// for a given environment (prod, staging, local, or custom).
// Source: constants/oauth.ts — OauthConfig type (14 fields)
type EnvironmentOAuthConfig struct {
	BaseAPIURL           string
	ConsoleAuthorizeURL  string
	ClaudeAIAuthorizeURL string
	ClaudeAIOrigin       string
	TokenURL             string
	APIKeyURL            string
	RolesURL             string
	ConsoleSuccessURL    string
	ClaudeAISuccessURL   string
	ManualRedirectURL    string
	ClientID             string
	OAuthFileSuffix      string
	MCPProxyURL          string
	MCPProxyPath         string
}

// Scope constants — verbatim from TS.
const (
	ClaudeAIInferenceScope = "user:inference"
	ClaudeAIProfileScope   = "user:profile"
	consoleScope           = "org:create_api_key"
)

// OAuthBetaHeader is the beta header value for OAuth requests.
const OAuthBetaHeader = "oauth-2025-04-20"

// MCPClientMetadataURL is the Client ID Metadata Document URL for MCP OAuth (CIMD / SEP-991).
const MCPClientMetadataURL = "https://claude.ai/oauth/claude-code-client-metadata"

// ConsoleOAuthScopes are scopes for API key creation via Console.
var ConsoleOAuthScopes = []string{
	consoleScope,
	ClaudeAIProfileScope,
}

// ClaudeAIOAuthScopes are scopes for Claude.ai subscribers.
var ClaudeAIOAuthScopes = []string{
	ClaudeAIProfileScope,
	ClaudeAIInferenceScope,
	"user:sessions:claude_code",
	"user:mcp_servers",
	"user:file_upload",
}

// AllOAuthScopes is the deduplicated union of ConsoleOAuthScopes and ClaudeAIOAuthScopes,
// preserving first-appearance order.
var AllOAuthScopes = func() []string {
	seen := make(map[string]bool)
	var out []string
	for _, s := range append(ConsoleOAuthScopes, ClaudeAIOAuthScopes...) {
		if !seen[s] {
			seen[s] = true
			out = append(out, s)
		}
	}
	return out
}()

// ProdOAuthConfig is the production OAuth configuration.
var ProdOAuthConfig = EnvironmentOAuthConfig{
	BaseAPIURL:           "https://api.anthropic.com",
	ConsoleAuthorizeURL:  "https://platform.claude.com/oauth/authorize",
	ClaudeAIAuthorizeURL: "https://claude.com/cai/oauth/authorize",
	ClaudeAIOrigin:       "https://claude.ai",
	TokenURL:             "https://platform.claude.com/v1/oauth/token",
	APIKeyURL:            "https://api.anthropic.com/api/oauth/claude_cli/create_api_key",
	RolesURL:             "https://api.anthropic.com/api/oauth/claude_cli/roles",
	ConsoleSuccessURL:    "https://platform.claude.com/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code",
	ClaudeAISuccessURL:   "https://platform.claude.com/oauth/code/success?app=claude-code",
	ManualRedirectURL:    "https://platform.claude.com/oauth/code/callback",
	ClientID:             "9d1c250a-e61b-44d9-88ed-5944d1962f5e",
	OAuthFileSuffix:      "",
	MCPProxyURL:          "https://mcp-proxy.anthropic.com",
	MCPProxyPath:         "/v1/mcp/{server_id}",
}

// StagingOAuthConfig is the staging OAuth configuration (ant-only).
var StagingOAuthConfig = EnvironmentOAuthConfig{
	BaseAPIURL:           "https://api-staging.anthropic.com",
	ConsoleAuthorizeURL:  "https://platform.staging.ant.dev/oauth/authorize",
	ClaudeAIAuthorizeURL: "https://claude-ai.staging.ant.dev/oauth/authorize",
	ClaudeAIOrigin:       "https://claude-ai.staging.ant.dev",
	TokenURL:             "https://platform.staging.ant.dev/v1/oauth/token",
	APIKeyURL:            "https://api-staging.anthropic.com/api/oauth/claude_cli/create_api_key",
	RolesURL:             "https://api-staging.anthropic.com/api/oauth/claude_cli/roles",
	ConsoleSuccessURL:    "https://platform.staging.ant.dev/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code",
	ClaudeAISuccessURL:   "https://platform.staging.ant.dev/oauth/code/success?app=claude-code",
	ManualRedirectURL:    "https://platform.staging.ant.dev/oauth/code/callback",
	ClientID:             "22422756-60c9-4084-8eb7-27705fd5cf9a",
	OAuthFileSuffix:      "-staging-oauth",
	MCPProxyURL:          "https://mcp-proxy-staging.anthropic.com",
	MCPProxyPath:         "/v1/mcp/{server_id}",
}

// AllowedOAuthBaseURLs are the only base URLs permitted for CLAUDE_CODE_CUSTOM_OAUTH_URL.
// Only FedStart/PubSec deployments are allowed to prevent credential leakage.
var AllowedOAuthBaseURLs = []string{
	"https://beacon.claude-ai.staging.ant.dev",
	"https://claude.fedstart.com",
	"https://claude-staging.fedstart.com",
}

// ErrUnapprovedOAuthEndpoint is returned when CLAUDE_CODE_CUSTOM_OAUTH_URL is not in the allowlist.
var ErrUnapprovedOAuthEndpoint = errors.New("CLAUDE_CODE_CUSTOM_OAUTH_URL is not an approved endpoint.")

// isEnvTruthy checks if a string looks truthy (1, true, yes).
func isEnvTruthy(val string) bool {
	v := strings.ToLower(strings.TrimSpace(val))
	return v == "1" || v == "true" || v == "yes"
}

// getOAuthConfigType determines the environment based on env vars.
func getOAuthConfigType() OAuthConfigType {
	if os.Getenv("USER_TYPE") == "ant" {
		if isEnvTruthy(os.Getenv("USE_LOCAL_OAUTH")) {
			return OAuthConfigTypeLocal
		}
		if isEnvTruthy(os.Getenv("USE_STAGING_OAUTH")) {
			return OAuthConfigTypeStaging
		}
	}
	return OAuthConfigTypeProd
}

// FileSuffixForOAuthConfig returns the file suffix for the current OAuth config type.
// A custom OAuth URL always takes priority.
func FileSuffixForOAuthConfig() string {
	if os.Getenv("CLAUDE_CODE_CUSTOM_OAUTH_URL") != "" {
		return "-custom-oauth"
	}
	switch getOAuthConfigType() {
	case OAuthConfigTypeLocal:
		return "-local-oauth"
	case OAuthConfigTypeStaging:
		return "-staging-oauth"
	default:
		return ""
	}
}

// GetLocalOAuthConfig builds a local development OAuth config.
// Three local dev servers: :8000 API, :4000 claude-ai frontend, :3000 Console frontend.
func GetLocalOAuthConfig() EnvironmentOAuthConfig {
	api := strings.TrimRight(envOrDefault("CLAUDE_LOCAL_OAUTH_API_BASE", "http://localhost:8000"), "/")
	apps := strings.TrimRight(envOrDefault("CLAUDE_LOCAL_OAUTH_APPS_BASE", "http://localhost:4000"), "/")
	consoleBase := strings.TrimRight(envOrDefault("CLAUDE_LOCAL_OAUTH_CONSOLE_BASE", "http://localhost:3000"), "/")

	return EnvironmentOAuthConfig{
		BaseAPIURL:           api,
		ConsoleAuthorizeURL:  consoleBase + "/oauth/authorize",
		ClaudeAIAuthorizeURL: apps + "/oauth/authorize",
		ClaudeAIOrigin:       apps,
		TokenURL:             api + "/v1/oauth/token",
		APIKeyURL:            api + "/api/oauth/claude_cli/create_api_key",
		RolesURL:             api + "/api/oauth/claude_cli/roles",
		ConsoleSuccessURL:    consoleBase + "/buy_credits?returnUrl=/oauth/code/success%3Fapp%3Dclaude-code",
		ClaudeAISuccessURL:   consoleBase + "/oauth/code/success?app=claude-code",
		ManualRedirectURL:    consoleBase + "/oauth/code/callback",
		ClientID:             "22422756-60c9-4084-8eb7-27705fd5cf9a",
		OAuthFileSuffix:      "-local-oauth",
		MCPProxyURL:          "http://localhost:8205",
		MCPProxyPath:         "/v1/toolbox/shttp/mcp/{server_id}",
	}
}

// envOrDefault returns the env var value if non-empty, otherwise the default.
func envOrDefault(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

// GetOAuthConfig returns the resolved OAuth configuration based on environment.
// It applies CLAUDE_CODE_CUSTOM_OAUTH_URL and CLAUDE_CODE_OAUTH_CLIENT_ID overrides.
func GetOAuthConfig() (EnvironmentOAuthConfig, error) {
	var cfg EnvironmentOAuthConfig
	switch getOAuthConfigType() {
	case OAuthConfigTypeLocal:
		cfg = GetLocalOAuthConfig()
	case OAuthConfigTypeStaging:
		cfg = StagingOAuthConfig
	default:
		cfg = ProdOAuthConfig
	}

	// Apply custom OAuth URL override (FedStart only).
	if oauthBaseURL := os.Getenv("CLAUDE_CODE_CUSTOM_OAUTH_URL"); oauthBaseURL != "" {
		base := strings.TrimRight(oauthBaseURL, "/")
		if !isAllowedOAuthBaseURL(base) {
			return EnvironmentOAuthConfig{}, ErrUnapprovedOAuthEndpoint
		}
		cfg.BaseAPIURL = base
		cfg.ConsoleAuthorizeURL = base + "/oauth/authorize"
		cfg.ClaudeAIAuthorizeURL = base + "/oauth/authorize"
		cfg.ClaudeAIOrigin = base
		cfg.TokenURL = base + "/v1/oauth/token"
		cfg.APIKeyURL = base + "/api/oauth/claude_cli/create_api_key"
		cfg.RolesURL = base + "/api/oauth/claude_cli/roles"
		cfg.ConsoleSuccessURL = base + "/oauth/code/success?app=claude-code"
		cfg.ClaudeAISuccessURL = base + "/oauth/code/success?app=claude-code"
		cfg.ManualRedirectURL = base + "/oauth/code/callback"
		cfg.OAuthFileSuffix = "-custom-oauth"
	}

	// Apply client ID override.
	if clientID := os.Getenv("CLAUDE_CODE_OAUTH_CLIENT_ID"); clientID != "" {
		cfg.ClientID = clientID
	}

	return cfg, nil
}

// isAllowedOAuthBaseURL checks whether a base URL is in the allowlist.
func isAllowedOAuthBaseURL(base string) bool {
	for _, u := range AllowedOAuthBaseURLs {
		if u == base {
			return true
		}
	}
	return false
}
