// EnvLessBridgeConfig holds timing and behavior configuration for the
// env-less (v2) bridge path. Fetched from GrowthBook once per bridge
// session init; validated with whole-object rejection semantics.
// Source: src/bridge/envLessBridgeConfig.ts
package bridge

import (
	"encoding/json"
	"fmt"
	"math"
	"strconv"
	"strings"
)

// EnvLessBridgeConfig is the timing/behavior config for the env-less (v2)
// bridge. All fields have snake_case JSON tags matching the TS wire format.
type EnvLessBridgeConfig struct {
	InitRetryMaxAttempts       int     `json:"init_retry_max_attempts"`
	InitRetryBaseDelayMS       int     `json:"init_retry_base_delay_ms"`
	InitRetryJitterFraction    float64 `json:"init_retry_jitter_fraction"`
	InitRetryMaxDelayMS        int     `json:"init_retry_max_delay_ms"`
	HTTPTimeoutMS              int     `json:"http_timeout_ms"`
	UUIDDedupBufferSize        int     `json:"uuid_dedup_buffer_size"`
	HeartbeatIntervalMS        int     `json:"heartbeat_interval_ms"`
	HeartbeatJitterFraction    float64 `json:"heartbeat_jitter_fraction"`
	TokenRefreshBufferMS       int     `json:"token_refresh_buffer_ms"`
	TeardownArchiveTimeoutMS   int     `json:"teardown_archive_timeout_ms"`
	ConnectTimeoutMS           int     `json:"connect_timeout_ms"`
	MinVersion                 string  `json:"min_version"`
	ShouldShowAppUpgradeMessage bool   `json:"should_show_app_upgrade_message"`
}

// DefaultEnvLessBridgeConfig matches the TS DEFAULT_ENV_LESS_BRIDGE_CONFIG.
var DefaultEnvLessBridgeConfig = EnvLessBridgeConfig{
	InitRetryMaxAttempts:       3,
	InitRetryBaseDelayMS:       500,
	InitRetryJitterFraction:    0.25,
	InitRetryMaxDelayMS:        4000,
	HTTPTimeoutMS:              10_000,
	UUIDDedupBufferSize:        2000,
	HeartbeatIntervalMS:        20_000,
	HeartbeatJitterFraction:    0.1,
	TokenRefreshBufferMS:       300_000,
	TeardownArchiveTimeoutMS:   1500,
	ConnectTimeoutMS:           15_000,
	MinVersion:                 "0.0.0",
	ShouldShowAppUpgradeMessage: false,
}

// ValidateEnvLessBridgeConfig validates all fields against the bounds from the
// TS zod schema. Uses whole-object rejection: if ANY field is out of bounds,
// the entire config is rejected and the default is returned with ok=false.
func ValidateEnvLessBridgeConfig(c EnvLessBridgeConfig) (EnvLessBridgeConfig, bool) {
	if !inRange(c.InitRetryMaxAttempts, 1, 10) {
		return DefaultEnvLessBridgeConfig, false
	}
	if c.InitRetryBaseDelayMS < 100 {
		return DefaultEnvLessBridgeConfig, false
	}
	if !floatInRange(c.InitRetryJitterFraction, 0, 1) {
		return DefaultEnvLessBridgeConfig, false
	}
	if c.InitRetryMaxDelayMS < 500 {
		return DefaultEnvLessBridgeConfig, false
	}
	if c.HTTPTimeoutMS < 2000 {
		return DefaultEnvLessBridgeConfig, false
	}
	if !inRange(c.UUIDDedupBufferSize, 100, 50_000) {
		return DefaultEnvLessBridgeConfig, false
	}
	if !inRange(c.HeartbeatIntervalMS, 5000, 30_000) {
		return DefaultEnvLessBridgeConfig, false
	}
	if !floatInRange(c.HeartbeatJitterFraction, 0, 0.5) {
		return DefaultEnvLessBridgeConfig, false
	}
	if !inRange(c.TokenRefreshBufferMS, 30_000, 1_800_000) {
		return DefaultEnvLessBridgeConfig, false
	}
	if !inRange(c.TeardownArchiveTimeoutMS, 500, 2000) {
		return DefaultEnvLessBridgeConfig, false
	}
	if !inRange(c.ConnectTimeoutMS, 5000, 60_000) {
		return DefaultEnvLessBridgeConfig, false
	}
	if c.MinVersion != "" && !isValidSemver(c.MinVersion) {
		return DefaultEnvLessBridgeConfig, false
	}
	return c, true
}

// ParseEnvLessBridgeConfigJSON unmarshals raw JSON into an EnvLessBridgeConfig,
// validates it, and returns the default on any failure (parse or validation).
// Mirrors the TS safeParse + fallback pattern.
func ParseEnvLessBridgeConfigJSON(data []byte) EnvLessBridgeConfig {
	if len(data) == 0 {
		return DefaultEnvLessBridgeConfig
	}
	var c EnvLessBridgeConfig
	if err := json.Unmarshal(data, &c); err != nil {
		return DefaultEnvLessBridgeConfig
	}
	cfg, ok := ValidateEnvLessBridgeConfig(c)
	if !ok {
		return DefaultEnvLessBridgeConfig
	}
	return cfg
}

// CheckEnvLessBridgeMinVersion returns an error message if currentVersion is
// below cfg.MinVersion, or "" if the version is fine. Matches the TS
// checkEnvLessBridgeMinVersion verbatim message format.
func CheckEnvLessBridgeMinVersion(cfg EnvLessBridgeConfig, currentVersion string) string {
	if cfg.MinVersion == "" {
		return ""
	}
	if semverLT(currentVersion, cfg.MinVersion) {
		return fmt.Sprintf(
			"Your version of Claude Code (%s) is too old for Remote Control.\nVersion %s or higher is required. Run `claude update` to update.",
			currentVersion, cfg.MinVersion,
		)
	}
	return ""
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func inRange(v, min, max int) bool {
	return v >= min && v <= max
}

func floatInRange(v, min, max float64) bool {
	if math.IsNaN(v) || math.IsInf(v, 0) {
		return false
	}
	return v >= min && v <= max
}

// isValidSemver checks if s is a valid MAJOR.MINOR.PATCH semver string.
func isValidSemver(s string) bool {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return false
	}
	for _, p := range parts {
		if len(p) == 0 {
			return false
		}
		if _, err := strconv.Atoi(p); err != nil {
			return false
		}
	}
	return true
}

// parseSemver returns (major, minor, patch, ok).
func parseSemver(s string) (int, int, int, bool) {
	parts := strings.Split(s, ".")
	if len(parts) != 3 {
		return 0, 0, 0, false
	}
	major, err := strconv.Atoi(parts[0])
	if err != nil {
		return 0, 0, 0, false
	}
	minor, err := strconv.Atoi(parts[1])
	if err != nil {
		return 0, 0, 0, false
	}
	patch, err := strconv.Atoi(parts[2])
	if err != nil {
		return 0, 0, 0, false
	}
	return major, minor, patch, true
}

// semverLT returns true if a < b (strictly less than).
func semverLT(a, b string) bool {
	aMaj, aMin, aPat, aOK := parseSemver(a)
	bMaj, bMin, bPat, bOK := parseSemver(b)
	if !aOK || !bOK {
		return false
	}
	if aMaj != bMaj {
		return aMaj < bMaj
	}
	if aMin != bMin {
		return aMin < bMin
	}
	return aPat < bPat
}
