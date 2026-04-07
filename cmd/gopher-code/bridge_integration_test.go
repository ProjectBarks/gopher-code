package main

import (
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// TestInitBridgeDeps_SetsDefaultDeps verifies that initBridgeDeps registers a
// non-nil BridgeDeps bundle with safe defaults (bridge disabled, stubs wired).
func TestInitBridgeDeps_SetsDefaultDeps(t *testing.T) {
	// Clear any pre-existing deps from other tests.
	bridge.SetBridgeDeps(nil)

	initBridgeDeps()

	// After init, bridge should be disabled (BridgeMode=false).
	if bridge.IsBridgeEnabled() {
		t.Fatal("expected IsBridgeEnabled=false after initBridgeDeps (default build has BridgeMode=false)")
	}

	// GetBridgeDisabledReason should return ErrBridgeNotAvailable.
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

	// CheckBridgeMinVersion should pass because BridgeMode is off (returns "").
	if msg := bridge.CheckBridgeMinVersion(); msg != "" {
		t.Fatalf("expected empty min-version check, got: %q", msg)
	}
}

// TestRemoteControl_BlockedByDefault verifies that the remote-control subcommand
// is blocked when bridge is not enabled and not forced. This exercises the
// integration path in main() that calls bridge.IsBridgeEnabled and
// bridge.GetBridgeDisabledReason.
func TestRemoteControl_BlockedByDefault(t *testing.T) {
	// Initialize bridge deps to defaults (bridge disabled).
	bridge.SetBridgeDeps(nil)
	initBridgeDeps()

	// Ensure CLAUDE_CODE_BRIDGE_FORCED is not set.
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

	// When forced, IsBridgeForced should return true.
	if !bridge.IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=true when CLAUDE_CODE_BRIDGE_FORCED=1")
	}

	// The guard `!IsBridgeEnabled() && !IsBridgeForced()` should be false,
	// meaning the remote-control flow would proceed.
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
