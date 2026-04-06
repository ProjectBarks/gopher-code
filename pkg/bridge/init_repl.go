// Package bridge — REPL-side bridge initializer.
// Runs pre-flight checks (enabled, OAuth, policy), then delegates to the
// bootstrap-free bridge core.
// Source: src/bridge/initReplBridge.ts
package bridge

import (
	"fmt"
	"time"
)

// ---------------------------------------------------------------------------
// InitReplBridgeHandle — returned by InitReplBridge on success
// ---------------------------------------------------------------------------

// InitReplBridgeHandle represents the result of a successful REPL bridge init.
// Wraps the underlying ReplBridge with the metadata needed by callers.
type InitReplBridgeHandle struct {
	Bridge    *ReplBridge
	OrgUUID   string
	BaseURL   string
}

// ---------------------------------------------------------------------------
// InitBridgeOptions — mirrors TS InitBridgeOptions (14 fields)
// ---------------------------------------------------------------------------

// InitBridgeOptions configures the REPL bridge init sequence.
type InitBridgeOptions struct {
	OnStateChange StateChangeFunc

	// InitialName is an explicit session name from `/remote-control <name>`.
	InitialName string

	// Perpetual enables assistant-mode session continuity via bridge-pointer.json.
	Perpetual bool

	// OutboundOnly — when true, the bridge only forwards events outbound
	// (no SSE inbound stream). Used by CCR mirror mode.
	OutboundOnly bool

	// Tags are optional metadata tags for the bridge session.
	Tags []string
}

// ---------------------------------------------------------------------------
// InitReplDeps — injectable dependencies for InitReplBridge
// ---------------------------------------------------------------------------

// OAuthTokens holds the OAuth token data needed for pre-flight checks.
type OAuthTokens struct {
	AccessToken string
	ExpiresAt   *int64 // nil means "never expires" (env/FD tokens)
}

// GlobalBridgeConfig holds the cross-process backoff fields from GlobalConfig.
type GlobalBridgeConfig struct {
	BridgeOauthDeadExpiresAt *int64
	BridgeOauthDeadFailCount int
}

// InitReplDeps bundles the external dependencies for InitReplBridge.
// Tests inject stubs; production code wires real implementations.
type InitReplDeps struct {
	// IsBridgeEnabledBlocking is the blocking entitlement check.
	IsBridgeEnabledBlocking func() (bool, error)

	// GetBridgeAccessToken returns the OAuth access token for bridge API calls.
	// Returns ("", false) when no token is available.
	GetBridgeAccessToken func() (string, bool)

	// GetBridgeTokenOverride returns the ant-only dev token override.
	// Returns ("", false) when unset.
	GetBridgeTokenOverride func() (string, bool)

	// WaitForPolicyLimitsToLoad blocks until policy limits are available.
	WaitForPolicyLimitsToLoad func() error

	// IsPolicyAllowed checks if a policy key is allowed.
	IsPolicyAllowed func(key string) bool

	// GetGlobalConfig returns the cross-process backoff state.
	GetGlobalConfig func() GlobalBridgeConfig

	// SaveGlobalConfig persists updated cross-process backoff state.
	SaveGlobalConfig func(cfg GlobalBridgeConfig)

	// GetOAuthTokens returns the current OAuth tokens for dead-token checks.
	GetOAuthTokens func() *OAuthTokens

	// CheckAndRefreshOAuthToken proactively refreshes the token if expired.
	CheckAndRefreshOAuthToken func() error

	// GetOrganizationUUID returns the org UUID for the current user.
	GetOrganizationUUID func() (string, error)

	// NowMillis returns the current time in Unix milliseconds.
	// Defaults to time.Now().UnixMilli() when nil.
	NowMillis func() int64

	// LogDebug logs a debug message. No-op when nil.
	LogDebug func(msg string)
}

// ---------------------------------------------------------------------------
// Constants
// ---------------------------------------------------------------------------

// titleMaxLenREPL is the maximum session title length before truncation
// in the REPL init path. Note: main.go defines TitleMaxLen=80 for the
// bridge-main path; this is the REPL-specific limit from TS initReplBridge.
const titleMaxLenREPL = 50

// deadTokenThreshold is the number of consecutive dead-token observations
// before cross-process backoff kicks in.
const deadTokenThreshold = 3

// ---------------------------------------------------------------------------
// Skip-reason strings (verbatim from TS)
// ---------------------------------------------------------------------------

const (
	skipNotEnabled          = "[bridge:repl] Skipping: bridge not enabled"
	skipNoOAuth             = "[bridge:repl] Skipping: no OAuth tokens"
	skipPolicyDenied        = "[bridge:repl] Skipping: allow_remote_control policy not allowed"
	skipOAuthExpired        = "[bridge:repl] Skipping: OAuth token expired and refresh failed (re-login required)"
	skipNoOrgUUID           = "[bridge:repl] Skipping: no org UUID"
)

// skipCrossProcessBackoff returns the formatted cross-process backoff message.
func skipCrossProcessBackoff(count int) string {
	return fmt.Sprintf("[bridge:repl] Skipping: cross-process backoff (dead token seen %d times)", count)
}

// skipVersionTooOld returns the formatted version-gate skip message.
func skipVersionTooOld(versionError string) string {
	return fmt.Sprintf("[bridge:repl] Skipping: %s", versionError)
}

// ---------------------------------------------------------------------------
// State-detail strings (verbatim from TS)
// ---------------------------------------------------------------------------

const (
	stateDetailLogin         = "/login"
	stateDetailPolicyDenied  = "disabled by your organization's policy"
	stateDetailUpdateNeeded  = "run `claude update` to upgrade"
)

// ---------------------------------------------------------------------------
// InitReplBridge — entry point
// ---------------------------------------------------------------------------

// InitReplBridge runs the REPL bridge pre-flight checks and, on success,
// returns an InitReplBridgeHandle. Returns (nil, nil) when the bridge is
// intentionally skipped (not enabled, no auth, policy denied, etc.).
// Returns (nil, err) only on unexpected errors.
func InitReplBridge(deps InitReplDeps, opts *InitBridgeOptions) (*InitReplBridgeHandle, error) {
	if opts == nil {
		opts = &InitBridgeOptions{}
	}

	nowMillis := deps.NowMillis
	if nowMillis == nil {
		nowMillis = func() int64 { return time.Now().UnixMilli() }
	}

	onStateChange := opts.OnStateChange
	if onStateChange == nil {
		onStateChange = func(BridgeState, string) {}
	}

	logDebug := deps.LogDebug
	if logDebug == nil {
		logDebug = func(string) {}
	}

	// Wire the cse_ shim kill switch so ToCompatSessionID respects the
	// GrowthBook gate. Daemon/SDK paths skip this — shim defaults to active.
	SetCseShimGate(IsCseShimEnabled)

	// 1. Runtime gate — is bridge enabled for this account?
	enabled, err := deps.IsBridgeEnabledBlocking()
	if err != nil {
		return nil, fmt.Errorf("bridge enabled check: %w", err)
	}
	if !enabled {
		LogBridgeSkip("not_enabled", skipNotEnabled, nil)
		return nil, nil
	}

	// 2. Check OAuth — must be signed in with claude.ai.
	if _, ok := deps.GetBridgeAccessToken(); !ok {
		LogBridgeSkip("no_oauth", skipNoOAuth, nil)
		onStateChange(BridgeStateFailed, stateDetailLogin)
		return nil, nil
	}

	// 3. Check organization policy — remote control may be disabled.
	if err := deps.WaitForPolicyLimitsToLoad(); err != nil {
		return nil, fmt.Errorf("policy limits load: %w", err)
	}
	if !deps.IsPolicyAllowed("allow_remote_control") {
		LogBridgeSkip("policy_denied", skipPolicyDenied, nil)
		onStateChange(BridgeStateFailed, stateDetailPolicyDenied)
		return nil, nil
	}

	// When CLAUDE_BRIDGE_OAUTH_TOKEN is set (ant-only local dev), the bridge
	// uses that token directly — keychain state is irrelevant. Skip 2a/2b/2c.
	if _, hasOverride := deps.GetBridgeTokenOverride(); !hasOverride {
		// 2a. Cross-process backoff. If N prior processes saw this exact dead
		// token (matched by expiresAt), skip silently.
		cfg := deps.GetGlobalConfig()
		if cfg.BridgeOauthDeadExpiresAt != nil &&
			cfg.BridgeOauthDeadFailCount >= deadTokenThreshold {
			tokens := deps.GetOAuthTokens()
			if tokens != nil && tokens.ExpiresAt != nil &&
				*tokens.ExpiresAt == *cfg.BridgeOauthDeadExpiresAt {
				logDebug(skipCrossProcessBackoff(cfg.BridgeOauthDeadFailCount))
				return nil, nil
			}
		}

		// 2b. Proactively refresh if expired.
		if deps.CheckAndRefreshOAuthToken != nil {
			_ = deps.CheckAndRefreshOAuthToken()
		}

		// 2c. Skip if token is still expired post-refresh-attempt.
		tokens := deps.GetOAuthTokens()
		if tokens != nil && tokens.ExpiresAt != nil && *tokens.ExpiresAt <= nowMillis() {
			LogBridgeSkip("oauth_expired_unrefreshable", skipOAuthExpired, nil)
			onStateChange(BridgeStateFailed, stateDetailLogin)

			// Persist for the next process.
			deadExpiresAt := *tokens.ExpiresAt
			oldCfg := deps.GetGlobalConfig()
			newCount := 1
			if oldCfg.BridgeOauthDeadExpiresAt != nil &&
				*oldCfg.BridgeOauthDeadExpiresAt == deadExpiresAt {
				newCount = oldCfg.BridgeOauthDeadFailCount + 1
			}
			deps.SaveGlobalConfig(GlobalBridgeConfig{
				BridgeOauthDeadExpiresAt: &deadExpiresAt,
				BridgeOauthDeadFailCount: newCount,
			})
			return nil, nil
		}
	}

	// 4. Fetch orgUUID — both v1 and v2 paths need it.
	orgUUID, err := deps.GetOrganizationUUID()
	if err != nil {
		return nil, fmt.Errorf("get organization UUID: %w", err)
	}
	if orgUUID == "" {
		LogBridgeSkip("no_org_uuid", skipNoOrgUUID, nil)
		onStateChange(BridgeStateFailed, stateDetailLogin)
		return nil, nil
	}

	// TODO(T195+): v1/v2 branch selection, version gates, session title
	// derivation, environment registration, and bridge core init are
	// deferred to subsequent tasks.

	// Placeholder: return a handle with metadata so callers can integrate.
	// The Bridge field is nil until the core init tasks are implemented.
	return &InitReplBridgeHandle{
		OrgUUID: orgUUID,
	}, nil
}
