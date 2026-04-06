// Package bridge — config helpers for bridge auth/URL resolution.
// Source: src/bridge/bridgeConfig.ts
package bridge

import "os"

// ---------------------------------------------------------------------------
// Env-var keys (verbatim from TS)
// ---------------------------------------------------------------------------

const (
	envUserType             = "USER_TYPE"
	envBridgeOAuthToken     = "CLAUDE_BRIDGE_OAUTH_TOKEN"
	envBridgeBaseURL        = "CLAUDE_BRIDGE_BASE_URL"
	userTypeAnt             = "ant"
)

// AccessTokenFunc is a function that returns the OAuth access token from the
// keychain (or equivalent credential store). Nil return means "not logged in".
// This mirrors TS getClaudeAIOAuthTokens()?.accessToken.
type AccessTokenFunc func() (string, bool)

// BaseURLFunc returns the production base API URL.
// This mirrors TS getOauthConfig().BASE_API_URL.
type BaseURLFunc func() string

// ConfigDeps bundles the external dependencies that the config helpers need.
// Callers wire these once; tests supply stubs.
type ConfigDeps struct {
	// GetAccessToken returns the OAuth keychain access token (fallback).
	// Return ("", false) when no token is available.
	GetAccessToken AccessTokenFunc

	// GetBaseAPIURL returns the production base API URL (fallback).
	GetBaseAPIURL BaseURLFunc
}

// BridgeTokenOverride returns the ant-only dev override for the bridge OAuth
// token (CLAUDE_BRIDGE_OAUTH_TOKEN). Returns ("", false) when the env var is
// unset or the user is not an Anthropic employee.
// Source: bridgeConfig.ts — getBridgeTokenOverride
func BridgeTokenOverride() (string, bool) {
	if os.Getenv(envUserType) != userTypeAnt {
		return "", false
	}
	tok := os.Getenv(envBridgeOAuthToken)
	if tok == "" {
		return "", false
	}
	return tok, true
}

// BridgeBaseURLOverride returns the ant-only dev override for the bridge base
// URL (CLAUDE_BRIDGE_BASE_URL). Returns ("", false) when unset or non-ant.
// Source: bridgeConfig.ts — getBridgeBaseUrlOverride
func BridgeBaseURLOverride() (string, bool) {
	if os.Getenv(envUserType) != userTypeAnt {
		return "", false
	}
	u := os.Getenv(envBridgeBaseURL)
	if u == "" {
		return "", false
	}
	return u, true
}

// BridgeAccessToken returns the access token for bridge API calls.
// Priority: ant-only env override > OAuth keychain token.
// Returns ("", false) when no token is available ("not logged in").
// Source: bridgeConfig.ts — getBridgeAccessToken
func BridgeAccessToken(deps ConfigDeps) (string, bool) {
	if tok, ok := BridgeTokenOverride(); ok {
		return tok, true
	}
	if deps.GetAccessToken != nil {
		return deps.GetAccessToken()
	}
	return "", false
}

// BridgeBaseURL returns the base URL for bridge API calls.
// Priority: ant-only env override > production OAuth config BASE_API_URL.
// Always returns a URL (falls through to the production default).
// Source: bridgeConfig.ts — getBridgeBaseUrl
func BridgeBaseURL(deps ConfigDeps) string {
	if u, ok := BridgeBaseURLOverride(); ok {
		return u
	}
	if deps.GetBaseAPIURL != nil {
		return deps.GetBaseAPIURL()
	}
	// Hardcoded production default — matches TS getOauthConfig().BASE_API_URL.
	return "https://api.anthropic.com"
}
