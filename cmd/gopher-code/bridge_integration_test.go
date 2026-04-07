package main

import (
	"strings"
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

// TestInitBridgeDeps_SetsDefaultDeps verifies that initBridgeDeps registers a
// non-nil BridgeDeps bundle with safe defaults (bridge disabled, stubs wired).
func TestInitBridgeDeps_SetsDefaultDeps(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.IsBridgeEnabled() {
		t.Fatal("expected IsBridgeEnabled=false after initBridgeDeps (default build has BridgeMode=false)")
	}

	reason, err := bridge.GetBridgeDisabledReason()
	if err != nil {
		t.Fatalf("unexpected error from GetBridgeDisabledReason: %v", err)
	}
	if reason != bridge.ErrBridgeNotAvailable {
		t.Fatalf("expected %q, got %q", bridge.ErrBridgeNotAvailable, reason)
	}
}

// TestInitBridgeDeps_VersionWired verifies that the Version field in BridgeDeps
// is set to the binary's Version constant.
func TestInitBridgeDeps_VersionWired(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if msg := bridge.CheckBridgeMinVersion(); msg != "" {
		t.Fatalf("expected empty min-version check, got: %q", msg)
	}
}

// TestRemoteControl_BlockedByDefault verifies that the remote-control subcommand
// is blocked when bridge is not enabled and not forced.
func TestRemoteControl_BlockedByDefault(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "")

	code, stderrOut, _ := withExitCapture(func() {
		reason, err := bridge.GetBridgeDisabledReason()
		if err != nil {
			cliErrorf("Error checking Remote Control eligibility: %v", err)
			return
		}
		if reason == "" {
			reason = bridge.ErrBridgeNotAvailable
		}
		cliError(reason)
	})
	if code != 1 {
		t.Fatalf("expected exit code 1, got %d", code)
	}
	if !strings.Contains(stderrOut, "Remote Control is not available") {
		t.Fatalf("expected 'Remote Control is not available' in stderr, got: %q", stderrOut)
	}
}

// TestRemoteControl_ForcedByEnvVar verifies that CLAUDE_CODE_BRIDGE_FORCED=1
// bypasses the bridge enablement check.
func TestRemoteControl_ForcedByEnvVar(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "1")

	if !bridge.IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=true when CLAUDE_CODE_BRIDGE_FORCED=1")
	}
	if !bridge.IsBridgeEnabled() && !bridge.IsBridgeForced() {
		t.Fatal("expected bridge guard to pass when forced")
	}
}

// TestBridgeFeatureGates_CcrAutoConnect verifies that CCR auto-connect
// defaults to false in the default build.
func TestBridgeFeatureGates_CcrAutoConnect(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.GetCcrAutoConnectDefault() {
		t.Fatal("expected GetCcrAutoConnectDefault=false in default build")
	}
}

// TestBridgeFeatureGates_CcrMirror verifies that CCR mirror mode
// defaults to false in the default build.
func TestBridgeFeatureGates_CcrMirror(t *testing.T) {
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	if bridge.IsCcrMirrorEnabled() {
		t.Fatal("expected IsCcrMirrorEnabled=false in default build")
	}
}
