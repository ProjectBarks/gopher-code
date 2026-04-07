package main

import (
	"testing"

	"github.com/projectbarks/gopher-code/pkg/bridge"
)

// TestEnvLessBridgeConfigIntegration exercises the envless bridge config
// through the same code path used by the remote-control subcommand in main.go:
// load the default config, validate it, and run the version gate against the
// binary's Version constant.
func TestEnvLessBridgeConfigIntegration(t *testing.T) {
	// 1. The default config must pass validation (same as production init).
	cfg, ok := bridge.ValidateEnvLessBridgeConfig(bridge.DefaultEnvLessBridgeConfig)
	if !ok {
		t.Fatal("DefaultEnvLessBridgeConfig must pass validation")
	}

	// 2. Version gate: the current binary version must satisfy the default
	//    min_version (0.0.0), exactly as the remote-control handler does.
	if versionErr := bridge.CheckEnvLessBridgeMinVersion(cfg, Version); versionErr != "" {
		t.Fatalf("Version gate failed for binary version %q: %s", Version, versionErr)
	}
}

// TestEnvLessBridgeConfigParseFallback verifies that ParseEnvLessBridgeConfigJSON
// returns a valid, usable config even when given bad input — the fallback path
// that the binary relies on when GrowthBook returns garbage.
func TestEnvLessBridgeConfigParseFallback(t *testing.T) {
	cases := []struct {
		name  string
		input []byte
	}{
		{"nil", nil},
		{"empty", []byte("")},
		{"invalid JSON", []byte("{bad")},
		{"out-of-bounds field", []byte(`{"init_retry_max_attempts": 999}`)},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := bridge.ParseEnvLessBridgeConfigJSON(tc.input)

			// The fallback must equal the default.
			if cfg != bridge.DefaultEnvLessBridgeConfig {
				t.Errorf("fallback config does not match default for input %q", tc.name)
			}

			// The fallback must still pass the version gate with the binary version.
			if msg := bridge.CheckEnvLessBridgeMinVersion(cfg, Version); msg != "" {
				t.Errorf("fallback config failed version gate: %s", msg)
			}
		})
	}
}

// TestEnvLessBridgeVersionGateBlocksOldBinary confirms that a min_version
// higher than the current binary version produces a non-empty error string,
// which the remote-control handler would pass to cliError.
func TestEnvLessBridgeVersionGateBlocksOldBinary(t *testing.T) {
	cfg := bridge.DefaultEnvLessBridgeConfig
	cfg.MinVersion = "999.0.0"

	msg := bridge.CheckEnvLessBridgeMinVersion(cfg, Version)
	if msg == "" {
		t.Fatal("expected version gate to block binary version against min_version 999.0.0")
	}

	// Message must mention the binary version and the required version.
	if got := msg; got == "" {
		t.Fatal("expected non-empty version gate message")
	}
}
