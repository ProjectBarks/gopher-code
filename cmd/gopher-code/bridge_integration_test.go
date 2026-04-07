package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/auth"
	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// TestBridgeConfig_IntegrationDefaults verifies that the bridge config helpers
// are reachable from the binary and return sane defaults when no env overrides
// are set (the common case for non-Anthropic users).
func TestBridgeConfig_IntegrationDefaults(t *testing.T) {
	// Build the same ConfigDeps that main.go wires for remote-control.
	deps := bridge.ConfigDeps{
		GetAccessToken: func() (string, bool) {
			key, err := auth.GetAPIKey()
			if err != nil {
				return "", false
			}
			return key, true
		},
		GetBaseAPIURL: func() string { return "https://api.anthropic.com" },
	}

	// Without ant env overrides, BridgeBaseURL should return the production default.
	u := bridge.BridgeBaseURL(deps)
	if u != "https://api.anthropic.com" {
		t.Fatalf("BridgeBaseURL = %q, want production default", u)
	}

	// Without any token source, BridgeAccessToken should return not-authed.
	// (We don't set ANTHROPIC_API_KEY or keychain tokens in test env.)
	// Just verify it doesn't panic.
	_, _ = bridge.BridgeAccessToken(deps)
}

// TestBridgeConfig_EnvOverride verifies that ant-only env overrides flow
// through the same ConfigDeps path used in the binary.
func TestBridgeConfig_EnvOverride(t *testing.T) {
	t.Setenv("USER_TYPE", "ant")
	t.Setenv("CLAUDE_BRIDGE_BASE_URL", "https://dev.example.com")
	t.Setenv("CLAUDE_BRIDGE_OAUTH_TOKEN", "test-token-xyz")

	deps := bridge.ConfigDeps{
		GetAccessToken: func() (string, bool) { return "fallback", true },
		GetBaseAPIURL:  func() string { return "https://api.anthropic.com" },
	}

	// Env override should win over deps.
	u := bridge.BridgeBaseURL(deps)
	if u != "https://dev.example.com" {
		t.Fatalf("BridgeBaseURL = %q, want env override", u)
	}

	tok, ok := bridge.BridgeAccessToken(deps)
	if !ok {
		t.Fatal("expected bridge access token to be available")
	}
	if tok != "test-token-xyz" {
		t.Fatalf("BridgeAccessToken = %q, want env override token", tok)
	}
}
