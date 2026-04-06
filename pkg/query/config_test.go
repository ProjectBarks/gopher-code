package query

import (
	"os"
	"testing"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestBuildQueryConfig_SnapshotsSessionID verifies that BuildQueryConfig
// captures the session ID into the immutable config.
func TestBuildQueryConfig_SnapshotsSessionID(t *testing.T) {
	cfg := BuildQueryConfig("sess-abc-123")
	assert.Equal(t, "sess-abc-123", cfg.SessionID)
}

// TestBuildQueryConfig_DefaultGates verifies gate defaults when no env vars
// are set.
func TestBuildQueryConfig_DefaultGates(t *testing.T) {
	// Clear all relevant env vars to get clean defaults.
	for _, key := range []string{
		"CLAUDE_CODE_STREAMING_TOOL_EXECUTION",
		"CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES",
		"USER_TYPE",
		"CLAUDE_CODE_DISABLE_FAST_MODE",
	} {
		t.Setenv(key, "")
		os.Unsetenv(key)
	}

	cfg := BuildQueryConfig("test-session")

	// T44: streaming tool execution defaults to true (broadly rolled out).
	assert.True(t, cfg.Gates.StreamingToolExecution,
		"StreamingToolExecution should default to true")

	// T45: emitToolUseSummaries defaults to false (opt-in env var).
	assert.False(t, cfg.Gates.EmitToolUseSummaries,
		"EmitToolUseSummaries should default to false")

	// isAnt defaults to false (USER_TYPE not set).
	assert.False(t, cfg.Gates.IsAnt,
		"IsAnt should default to false")

	// fastModeEnabled defaults to true (CLAUDE_CODE_DISABLE_FAST_MODE not set).
	assert.True(t, cfg.Gates.FastModeEnabled,
		"FastModeEnabled should default to true")
}

// TestBuildQueryConfig_StreamingToolExecution_EnvOverride tests the
// CLAUDE_CODE_STREAMING_TOOL_EXECUTION env var override.
func TestBuildQueryConfig_StreamingToolExecution_EnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAMING_TOOL_EXECUTION", "false")

	cfg := BuildQueryConfig("test")
	assert.False(t, cfg.Gates.StreamingToolExecution,
		"StreamingToolExecution should be false when env is 'false'")
}

// TestBuildQueryConfig_StreamingToolExecution_Enabled tests explicit enable.
func TestBuildQueryConfig_StreamingToolExecution_Enabled(t *testing.T) {
	t.Setenv("CLAUDE_CODE_STREAMING_TOOL_EXECUTION", "1")

	cfg := BuildQueryConfig("test")
	assert.True(t, cfg.Gates.StreamingToolExecution,
		"StreamingToolExecution should be true when env is '1'")
}

// TestBuildQueryConfig_EmitToolUseSummaries tests the
// CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES env flag (T45).
func TestBuildQueryConfig_EmitToolUseSummaries(t *testing.T) {
	tests := []struct {
		envVal   string
		expected bool
	}{
		{"1", true},
		{"true", true},
		{"yes", true},
		{"TRUE", true},
		{"0", false},
		{"false", false},
		{"", false},
		{"no", false},
	}

	for _, tt := range tests {
		t.Run("val="+tt.envVal, func(t *testing.T) {
			if tt.envVal == "" {
				os.Unsetenv("CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES")
			} else {
				t.Setenv("CLAUDE_CODE_EMIT_TOOL_USE_SUMMARIES", tt.envVal)
			}

			cfg := BuildQueryConfig("test")
			assert.Equal(t, tt.expected, cfg.Gates.EmitToolUseSummaries)
		})
	}
}

// TestBuildQueryConfig_IsAnt tests the USER_TYPE == "ant" gate.
func TestBuildQueryConfig_IsAnt(t *testing.T) {
	t.Run("ant", func(t *testing.T) {
		t.Setenv("USER_TYPE", "ant")
		cfg := BuildQueryConfig("test")
		assert.True(t, cfg.Gates.IsAnt)
	})
	t.Run("external", func(t *testing.T) {
		t.Setenv("USER_TYPE", "external")
		cfg := BuildQueryConfig("test")
		assert.False(t, cfg.Gates.IsAnt)
	})
	t.Run("empty", func(t *testing.T) {
		os.Unsetenv("USER_TYPE")
		cfg := BuildQueryConfig("test")
		assert.False(t, cfg.Gates.IsAnt)
	})
}

// TestBuildQueryConfig_FastModeEnabled tests the CLAUDE_CODE_DISABLE_FAST_MODE gate.
func TestBuildQueryConfig_FastModeEnabled(t *testing.T) {
	t.Run("disabled_by_env", func(t *testing.T) {
		t.Setenv("CLAUDE_CODE_DISABLE_FAST_MODE", "1")
		cfg := BuildQueryConfig("test")
		assert.False(t, cfg.Gates.FastModeEnabled,
			"FastModeEnabled should be false when DISABLE is truthy")
	})
	t.Run("enabled_by_default", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_DISABLE_FAST_MODE")
		cfg := BuildQueryConfig("test")
		assert.True(t, cfg.Gates.FastModeEnabled,
			"FastModeEnabled should be true when DISABLE is not set")
	})
}

// TestBuildQueryConfig_Immutability verifies the config is a value type
// snapshot -- modifying a copy does not affect the original.
func TestBuildQueryConfig_Immutability(t *testing.T) {
	cfg1 := BuildQueryConfig("session-1")
	cfg2 := cfg1 // copy

	cfg2.SessionID = "session-2"
	cfg2.Gates.StreamingToolExecution = !cfg1.Gates.StreamingToolExecution

	require.Equal(t, "session-1", cfg1.SessionID, "original should be unmodified")
	require.NotEqual(t, cfg1.Gates.StreamingToolExecution, cfg2.Gates.StreamingToolExecution)
}

// TestIsEnvTruthy exercises the isEnvTruthy helper directly.
func TestIsEnvTruthy(t *testing.T) {
	truthy := []string{"1", "true", "TRUE", "True", "yes", "YES", "Yes", " true ", " 1 "}
	for _, v := range truthy {
		assert.True(t, isEnvTruthy(v), "expected truthy for %q", v)
	}
	falsy := []string{"0", "false", "", "no", "nope", "2", "enabled"}
	for _, v := range falsy {
		assert.False(t, isEnvTruthy(v), "expected falsy for %q", v)
	}
}
