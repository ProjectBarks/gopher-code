package provider

import (
	"os"
	"testing"
)

// Source: utils/effort.ts, utils/thinking.ts

func TestEffortLevels(t *testing.T) {
	// Source: effort.ts:13-18
	if len(EffortLevels) != 4 {
		t.Fatalf("expected 4 effort levels, got %d", len(EffortLevels))
	}
	expected := []EffortLevel{EffortLow, EffortMedium, EffortHigh, EffortMax}
	for i, e := range expected {
		if EffortLevels[i] != e {
			t.Errorf("level[%d] = %q, want %q", i, EffortLevels[i], e)
		}
	}
}

func TestIsEffortLevel(t *testing.T) {
	// Source: effort.ts:68-70
	for _, level := range []string{"low", "medium", "high", "max"} {
		if !IsEffortLevel(level) {
			t.Errorf("%q should be a valid effort level", level)
		}
	}
	if IsEffortLevel("ultra") {
		t.Error("ultra should not be a valid effort level")
	}
	if IsEffortLevel("") {
		t.Error("empty should not be a valid effort level")
	}
}

func TestParseEffortValue(t *testing.T) {
	// Source: effort.ts:71-87
	t.Run("named_levels", func(t *testing.T) {
		for _, level := range []string{"low", "medium", "high", "max"} {
			result, ok := ParseEffortValue(level)
			if !ok {
				t.Errorf("should parse %q", level)
			}
			if string(result) != level {
				t.Errorf("got %q, want %q", result, level)
			}
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		result, ok := ParseEffortValue("HIGH")
		if !ok {
			t.Fatal("should parse HIGH")
		}
		if result != EffortHigh {
			t.Errorf("got %q, want high", result)
		}
	})

	t.Run("empty_returns_false", func(t *testing.T) {
		_, ok := ParseEffortValue("")
		if ok {
			t.Error("empty should return false")
		}
	})

	t.Run("numeric_30_is_low", func(t *testing.T) {
		result, ok := ParseEffortValue("30")
		if !ok {
			t.Fatal("should parse 30")
		}
		if result != EffortLow {
			t.Errorf("30 should be low, got %q", result)
		}
	})

	t.Run("numeric_75_is_medium", func(t *testing.T) {
		result, ok := ParseEffortValue("75")
		if !ok {
			t.Fatal("should parse 75")
		}
		if result != EffortMedium {
			t.Errorf("75 should be medium, got %q", result)
		}
	})

	t.Run("numeric_100_is_high", func(t *testing.T) {
		result, ok := ParseEffortValue("100")
		if !ok {
			t.Fatal("should parse 100")
		}
		if result != EffortHigh {
			t.Errorf("100 should be high, got %q", result)
		}
	})

	t.Run("numeric_200_is_max", func(t *testing.T) {
		result, ok := ParseEffortValue("200")
		if !ok {
			t.Fatal("should parse 200")
		}
		if result != EffortMax {
			t.Errorf("200 should be max, got %q", result)
		}
	})
}

func TestModelSupportsEffort(t *testing.T) {
	// Source: effort.ts:23-49
	os.Unsetenv("CLAUDE_CODE_ALWAYS_ENABLE_EFFORT")
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("opus_4_6_supports", func(t *testing.T) {
		if !ModelSupportsEffort("claude-opus-4-6") {
			t.Error("opus 4.6 should support effort")
		}
	})

	t.Run("sonnet_4_6_supports", func(t *testing.T) {
		if !ModelSupportsEffort("claude-sonnet-4-6") {
			t.Error("sonnet 4.6 should support effort")
		}
	})

	t.Run("haiku_no_effort", func(t *testing.T) {
		// Source: effort.ts:38-39 — haiku excluded
		if ModelSupportsEffort("claude-haiku-4-5-20251001") {
			t.Error("haiku should NOT support effort")
		}
	})

	t.Run("sonnet_4_5_no_effort", func(t *testing.T) {
		// Source: effort.ts:38-39 — older sonnet excluded
		if ModelSupportsEffort("claude-sonnet-4-5-20250929") {
			t.Error("sonnet 4.5 should NOT support effort")
		}
	})

	t.Run("env_force_enable", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_ALWAYS_ENABLE_EFFORT", "1")
		defer os.Unsetenv("CLAUDE_CODE_ALWAYS_ENABLE_EFFORT")
		if !ModelSupportsEffort("claude-haiku-4-5-20251001") {
			t.Error("env override should enable effort for haiku")
		}
	})

	t.Run("unknown_model_1p_defaults_true", func(t *testing.T) {
		// Source: effort.ts:48 — unknown on 1P defaults to true
		if !ModelSupportsEffort("totally-new-model") {
			t.Error("unknown 1P model should default to effort support")
		}
	})

	t.Run("unknown_model_3p_defaults_false", func(t *testing.T) {
		// Source: effort.ts:48 — only 1P gets default true
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		if ModelSupportsEffort("totally-new-model") {
			t.Error("unknown 3P model should default to no effort support")
		}
	})
}

func TestModelSupportsMaxEffort(t *testing.T) {
	// Source: effort.ts:53-65
	t.Run("opus_4_6_supports_max", func(t *testing.T) {
		if !ModelSupportsMaxEffort("claude-opus-4-6") {
			t.Error("opus 4.6 should support max effort")
		}
	})

	t.Run("sonnet_4_6_no_max", func(t *testing.T) {
		// Source: effort.ts:53 — max is Opus 4.6 only for public models
		if ModelSupportsMaxEffort("claude-sonnet-4-6") {
			t.Error("sonnet 4.6 should NOT support max effort")
		}
	})
}

func TestResolveAppliedEffort(t *testing.T) {
	// Source: effort.ts:152-167
	os.Unsetenv("CLAUDE_CODE_EFFORT_LEVEL")

	t.Run("appstate_passed_through", func(t *testing.T) {
		result := ResolveAppliedEffort("claude-opus-4-6", EffortMedium)
		if result != EffortMedium {
			t.Errorf("got %q, want medium", result)
		}
	})

	t.Run("env_overrides_appstate", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "low")
		defer os.Unsetenv("CLAUDE_CODE_EFFORT_LEVEL")
		result := ResolveAppliedEffort("claude-opus-4-6", EffortHigh)
		if result != EffortLow {
			t.Errorf("got %q, want low (env override)", result)
		}
	})

	t.Run("env_unset_returns_empty", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_EFFORT_LEVEL", "unset")
		defer os.Unsetenv("CLAUDE_CODE_EFFORT_LEVEL")
		result := ResolveAppliedEffort("claude-opus-4-6", EffortMedium)
		if result != "" {
			t.Errorf("got %q, want empty (unset)", result)
		}
	})

	t.Run("max_downgraded_on_non_opus", func(t *testing.T) {
		// Source: effort.ts:165-166 — max → high on non-Opus-4.6
		result := ResolveAppliedEffort("claude-sonnet-4-6", EffortMax)
		if result != EffortHigh {
			t.Errorf("got %q, want high (max downgraded for sonnet)", result)
		}
	})

	t.Run("max_preserved_on_opus_4_6", func(t *testing.T) {
		result := ResolveAppliedEffort("claude-opus-4-6", EffortMax)
		if result != EffortMax {
			t.Errorf("got %q, want max", result)
		}
	})

	t.Run("no_appstate_returns_empty", func(t *testing.T) {
		result := ResolveAppliedEffort("claude-opus-4-6", "")
		if result != "" {
			t.Errorf("got %q, want empty (no appstate, no default)", result)
		}
	})
}

func TestGetDisplayedEffortLevel(t *testing.T) {
	// Source: effort.ts:174-180
	os.Unsetenv("CLAUDE_CODE_EFFORT_LEVEL")

	t.Run("defaults_to_high", func(t *testing.T) {
		// Source: effort.ts:179 — fallback is 'high'
		result := GetDisplayedEffortLevel("claude-opus-4-6", "")
		if result != EffortHigh {
			t.Errorf("got %q, want high", result)
		}
	})

	t.Run("with_appstate", func(t *testing.T) {
		result := GetDisplayedEffortLevel("claude-opus-4-6", EffortLow)
		if result != EffortLow {
			t.Errorf("got %q, want low", result)
		}
	})
}

func TestGetEffortLevelDescription(t *testing.T) {
	// Source: effort.ts:224-235
	for _, level := range EffortLevels {
		desc := GetEffortLevelDescription(level)
		if desc == "" {
			t.Errorf("description for %q should not be empty", level)
		}
	}
	// Verify specific descriptions
	if d := GetEffortLevelDescription(EffortMax); d != "Maximum capability with deepest reasoning (Opus 4.6 only)" {
		t.Errorf("max description = %q", d)
	}
}

func TestModelSupportsThinking(t *testing.T) {
	// Source: thinking.ts:90-110
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("opus_4_6_supports", func(t *testing.T) {
		if !ModelSupportsThinking("claude-opus-4-6") {
			t.Error("opus 4.6 should support thinking")
		}
	})

	t.Run("haiku_4_5_supports_on_1p", func(t *testing.T) {
		// Source: thinking.ts:105 — 1P: all non-claude-3
		if !ModelSupportsThinking("claude-haiku-4-5-20251001") {
			t.Error("haiku 4.5 should support thinking on 1P")
		}
	})

	t.Run("claude_3_no_thinking_on_1p", func(t *testing.T) {
		// Source: thinking.ts:106
		if ModelSupportsThinking("claude-3-5-sonnet-20241022") {
			t.Error("claude 3.x should NOT support thinking on 1P")
		}
	})

	t.Run("3p_only_opus4_sonnet4", func(t *testing.T) {
		// Source: thinking.ts:109
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")

		if !ModelSupportsThinking("claude-opus-4-6") {
			t.Error("opus 4 should support thinking on 3P")
		}
		if ModelSupportsThinking("claude-haiku-4-5-20251001") {
			t.Error("haiku should NOT support thinking on 3P (Bedrock)")
		}
	})
}

func TestModelSupportsAdaptiveThinking(t *testing.T) {
	// Source: thinking.ts:113-144
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("opus_4_6_supports", func(t *testing.T) {
		if !ModelSupportsAdaptiveThinking("claude-opus-4-6") {
			t.Error("opus 4.6 should support adaptive thinking")
		}
	})

	t.Run("sonnet_4_6_supports", func(t *testing.T) {
		if !ModelSupportsAdaptiveThinking("claude-sonnet-4-6") {
			t.Error("sonnet 4.6 should support adaptive thinking")
		}
	})

	t.Run("opus_4_5_no_adaptive", func(t *testing.T) {
		// Source: thinking.ts:127-130 — older models excluded
		if ModelSupportsAdaptiveThinking("claude-opus-4-5-20251101") {
			t.Error("opus 4.5 should NOT support adaptive thinking")
		}
	})

	t.Run("haiku_no_adaptive", func(t *testing.T) {
		if ModelSupportsAdaptiveThinking("claude-haiku-4-5-20251001") {
			t.Error("haiku should NOT support adaptive thinking")
		}
	})
}
