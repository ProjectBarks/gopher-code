package provider

import (
	"testing"
)

func TestModelSupportsStructuredOutputs(t *testing.T) {
	if !ModelSupportsStructuredOutputs("claude-sonnet-4-20250514") {
		t.Error("sonnet-4 should support structured outputs")
	}
	if !ModelSupportsStructuredOutputs("claude-opus-4-6") {
		t.Error("opus-4-6 should support structured outputs")
	}
	if ModelSupportsStructuredOutputs("gpt-4") {
		t.Error("gpt-4 should not match")
	}
}

func TestModelSupportsAutoMode(t *testing.T) {
	if !ModelSupportsAutoMode("claude-opus-4-6") {
		t.Error("opus-4-6 should support auto mode")
	}
	if !ModelSupportsAutoMode("claude-sonnet-4-6-20250514") {
		t.Error("sonnet-4-6 should support auto mode")
	}
	if ModelSupportsAutoMode("claude-haiku-4-5") {
		t.Error("haiku should not support auto mode")
	}
}

func TestShouldUseGlobalCacheScope(t *testing.T) {
	t.Setenv("CLAUDE_CODE_GLOBAL_CACHE_SCOPE", "")
	if ShouldUseGlobalCacheScope() {
		t.Error("should be false when env not set")
	}
	t.Setenv("CLAUDE_CODE_GLOBAL_CACHE_SCOPE", "1")
	if !ShouldUseGlobalCacheScope() {
		t.Error("should be true when env is set")
	}
}

func TestGetEffortSuffix(t *testing.T) {
	suffix := GetEffortSuffix("claude-opus-4-6", EffortHigh)
	if suffix != " with high effort" {
		t.Errorf("suffix = %q, want ' with high effort'", suffix)
	}

	empty := GetEffortSuffix("claude-opus-4-6", "")
	if empty != "" {
		t.Errorf("empty effort should give empty suffix, got %q", empty)
	}
}

func TestToPersistableEffort(t *testing.T) {
	if ToPersistableEffort(EffortLow) != EffortLow {
		t.Error("low should persist")
	}
	if ToPersistableEffort(EffortMedium) != EffortMedium {
		t.Error("medium should persist")
	}
	if ToPersistableEffort(EffortHigh) != EffortHigh {
		t.Error("high should persist")
	}
	// Max should not persist for non-ant
	t.Setenv("USER_TYPE", "external")
	if ToPersistableEffort(EffortMax) != "" {
		t.Error("max should not persist for non-ant")
	}
	t.Setenv("USER_TYPE", "ant")
	if ToPersistableEffort(EffortMax) != EffortMax {
		t.Error("max should persist for ant")
	}
}

func TestGetDefaultEffortForModel(t *testing.T) {
	if d := GetDefaultEffortForModel("claude-opus-4-6"); d != EffortMedium {
		t.Errorf("opus-4-6 default = %q, want medium", d)
	}
	if d := GetDefaultEffortForModel("claude-sonnet-4-6"); d != "" {
		t.Errorf("sonnet-4-6 default = %q, want empty", d)
	}
}

func TestGetEffortEnvOverride(t *testing.T) {
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "")
	if GetEffortEnvOverride() != "" {
		t.Error("should be empty when not set")
	}
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "low")
	if GetEffortEnvOverride() != EffortLow {
		t.Error("should return low")
	}
	t.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "unset")
	if GetEffortEnvOverride() != "" {
		t.Error("unset should return empty")
	}
}
