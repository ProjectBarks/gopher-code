// Source: src/bridge/bridgeEnabled.ts
package bridge

import (
	"fmt"
	"os"
	"strings"
	"sync"
)

// ---------------------------------------------------------------------------
// User-visible error strings (verbatim match with TS source)
// ---------------------------------------------------------------------------

const (
	// ErrBridgeNeedSubscription is shown when the user lacks a claude.ai subscription.
	ErrBridgeNeedSubscription = "Remote Control requires a claude.ai subscription. Run `claude auth login` to sign in with your claude.ai account."

	// ErrBridgeNeedProfileScope is shown when the token lacks user:profile scope.
	ErrBridgeNeedProfileScope = "Remote Control requires a full-scope login token. Long-lived tokens (from `claude setup-token` or CLAUDE_CODE_OAUTH_TOKEN) are limited to inference-only for security reasons. Run `claude auth login` to use Remote Control."

	// ErrBridgeNoOrganization is shown when organizationUuid is missing from OAuth account info.
	ErrBridgeNoOrganization = "Unable to determine your organization for Remote Control eligibility. Run `claude auth login` to refresh your account information."

	// ErrBridgeNotEnabled is shown when the GrowthBook gate rejects the account.
	ErrBridgeNotEnabled = "Remote Control is not yet enabled for your account."

	// ErrBridgeNotAvailable is shown in builds that lack the BRIDGE_MODE feature.
	ErrBridgeNotAvailable = "Remote Control is not available in this build."
)

// ErrBridgeVersionTooOld returns the version-floor error message.
// Uses a function because it interpolates the current and required versions.
func ErrBridgeVersionTooOld(currentVersion, minVersion string) string {
	return fmt.Sprintf(
		"Your version of Claude Code (%s) is too old for Remote Control.\nVersion %s or higher is required. Run `claude update` to update.",
		currentVersion, minVersion,
	)
}

// ---------------------------------------------------------------------------
// Dependency injection — injectable providers for auth and feature flags
// ---------------------------------------------------------------------------

// OAuthAccountInfo holds the subset of account info needed for bridge checks.
type OAuthAccountInfo struct {
	OrganizationUUID string
}

// BridgeDeps bundles the external dependencies for bridge enablement checks.
// Tests inject stubs; production code wires real implementations.
type BridgeDeps struct {
	// BuildFeatures — true when the build includes the feature.
	BridgeMode     bool
	CCRAutoConnect bool
	CCRMirror      bool

	// Auth helpers — return false if config is not yet initialized.
	IsClaudeAISubscriber func() bool
	HasProfileScope      func() bool
	GetOAuthAccountInfo  func() *OAuthAccountInfo

	// GrowthBook helpers.
	// GetFeatureValueBool returns a cached (possibly stale) boolean feature value.
	GetFeatureValueBool func(key string, defaultVal bool) bool
	// CheckGateBlocking returns a fresh boolean gate value (may block up to ~5s).
	CheckGateBlocking func(key string) (bool, error)
	// GetDynamicConfig returns a cached dynamic config map.
	GetDynamicConfig func(key string, defaults map[string]string) map[string]string

	// Version is the current CLI version string (e.g. "1.2.3").
	Version string

	// SemverLT returns true if a < b (semver comparison).
	SemverLT func(a, b string) bool
}

var (
	depsMu sync.RWMutex
	deps   *BridgeDeps
)

// SetBridgeDeps registers the dependency bundle for bridge enablement checks.
func SetBridgeDeps(d *BridgeDeps) {
	depsMu.Lock()
	defer depsMu.Unlock()
	deps = d
}

func getDeps() *BridgeDeps {
	depsMu.RLock()
	defer depsMu.RUnlock()
	return deps
}

// ---------------------------------------------------------------------------
// Public API
// ---------------------------------------------------------------------------

// IsBridgeEnabled is the fast (cached, possibly stale) check for bridge mode
// entitlement. Returns true only when the BRIDGE_MODE build feature is present,
// the user is a claude.ai subscriber, and the tengu_ccr_bridge GrowthBook
// flag is enabled.
func IsBridgeEnabled() bool {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return false
	}
	if !safeIsSubscriber(d) {
		return false
	}
	return d.GetFeatureValueBool("tengu_ccr_bridge", false)
}

// IsBridgeEnabledBlocking is the blocking entitlement check. Returns cached
// true immediately (fast path). If the disk cache says false or is missing,
// awaits GrowthBook init and fetches the fresh server value (slow path, max ~5s).
func IsBridgeEnabledBlocking() (bool, error) {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return false, nil
	}
	if !safeIsSubscriber(d) {
		return false, nil
	}
	return d.CheckGateBlocking("tengu_ccr_bridge")
}

// GetBridgeDisabledReason returns a user-facing diagnostic message for why
// Remote Control is unavailable, or "" if it is enabled. The GrowthBook gate
// targets on organizationUUID from the OAuth profile.
func GetBridgeDisabledReason() (string, error) {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return ErrBridgeNotAvailable, nil
	}
	if !safeIsSubscriber(d) {
		return ErrBridgeNeedSubscription, nil
	}
	if !safeHasProfileScope(d) {
		return ErrBridgeNeedProfileScope, nil
	}
	acct := safeGetOAuthAccountInfo(d)
	if acct == nil || acct.OrganizationUUID == "" {
		return ErrBridgeNoOrganization, nil
	}
	ok, err := d.CheckGateBlocking("tengu_ccr_bridge")
	if err != nil {
		return "", err
	}
	if !ok {
		return ErrBridgeNotEnabled, nil
	}
	return "", nil
}

// IsBridgeForced returns true when the CLAUDE_CODE_BRIDGE_FORCED env var is
// set to a truthy value ("1", "true", "yes"). This bypasses normal entitlement
// gates for internal testing.
func IsBridgeForced() bool {
	return isEnvTruthy(os.Getenv("CLAUDE_CODE_BRIDGE_FORCED"))
}

// IsEnvLessBridgeEnabled returns true when the GrowthBook flag
// tengu_bridge_repl_v2 is enabled. This gates which REPL bridge implementation
// is used, NOT whether bridge is available at all.
func IsEnvLessBridgeEnabled() bool {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return false
	}
	return d.GetFeatureValueBool("tengu_bridge_repl_v2", false)
}

// IsCseShimEnabled returns true when the cse_* -> session_* client-side retag
// shim is active. Defaults to true; can be remotely disabled via GrowthBook.
func IsCseShimEnabled() bool {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return true
	}
	return d.GetFeatureValueBool("tengu_bridge_repl_v2_cse_shim_enabled", true)
}

// CheckBridgeMinVersion returns an error message if the current CLI version
// is below the minimum required for Remote Control, or "" if the version is OK.
// Uses cached (non-blocking) GrowthBook config. Default min version is "0.0.0"
// which means the check always passes when GrowthBook hasn't loaded yet.
func CheckBridgeMinVersion() string {
	d := getDeps()
	if d == nil || !d.BridgeMode {
		return ""
	}
	config := d.GetDynamicConfig("tengu_bridge_min_version", map[string]string{
		"minVersion": "0.0.0",
	})
	minVer := config["minVersion"]
	if minVer != "" && d.SemverLT != nil && d.SemverLT(d.Version, minVer) {
		return ErrBridgeVersionTooOld(d.Version, minVer)
	}
	return ""
}

// GetCcrAutoConnectDefault returns the default for remoteControlAtStartup
// when the user hasn't explicitly set it. True when both the CCR_AUTO_CONNECT
// build flag is present and the tengu_cobalt_harbor GrowthBook gate is on.
func GetCcrAutoConnectDefault() bool {
	d := getDeps()
	if d == nil || !d.CCRAutoConnect {
		return false
	}
	return d.GetFeatureValueBool("tengu_cobalt_harbor", false)
}

// IsCcrMirrorEnabled returns true when CCR mirror mode is enabled. Env var
// CLAUDE_CODE_CCR_MIRROR wins for local opt-in; GrowthBook controls rollout.
func IsCcrMirrorEnabled() bool {
	d := getDeps()
	if d == nil || !d.CCRMirror {
		return false
	}
	if isEnvTruthy(os.Getenv("CLAUDE_CODE_CCR_MIRROR")) {
		return true
	}
	return d.GetFeatureValueBool("tengu_ccr_mirror", false)
}

// ---------------------------------------------------------------------------
// Internal helpers
// ---------------------------------------------------------------------------

// safeIsSubscriber wraps the auth check, returning false if the provider
// panics (mirrors the TS try/catch for pre-config access).
func safeIsSubscriber(d *BridgeDeps) bool {
	if d.IsClaudeAISubscriber == nil {
		return false
	}
	return d.IsClaudeAISubscriber()
}

func safeHasProfileScope(d *BridgeDeps) bool {
	if d.HasProfileScope == nil {
		return false
	}
	return d.HasProfileScope()
}

func safeGetOAuthAccountInfo(d *BridgeDeps) *OAuthAccountInfo {
	if d.GetOAuthAccountInfo == nil {
		return nil
	}
	return d.GetOAuthAccountInfo()
}

// isEnvTruthy checks if a string looks truthy (1, true, yes).
func isEnvTruthy(val string) bool {
	switch strings.ToLower(val) {
	case "1", "true", "yes":
		return true
	}
	return false
}
