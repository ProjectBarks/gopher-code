package bridge

import (
	"os"
	"testing"
)

// ---------------------------------------------------------------------------
// Test helpers
// ---------------------------------------------------------------------------

// allPassingDeps returns a BridgeDeps with all gates passing.
func allPassingDeps() *BridgeDeps {
	return &BridgeDeps{
		BridgeMode:     true,
		CCRAutoConnect: true,
		CCRMirror:      true,
		IsClaudeAISubscriber: func() bool { return true },
		HasProfileScope:      func() bool { return true },
		GetOAuthAccountInfo: func() *OAuthAccountInfo {
			return &OAuthAccountInfo{OrganizationUUID: "org-123"}
		},
		GetFeatureValueBool: func(key string, def bool) bool {
			switch key {
			case "tengu_ccr_bridge", "tengu_bridge_repl_v2", "tengu_cobalt_harbor", "tengu_ccr_mirror":
				return true
			case "tengu_bridge_repl_v2_cse_shim_enabled":
				return true
			}
			return def
		},
		CheckGateBlocking: func(key string) (bool, error) { return true, nil },
		GetDynamicConfig: func(key string, defaults map[string]string) map[string]string {
			return defaults
		},
		Version:  "1.0.0",
		SemverLT: func(a, b string) bool { return a < b }, // simplified for tests
	}
}

func withDeps(t *testing.T, d *BridgeDeps, fn func()) {
	t.Helper()
	old := deps
	SetBridgeDeps(d)
	defer SetBridgeDeps(old)
	fn()
}

// ---------------------------------------------------------------------------
// IsBridgeEnabled
// ---------------------------------------------------------------------------

func TestIsBridgeEnabled_AllGatesPassing(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if !IsBridgeEnabled() {
			t.Fatal("expected IsBridgeEnabled=true when all gates pass")
		}
	})
}

func TestIsBridgeEnabled_DisabledWhenFeatureFlagOff(t *testing.T) {
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		if IsBridgeEnabled() {
			t.Fatal("expected IsBridgeEnabled=false when BridgeMode is off")
		}
	})
}

func TestIsBridgeEnabled_DisabledWhenNotSubscriber(t *testing.T) {
	d := allPassingDeps()
	d.IsClaudeAISubscriber = func() bool { return false }
	withDeps(t, d, func() {
		if IsBridgeEnabled() {
			t.Fatal("expected IsBridgeEnabled=false when not subscriber")
		}
	})
}

func TestIsBridgeEnabled_DisabledWhenGrowthBookGateOff(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool { return false }
	withDeps(t, d, func() {
		if IsBridgeEnabled() {
			t.Fatal("expected IsBridgeEnabled=false when GrowthBook gate is off")
		}
	})
}

func TestIsBridgeEnabled_NilDeps(t *testing.T) {
	withDeps(t, nil, func() {
		if IsBridgeEnabled() {
			t.Fatal("expected IsBridgeEnabled=false when deps are nil")
		}
	})
}

// ---------------------------------------------------------------------------
// IsBridgeEnabledBlocking
// ---------------------------------------------------------------------------

func TestIsBridgeEnabledBlocking_AllGatesPassing(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		ok, err := IsBridgeEnabledBlocking()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !ok {
			t.Fatal("expected IsBridgeEnabledBlocking=true when all gates pass")
		}
	})
}

func TestIsBridgeEnabledBlocking_DisabledWhenFeatureFlagOff(t *testing.T) {
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		ok, err := IsBridgeEnabledBlocking()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected IsBridgeEnabledBlocking=false when BridgeMode off")
		}
	})
}

func TestIsBridgeEnabledBlocking_DisabledWhenNotSubscriber(t *testing.T) {
	d := allPassingDeps()
	d.IsClaudeAISubscriber = func() bool { return false }
	withDeps(t, d, func() {
		ok, err := IsBridgeEnabledBlocking()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if ok {
			t.Fatal("expected IsBridgeEnabledBlocking=false when not subscriber")
		}
	})
}

// ---------------------------------------------------------------------------
// GetBridgeDisabledReason
// ---------------------------------------------------------------------------

func TestGetBridgeDisabledReason_Nil_WhenEnabled(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != "" {
			t.Fatalf("expected empty reason when enabled, got: %q", reason)
		}
	})
}

func TestGetBridgeDisabledReason_NotAvailableInBuild(t *testing.T) {
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNotAvailable {
			t.Fatalf("expected %q, got %q", ErrBridgeNotAvailable, reason)
		}
	})
}

func TestGetBridgeDisabledReason_NeedSubscription(t *testing.T) {
	d := allPassingDeps()
	d.IsClaudeAISubscriber = func() bool { return false }
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNeedSubscription {
			t.Fatalf("expected %q, got %q", ErrBridgeNeedSubscription, reason)
		}
	})
}

func TestGetBridgeDisabledReason_NeedProfileScope(t *testing.T) {
	d := allPassingDeps()
	d.HasProfileScope = func() bool { return false }
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNeedProfileScope {
			t.Fatalf("expected %q, got %q", ErrBridgeNeedProfileScope, reason)
		}
	})
}

func TestGetBridgeDisabledReason_NoOrganization(t *testing.T) {
	d := allPassingDeps()
	d.GetOAuthAccountInfo = func() *OAuthAccountInfo {
		return &OAuthAccountInfo{OrganizationUUID: ""}
	}
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNoOrganization {
			t.Fatalf("expected %q, got %q", ErrBridgeNoOrganization, reason)
		}
	})
}

func TestGetBridgeDisabledReason_NoOrganization_NilAccountInfo(t *testing.T) {
	d := allPassingDeps()
	d.GetOAuthAccountInfo = func() *OAuthAccountInfo { return nil }
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNoOrganization {
			t.Fatalf("expected %q, got %q", ErrBridgeNoOrganization, reason)
		}
	})
}

func TestGetBridgeDisabledReason_NotEnabledForAccount(t *testing.T) {
	d := allPassingDeps()
	d.CheckGateBlocking = func(key string) (bool, error) { return false, nil }
	withDeps(t, d, func() {
		reason, err := GetBridgeDisabledReason()
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if reason != ErrBridgeNotEnabled {
			t.Fatalf("expected %q, got %q", ErrBridgeNotEnabled, reason)
		}
	})
}

// ---------------------------------------------------------------------------
// IsBridgeForced
// ---------------------------------------------------------------------------

func TestIsBridgeForced_True(t *testing.T) {
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "1")
	if !IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=true when env is '1'")
	}
}

func TestIsBridgeForced_TrueString(t *testing.T) {
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "true")
	if !IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=true when env is 'true'")
	}
}

func TestIsBridgeForced_FalseWhenUnset(t *testing.T) {
	// Ensure env is not set (t.Setenv restores after test).
	os.Unsetenv("CLAUDE_CODE_BRIDGE_FORCED")
	if IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=false when env is unset")
	}
}

func TestIsBridgeForced_FalseWhenEmpty(t *testing.T) {
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "")
	if IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=false when env is empty")
	}
}

func TestIsBridgeForced_FalseWhenZero(t *testing.T) {
	t.Setenv("CLAUDE_CODE_BRIDGE_FORCED", "0")
	if IsBridgeForced() {
		t.Fatal("expected IsBridgeForced=false when env is '0'")
	}
}

// ---------------------------------------------------------------------------
// IsEnvLessBridgeEnabled
// ---------------------------------------------------------------------------

func TestIsEnvLessBridgeEnabled_True(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if !IsEnvLessBridgeEnabled() {
			t.Fatal("expected IsEnvLessBridgeEnabled=true")
		}
	})
}

func TestIsEnvLessBridgeEnabled_FalseWhenBridgeModeOff(t *testing.T) {
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		if IsEnvLessBridgeEnabled() {
			t.Fatal("expected IsEnvLessBridgeEnabled=false when BridgeMode off")
		}
	})
}

func TestIsEnvLessBridgeEnabled_FalseWhenGBOff(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool {
		if key == "tengu_bridge_repl_v2" {
			return false
		}
		return def
	}
	withDeps(t, d, func() {
		if IsEnvLessBridgeEnabled() {
			t.Fatal("expected IsEnvLessBridgeEnabled=false when GB flag off")
		}
	})
}

// ---------------------------------------------------------------------------
// IsCseShimEnabled
// ---------------------------------------------------------------------------

func TestIsCseShimEnabled_DefaultTrue(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if !IsCseShimEnabled() {
			t.Fatal("expected IsCseShimEnabled=true by default")
		}
	})
}

func TestIsCseShimEnabled_TrueWhenBridgeModeOff(t *testing.T) {
	// TS returns true when feature('BRIDGE_MODE') is false.
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		if !IsCseShimEnabled() {
			t.Fatal("expected IsCseShimEnabled=true even when BridgeMode off")
		}
	})
}

func TestIsCseShimEnabled_CanBeDisabled(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool {
		if key == "tengu_bridge_repl_v2_cse_shim_enabled" {
			return false
		}
		return def
	}
	withDeps(t, d, func() {
		if IsCseShimEnabled() {
			t.Fatal("expected IsCseShimEnabled=false when GB disables it")
		}
	})
}

// ---------------------------------------------------------------------------
// CheckBridgeMinVersion
// ---------------------------------------------------------------------------

func TestCheckBridgeMinVersion_PassesWhenDefault(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if msg := CheckBridgeMinVersion(); msg != "" {
			t.Fatalf("expected empty, got: %q", msg)
		}
	})
}

func TestCheckBridgeMinVersion_FailsWhenVersionTooOld(t *testing.T) {
	d := allPassingDeps()
	d.Version = "0.9.0"
	d.SemverLT = func(a, b string) bool { return true } // always below
	d.GetDynamicConfig = func(key string, defaults map[string]string) map[string]string {
		return map[string]string{"minVersion": "1.5.0"}
	}
	withDeps(t, d, func() {
		msg := CheckBridgeMinVersion()
		want := ErrBridgeVersionTooOld("0.9.0", "1.5.0")
		if msg != want {
			t.Fatalf("expected %q, got %q", want, msg)
		}
	})
}

func TestCheckBridgeMinVersion_EmptyWhenBridgeModeOff(t *testing.T) {
	d := allPassingDeps()
	d.BridgeMode = false
	withDeps(t, d, func() {
		if msg := CheckBridgeMinVersion(); msg != "" {
			t.Fatalf("expected empty when BridgeMode off, got: %q", msg)
		}
	})
}

// ---------------------------------------------------------------------------
// GetCcrAutoConnectDefault
// ---------------------------------------------------------------------------

func TestGetCcrAutoConnectDefault_TrueWhenAllGatesPass(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if !GetCcrAutoConnectDefault() {
			t.Fatal("expected GetCcrAutoConnectDefault=true")
		}
	})
}

func TestGetCcrAutoConnectDefault_FalseWhenFeatureOff(t *testing.T) {
	d := allPassingDeps()
	d.CCRAutoConnect = false
	withDeps(t, d, func() {
		if GetCcrAutoConnectDefault() {
			t.Fatal("expected GetCcrAutoConnectDefault=false when CCRAutoConnect off")
		}
	})
}

func TestGetCcrAutoConnectDefault_FalseWhenGBOff(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool {
		if key == "tengu_cobalt_harbor" {
			return false
		}
		return def
	}
	withDeps(t, d, func() {
		if GetCcrAutoConnectDefault() {
			t.Fatal("expected GetCcrAutoConnectDefault=false when GB gate off")
		}
	})
}

// ---------------------------------------------------------------------------
// IsCcrMirrorEnabled
// ---------------------------------------------------------------------------

func TestIsCcrMirrorEnabled_TrueViaGrowthBook(t *testing.T) {
	withDeps(t, allPassingDeps(), func() {
		if !IsCcrMirrorEnabled() {
			t.Fatal("expected IsCcrMirrorEnabled=true via GB")
		}
	})
}

func TestIsCcrMirrorEnabled_TrueViaEnvVar(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool { return false }
	t.Setenv("CLAUDE_CODE_CCR_MIRROR", "1")
	withDeps(t, d, func() {
		if !IsCcrMirrorEnabled() {
			t.Fatal("expected IsCcrMirrorEnabled=true via env var")
		}
	})
}

func TestIsCcrMirrorEnabled_FalseWhenFeatureOff(t *testing.T) {
	d := allPassingDeps()
	d.CCRMirror = false
	withDeps(t, d, func() {
		if IsCcrMirrorEnabled() {
			t.Fatal("expected IsCcrMirrorEnabled=false when CCRMirror off")
		}
	})
}

func TestIsCcrMirrorEnabled_FalseWhenBothOff(t *testing.T) {
	d := allPassingDeps()
	d.GetFeatureValueBool = func(key string, def bool) bool { return false }
	os.Unsetenv("CLAUDE_CODE_CCR_MIRROR")
	withDeps(t, d, func() {
		if IsCcrMirrorEnabled() {
			t.Fatal("expected IsCcrMirrorEnabled=false when env and GB both off")
		}
	})
}

// ---------------------------------------------------------------------------
// Verbatim string checks
// ---------------------------------------------------------------------------

func TestVerbatimErrorStrings(t *testing.T) {
	cases := []struct {
		name string
		got  string
		want string
	}{
		{
			"ErrBridgeNeedSubscription",
			ErrBridgeNeedSubscription,
			"Remote Control requires a claude.ai subscription. Run `claude auth login` to sign in with your claude.ai account.",
		},
		{
			"ErrBridgeNeedProfileScope",
			ErrBridgeNeedProfileScope,
			"Remote Control requires a full-scope login token. Long-lived tokens (from `claude setup-token` or CLAUDE_CODE_OAUTH_TOKEN) are limited to inference-only for security reasons. Run `claude auth login` to use Remote Control.",
		},
		{
			"ErrBridgeNoOrganization",
			ErrBridgeNoOrganization,
			"Unable to determine your organization for Remote Control eligibility. Run `claude auth login` to refresh your account information.",
		},
		{
			"ErrBridgeNotEnabled",
			ErrBridgeNotEnabled,
			"Remote Control is not yet enabled for your account.",
		},
		{
			"ErrBridgeNotAvailable",
			ErrBridgeNotAvailable,
			"Remote Control is not available in this build.",
		},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			if tc.got != tc.want {
				t.Errorf("mismatch:\n got: %q\nwant: %q", tc.got, tc.want)
			}
		})
	}
}

func TestErrBridgeVersionTooOld(t *testing.T) {
	got := ErrBridgeVersionTooOld("1.0.0", "2.0.0")
	want := "Your version of Claude Code (1.0.0) is too old for Remote Control.\nVersion 2.0.0 or higher is required. Run `claude update` to update."
	if got != want {
		t.Errorf("mismatch:\n got: %q\nwant: %q", got, want)
	}
}

// ---------------------------------------------------------------------------
// Safe helper nil-guard tests
// ---------------------------------------------------------------------------

func TestSafeHelpers_NilFuncs(t *testing.T) {
	d := &BridgeDeps{BridgeMode: true}
	if safeIsSubscriber(d) {
		t.Error("expected false for nil IsClaudeAISubscriber")
	}
	if safeHasProfileScope(d) {
		t.Error("expected false for nil HasProfileScope")
	}
	if safeGetOAuthAccountInfo(d) != nil {
		t.Error("expected nil for nil GetOAuthAccountInfo")
	}
}
