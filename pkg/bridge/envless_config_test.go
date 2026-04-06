package bridge

import (
	"encoding/json"
	"math"
	"strings"
	"testing"
)

// ---------------------------------------------------------------------------
// Default config
// ---------------------------------------------------------------------------

func TestDefaultEnvLessBridgeConfig(t *testing.T) {
	d := DefaultEnvLessBridgeConfig

	checks := []struct {
		name string
		got  any
		want any
	}{
		{"InitRetryMaxAttempts", d.InitRetryMaxAttempts, 3},
		{"InitRetryBaseDelayMS", d.InitRetryBaseDelayMS, 500},
		{"InitRetryJitterFraction", d.InitRetryJitterFraction, 0.25},
		{"InitRetryMaxDelayMS", d.InitRetryMaxDelayMS, 4000},
		{"HTTPTimeoutMS", d.HTTPTimeoutMS, 10_000},
		{"UUIDDedupBufferSize", d.UUIDDedupBufferSize, 2000},
		{"HeartbeatIntervalMS", d.HeartbeatIntervalMS, 20_000},
		{"HeartbeatJitterFraction", d.HeartbeatJitterFraction, 0.1},
		{"TokenRefreshBufferMS", d.TokenRefreshBufferMS, 300_000},
		{"TeardownArchiveTimeoutMS", d.TeardownArchiveTimeoutMS, 1500},
		{"ConnectTimeoutMS", d.ConnectTimeoutMS, 15_000},
		{"MinVersion", d.MinVersion, "0.0.0"},
		{"ShouldShowAppUpgradeMessage", d.ShouldShowAppUpgradeMessage, false},
	}
	for _, c := range checks {
		switch want := c.want.(type) {
		case int:
			if got, ok := c.got.(int); !ok || got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		case float64:
			if got, ok := c.got.(float64); !ok || got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		case string:
			if got, ok := c.got.(string); !ok || got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		case bool:
			if got, ok := c.got.(bool); !ok || got != want {
				t.Errorf("%s = %v, want %v", c.name, c.got, c.want)
			}
		}
	}
}

// ---------------------------------------------------------------------------
// JSON tags match TS wire format
// ---------------------------------------------------------------------------

func TestEnvLessBridgeConfigJSONKeys(t *testing.T) {
	b, err := json.Marshal(DefaultEnvLessBridgeConfig)
	if err != nil {
		t.Fatal(err)
	}

	var raw map[string]json.RawMessage
	if err := json.Unmarshal(b, &raw); err != nil {
		t.Fatal(err)
	}

	wantKeys := []string{
		"init_retry_max_attempts",
		"init_retry_base_delay_ms",
		"init_retry_jitter_fraction",
		"init_retry_max_delay_ms",
		"http_timeout_ms",
		"uuid_dedup_buffer_size",
		"heartbeat_interval_ms",
		"heartbeat_jitter_fraction",
		"token_refresh_buffer_ms",
		"teardown_archive_timeout_ms",
		"connect_timeout_ms",
		"min_version",
		"should_show_app_upgrade_message",
	}
	for _, key := range wantKeys {
		if _, ok := raw[key]; !ok {
			t.Errorf("missing JSON key %q", key)
		}
	}
	if len(raw) != len(wantKeys) {
		t.Errorf("JSON has %d keys, want %d", len(raw), len(wantKeys))
	}
}

// ---------------------------------------------------------------------------
// Validation — valid input
// ---------------------------------------------------------------------------

func TestValidateEnvLessBridgeConfig_ValidInput(t *testing.T) {
	input := EnvLessBridgeConfig{
		InitRetryMaxAttempts:       5,
		InitRetryBaseDelayMS:       200,
		InitRetryJitterFraction:    0.5,
		InitRetryMaxDelayMS:        2000,
		HTTPTimeoutMS:              5000,
		UUIDDedupBufferSize:        500,
		HeartbeatIntervalMS:        10_000,
		HeartbeatJitterFraction:    0.3,
		TokenRefreshBufferMS:       60_000,
		TeardownArchiveTimeoutMS:   1000,
		ConnectTimeoutMS:           30_000,
		MinVersion:                 "1.2.3",
		ShouldShowAppUpgradeMessage: true,
	}

	got, ok := ValidateEnvLessBridgeConfig(input)
	if !ok {
		t.Fatal("expected valid config to pass validation")
	}
	if got.InitRetryMaxAttempts != 5 {
		t.Errorf("InitRetryMaxAttempts = %d, want 5", got.InitRetryMaxAttempts)
	}
	if got.ShouldShowAppUpgradeMessage != true {
		t.Error("ShouldShowAppUpgradeMessage should be true")
	}
}

// ---------------------------------------------------------------------------
// Validation — whole-object rejection on any field violation
// ---------------------------------------------------------------------------

func TestValidateEnvLessBridgeConfig_RejectsWholeObject(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*EnvLessBridgeConfig)
	}{
		{"InitRetryMaxAttempts too low", func(c *EnvLessBridgeConfig) { c.InitRetryMaxAttempts = 0 }},
		{"InitRetryMaxAttempts too high", func(c *EnvLessBridgeConfig) { c.InitRetryMaxAttempts = 11 }},
		{"InitRetryBaseDelayMS too low", func(c *EnvLessBridgeConfig) { c.InitRetryBaseDelayMS = 99 }},
		{"InitRetryJitterFraction negative", func(c *EnvLessBridgeConfig) { c.InitRetryJitterFraction = -0.1 }},
		{"InitRetryJitterFraction too high", func(c *EnvLessBridgeConfig) { c.InitRetryJitterFraction = 1.1 }},
		{"InitRetryMaxDelayMS too low", func(c *EnvLessBridgeConfig) { c.InitRetryMaxDelayMS = 499 }},
		{"HTTPTimeoutMS too low", func(c *EnvLessBridgeConfig) { c.HTTPTimeoutMS = 1999 }},
		{"UUIDDedupBufferSize too low", func(c *EnvLessBridgeConfig) { c.UUIDDedupBufferSize = 99 }},
		{"UUIDDedupBufferSize too high", func(c *EnvLessBridgeConfig) { c.UUIDDedupBufferSize = 50_001 }},
		{"HeartbeatIntervalMS too low", func(c *EnvLessBridgeConfig) { c.HeartbeatIntervalMS = 4999 }},
		{"HeartbeatIntervalMS too high", func(c *EnvLessBridgeConfig) { c.HeartbeatIntervalMS = 30_001 }},
		{"HeartbeatJitterFraction negative", func(c *EnvLessBridgeConfig) { c.HeartbeatJitterFraction = -0.01 }},
		{"HeartbeatJitterFraction too high", func(c *EnvLessBridgeConfig) { c.HeartbeatJitterFraction = 0.51 }},
		{"TokenRefreshBufferMS too low", func(c *EnvLessBridgeConfig) { c.TokenRefreshBufferMS = 29_999 }},
		{"TokenRefreshBufferMS too high", func(c *EnvLessBridgeConfig) { c.TokenRefreshBufferMS = 1_800_001 }},
		{"TeardownArchiveTimeoutMS too low", func(c *EnvLessBridgeConfig) { c.TeardownArchiveTimeoutMS = 499 }},
		{"TeardownArchiveTimeoutMS too high", func(c *EnvLessBridgeConfig) { c.TeardownArchiveTimeoutMS = 2001 }},
		{"ConnectTimeoutMS too low", func(c *EnvLessBridgeConfig) { c.ConnectTimeoutMS = 4999 }},
		{"ConnectTimeoutMS too high", func(c *EnvLessBridgeConfig) { c.ConnectTimeoutMS = 60_001 }},
		{"MinVersion invalid semver", func(c *EnvLessBridgeConfig) { c.MinVersion = "not-a-version" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultEnvLessBridgeConfig // copy
			tc.modify(&cfg)
			got, ok := ValidateEnvLessBridgeConfig(cfg)
			if ok {
				t.Errorf("expected rejection for %s, but got ok=true", tc.name)
			}
			// On rejection, returned config must equal default.
			if got != DefaultEnvLessBridgeConfig {
				t.Errorf("rejected config should equal default")
			}
		})
	}
}

// ---------------------------------------------------------------------------
// Boundary values that should pass
// ---------------------------------------------------------------------------

func TestValidateEnvLessBridgeConfig_BoundaryPass(t *testing.T) {
	cases := []struct {
		name   string
		modify func(*EnvLessBridgeConfig)
	}{
		{"InitRetryMaxAttempts min", func(c *EnvLessBridgeConfig) { c.InitRetryMaxAttempts = 1 }},
		{"InitRetryMaxAttempts max", func(c *EnvLessBridgeConfig) { c.InitRetryMaxAttempts = 10 }},
		{"InitRetryBaseDelayMS min", func(c *EnvLessBridgeConfig) { c.InitRetryBaseDelayMS = 100 }},
		{"InitRetryJitterFraction zero", func(c *EnvLessBridgeConfig) { c.InitRetryJitterFraction = 0 }},
		{"InitRetryJitterFraction one", func(c *EnvLessBridgeConfig) { c.InitRetryJitterFraction = 1.0 }},
		{"InitRetryMaxDelayMS min", func(c *EnvLessBridgeConfig) { c.InitRetryMaxDelayMS = 500 }},
		{"HTTPTimeoutMS min", func(c *EnvLessBridgeConfig) { c.HTTPTimeoutMS = 2000 }},
		{"UUIDDedupBufferSize min", func(c *EnvLessBridgeConfig) { c.UUIDDedupBufferSize = 100 }},
		{"UUIDDedupBufferSize max", func(c *EnvLessBridgeConfig) { c.UUIDDedupBufferSize = 50_000 }},
		{"HeartbeatIntervalMS min", func(c *EnvLessBridgeConfig) { c.HeartbeatIntervalMS = 5000 }},
		{"HeartbeatIntervalMS max", func(c *EnvLessBridgeConfig) { c.HeartbeatIntervalMS = 30_000 }},
		{"HeartbeatJitterFraction zero", func(c *EnvLessBridgeConfig) { c.HeartbeatJitterFraction = 0 }},
		{"HeartbeatJitterFraction max", func(c *EnvLessBridgeConfig) { c.HeartbeatJitterFraction = 0.5 }},
		{"TokenRefreshBufferMS min", func(c *EnvLessBridgeConfig) { c.TokenRefreshBufferMS = 30_000 }},
		{"TokenRefreshBufferMS max", func(c *EnvLessBridgeConfig) { c.TokenRefreshBufferMS = 1_800_000 }},
		{"TeardownArchiveTimeoutMS min", func(c *EnvLessBridgeConfig) { c.TeardownArchiveTimeoutMS = 500 }},
		{"TeardownArchiveTimeoutMS max", func(c *EnvLessBridgeConfig) { c.TeardownArchiveTimeoutMS = 2000 }},
		{"ConnectTimeoutMS min", func(c *EnvLessBridgeConfig) { c.ConnectTimeoutMS = 5000 }},
		{"ConnectTimeoutMS max", func(c *EnvLessBridgeConfig) { c.ConnectTimeoutMS = 60_000 }},
		{"MinVersion valid semver", func(c *EnvLessBridgeConfig) { c.MinVersion = "2.0.0" }},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			cfg := DefaultEnvLessBridgeConfig // copy
			tc.modify(&cfg)
			_, ok := ValidateEnvLessBridgeConfig(cfg)
			if !ok {
				t.Errorf("expected boundary value to pass for %s", tc.name)
			}
		})
	}
}

// ---------------------------------------------------------------------------
// JSON round-trip
// ---------------------------------------------------------------------------

func TestEnvLessBridgeConfigJSONRoundTrip(t *testing.T) {
	orig := EnvLessBridgeConfig{
		InitRetryMaxAttempts:       5,
		InitRetryBaseDelayMS:       250,
		InitRetryJitterFraction:    0.3,
		InitRetryMaxDelayMS:        3000,
		HTTPTimeoutMS:              8000,
		UUIDDedupBufferSize:        1000,
		HeartbeatIntervalMS:        15_000,
		HeartbeatJitterFraction:    0.2,
		TokenRefreshBufferMS:       120_000,
		TeardownArchiveTimeoutMS:   1200,
		ConnectTimeoutMS:           20_000,
		MinVersion:                 "1.0.0",
		ShouldShowAppUpgradeMessage: true,
	}

	b, err := json.Marshal(orig)
	if err != nil {
		t.Fatal(err)
	}

	var decoded EnvLessBridgeConfig
	if err := json.Unmarshal(b, &decoded); err != nil {
		t.Fatal(err)
	}

	if decoded != orig {
		t.Errorf("round-trip mismatch:\n got: %+v\nwant: %+v", decoded, orig)
	}
}

// ---------------------------------------------------------------------------
// ParseEnvLessBridgeConfigJSON — parse raw JSON with fallback
// ---------------------------------------------------------------------------

func TestParseEnvLessBridgeConfigJSON_ValidJSON(t *testing.T) {
	raw := `{
		"init_retry_max_attempts": 5,
		"init_retry_base_delay_ms": 200,
		"init_retry_jitter_fraction": 0.5,
		"init_retry_max_delay_ms": 2000,
		"http_timeout_ms": 5000,
		"uuid_dedup_buffer_size": 500,
		"heartbeat_interval_ms": 10000,
		"heartbeat_jitter_fraction": 0.3,
		"token_refresh_buffer_ms": 60000,
		"teardown_archive_timeout_ms": 1000,
		"connect_timeout_ms": 30000,
		"min_version": "1.2.3",
		"should_show_app_upgrade_message": true
	}`

	cfg := ParseEnvLessBridgeConfigJSON([]byte(raw))
	if cfg.InitRetryMaxAttempts != 5 {
		t.Errorf("InitRetryMaxAttempts = %d, want 5", cfg.InitRetryMaxAttempts)
	}
	if cfg.MinVersion != "1.2.3" {
		t.Errorf("MinVersion = %q, want 1.2.3", cfg.MinVersion)
	}
	if !cfg.ShouldShowAppUpgradeMessage {
		t.Error("ShouldShowAppUpgradeMessage should be true")
	}
}

func TestParseEnvLessBridgeConfigJSON_InvalidJSON(t *testing.T) {
	cfg := ParseEnvLessBridgeConfigJSON([]byte(`{not valid json`))
	if cfg != DefaultEnvLessBridgeConfig {
		t.Error("invalid JSON should return default config")
	}
}

func TestParseEnvLessBridgeConfigJSON_OutOfBoundsField(t *testing.T) {
	// One field out of bounds should reject the whole object.
	raw := `{
		"init_retry_max_attempts": 999,
		"init_retry_base_delay_ms": 500,
		"init_retry_jitter_fraction": 0.25,
		"init_retry_max_delay_ms": 4000,
		"http_timeout_ms": 10000,
		"uuid_dedup_buffer_size": 2000,
		"heartbeat_interval_ms": 20000,
		"heartbeat_jitter_fraction": 0.1,
		"token_refresh_buffer_ms": 300000,
		"teardown_archive_timeout_ms": 1500,
		"connect_timeout_ms": 15000,
		"min_version": "0.0.0",
		"should_show_app_upgrade_message": false
	}`

	cfg := ParseEnvLessBridgeConfigJSON([]byte(raw))
	if cfg != DefaultEnvLessBridgeConfig {
		t.Error("out-of-bounds field should reject whole object and return default")
	}
}

func TestParseEnvLessBridgeConfigJSON_NilInput(t *testing.T) {
	cfg := ParseEnvLessBridgeConfigJSON(nil)
	if cfg != DefaultEnvLessBridgeConfig {
		t.Error("nil input should return default config")
	}
}

// ---------------------------------------------------------------------------
// CheckEnvLessBridgeMinVersion
// ---------------------------------------------------------------------------

func TestCheckEnvLessBridgeMinVersion_VersionAboveMin(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.MinVersion = "1.0.0"

	msg := CheckEnvLessBridgeMinVersion(cfg, "2.0.0")
	if msg != "" {
		t.Errorf("expected no error for version above min, got: %s", msg)
	}
}

func TestCheckEnvLessBridgeMinVersion_VersionBelowMin(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.MinVersion = "2.0.0"

	msg := CheckEnvLessBridgeMinVersion(cfg, "1.5.0")
	if msg == "" {
		t.Fatal("expected error for version below min")
	}

	// Check verbatim strings from TS source.
	wantContains := []string{
		"Your version of Claude Code (1.5.0) is too old for Remote Control.",
		"Version 2.0.0 or higher is required.",
		"Run `claude update` to update.",
	}
	for _, s := range wantContains {
		if !containsStr(msg, s) {
			t.Errorf("message missing %q:\n%s", s, msg)
		}
	}
}

func TestCheckEnvLessBridgeMinVersion_EqualVersion(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.MinVersion = "1.5.0"

	msg := CheckEnvLessBridgeMinVersion(cfg, "1.5.0")
	if msg != "" {
		t.Errorf("equal version should pass, got: %s", msg)
	}
}

func TestCheckEnvLessBridgeMinVersion_EmptyMinVersion(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.MinVersion = ""

	msg := CheckEnvLessBridgeMinVersion(cfg, "1.0.0")
	if msg != "" {
		t.Errorf("empty min_version should pass, got: %s", msg)
	}
}

func TestCheckEnvLessBridgeMinVersion_ZeroMinVersion(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.MinVersion = "0.0.0"

	msg := CheckEnvLessBridgeMinVersion(cfg, "0.0.1")
	if msg != "" {
		t.Errorf("0.0.0 min should pass any version, got: %s", msg)
	}
}

// ---------------------------------------------------------------------------
// Validation edge: NaN / Inf floats
// ---------------------------------------------------------------------------

func TestValidateEnvLessBridgeConfig_NaNFloat(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.InitRetryJitterFraction = math.NaN()

	_, ok := ValidateEnvLessBridgeConfig(cfg)
	if ok {
		t.Error("NaN jitter fraction should be rejected")
	}
}

func TestValidateEnvLessBridgeConfig_InfFloat(t *testing.T) {
	cfg := DefaultEnvLessBridgeConfig
	cfg.HeartbeatJitterFraction = math.Inf(1)

	_, ok := ValidateEnvLessBridgeConfig(cfg)
	if ok {
		t.Error("+Inf jitter fraction should be rejected")
	}
}

// ---------------------------------------------------------------------------
// helpers
// ---------------------------------------------------------------------------

func containsStr(haystack, needle string) bool {
	return strings.Contains(haystack, needle)
}
