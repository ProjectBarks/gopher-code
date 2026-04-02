package provider

import (
	"os"
	"strings"
	"testing"
)

// Source: utils/model/aliases.ts, utils/model/model.ts, utils/model/configs.ts

func TestModelAliasesCount(t *testing.T) {
	// Source: aliases.ts:1-9 — 7 aliases total
	if len(ModelAliases) != 7 {
		t.Errorf("expected 7 model aliases, got %d", len(ModelAliases))
	}
}

func TestModelAliasesContents(t *testing.T) {
	// Source: aliases.ts:1-9
	expected := []string{"sonnet", "opus", "haiku", "best", "sonnet[1m]", "opus[1m]", "opusplan"}
	for _, e := range expected {
		if !IsModelAlias(e) {
			t.Errorf("expected %q to be a model alias", e)
		}
	}
}

func TestModelFamilyAliases(t *testing.T) {
	// Source: aliases.ts:21
	if len(ModelFamilyAliases) != 3 {
		t.Fatalf("expected 3 family aliases, got %d", len(ModelFamilyAliases))
	}
	for _, f := range []string{"sonnet", "opus", "haiku"} {
		if !IsModelFamilyAlias(f) {
			t.Errorf("%q should be a family alias", f)
		}
	}
	if IsModelFamilyAlias("best") {
		t.Error("best should NOT be a family alias")
	}
}

func TestIsModelAlias(t *testing.T) {
	// Source: aliases.ts:12-14
	if IsModelAlias("claude-opus-4-6") {
		t.Error("full model ID should not be an alias")
	}
	if !IsModelAlias("opus") {
		t.Error("opus should be an alias")
	}
}

func TestModelConfigsComplete(t *testing.T) {
	// Source: configs.ts:87-99 — 11 model configs
	if len(AllModelConfigs) != 11 {
		t.Errorf("expected 11 model configs, got %d", len(AllModelConfigs))
	}
	expectedKeys := []string{"haiku35", "haiku45", "sonnet35", "sonnet37", "sonnet40", "sonnet45", "sonnet46", "opus40", "opus41", "opus45", "opus46"}
	for _, k := range expectedKeys {
		if _, ok := AllModelConfigs[k]; !ok {
			t.Errorf("missing model config key %q", k)
		}
	}
}

func TestModelConfigFirstPartyIDs(t *testing.T) {
	// Source: configs.ts — verify firstParty IDs match exactly
	tests := []struct {
		key    string
		firstP string
	}{
		{"opus46", "claude-opus-4-6"},                   // Source: configs.ts:73
		{"opus45", "claude-opus-4-5-20251101"},          // Source: configs.ts:66
		{"opus41", "claude-opus-4-1-20250805"},          // Source: configs.ts:59
		{"opus40", "claude-opus-4-20250514"},            // Source: configs.ts:52
		{"sonnet46", "claude-sonnet-4-6"},               // Source: configs.ts:80
		{"sonnet45", "claude-sonnet-4-5-20250929"},      // Source: configs.ts:45
		{"sonnet40", "claude-sonnet-4-20250514"},        // Source: configs.ts:38
		{"sonnet37", "claude-3-7-sonnet-20250219"},      // Source: configs.ts:10
		{"sonnet35", "claude-3-5-sonnet-20241022"},      // Source: configs.ts:17
		{"haiku45", "claude-haiku-4-5-20251001"},        // Source: configs.ts:31
		{"haiku35", "claude-3-5-haiku-20241022"},        // Source: configs.ts:24
	}
	for _, tc := range tests {
		t.Run(tc.key, func(t *testing.T) {
			cfg := AllModelConfigs[tc.key]
			if cfg.FirstParty != tc.firstP {
				t.Errorf("FirstParty = %q, want %q", cfg.FirstParty, tc.firstP)
			}
		})
	}
}

func TestModelConfigProviderVariants(t *testing.T) {
	// Source: configs.ts:72-77 — Opus 4.6 has all provider variants
	cfg := ModelConfigOpus46
	if cfg.FirstParty != "claude-opus-4-6" {
		t.Errorf("firstParty = %q", cfg.FirstParty)
	}
	if cfg.Bedrock != "us.anthropic.claude-opus-4-6-v1" {
		t.Errorf("bedrock = %q", cfg.Bedrock)
	}
	if cfg.Vertex != "claude-opus-4-6" {
		t.Errorf("vertex = %q", cfg.Vertex)
	}
	if cfg.Foundry != "claude-opus-4-6" {
		t.Errorf("foundry = %q", cfg.Foundry)
	}
}

func TestGetAPIProvider(t *testing.T) {
	// Source: providers.ts:6-14
	// Default is firstParty
	t.Run("default_firstParty", func(t *testing.T) {
		os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
		os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
		if p := GetAPIProvider(); p != ProviderFirstParty {
			t.Errorf("expected firstParty, got %s", p)
		}
	})

	t.Run("bedrock", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		if p := GetAPIProvider(); p != ProviderBedrock {
			t.Errorf("expected bedrock, got %s", p)
		}
	})

	t.Run("vertex", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_USE_VERTEX", "true")
		defer os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
		if p := GetAPIProvider(); p != ProviderVertex {
			t.Errorf("expected vertex, got %s", p)
		}
	})

	t.Run("foundry", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
		if p := GetAPIProvider(); p != ProviderFoundry {
			t.Errorf("expected foundry, got %s", p)
		}
	})

	t.Run("bedrock_takes_priority", func(t *testing.T) {
		// Source: providers.ts — bedrock checked first
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		os.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		defer os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
		if p := GetAPIProvider(); p != ProviderBedrock {
			t.Errorf("expected bedrock priority, got %s", p)
		}
	})
}

func TestParseUserSpecifiedModel(t *testing.T) {
	// Source: model.ts:445-506
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("sonnet_alias", func(t *testing.T) {
		result := ParseUserSpecifiedModel("sonnet")
		expected := GetDefaultSonnetModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("opus_alias", func(t *testing.T) {
		result := ParseUserSpecifiedModel("opus")
		expected := GetDefaultOpusModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("haiku_alias", func(t *testing.T) {
		result := ParseUserSpecifiedModel("haiku")
		expected := GetDefaultHaikuModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("best_alias", func(t *testing.T) {
		// Source: model.ts:100-102 — best maps to opus
		result := ParseUserSpecifiedModel("best")
		expected := GetBestModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("opusplan_resolves_to_sonnet", func(t *testing.T) {
		// Source: model.ts:459 — opusplan defaults to Sonnet (Opus in plan mode)
		result := ParseUserSpecifiedModel("opusplan")
		expected := GetDefaultSonnetModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("1m_suffix_preserved", func(t *testing.T) {
		// Source: model.ts:451-453
		result := ParseUserSpecifiedModel("opus[1m]")
		if !strings.HasSuffix(result, "[1m]") {
			t.Errorf("expected [1m] suffix, got %q", result)
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		result := ParseUserSpecifiedModel("SONNET")
		expected := GetDefaultSonnetModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("full_model_id_passthrough", func(t *testing.T) {
		result := ParseUserSpecifiedModel("claude-opus-4-6")
		if result != "claude-opus-4-6" {
			t.Errorf("got %q, want claude-opus-4-6", result)
		}
	})

	t.Run("whitespace_trimmed", func(t *testing.T) {
		result := ParseUserSpecifiedModel("  sonnet  ")
		expected := GetDefaultSonnetModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("custom_model_preserves_case", func(t *testing.T) {
		// Source: model.ts:501-502 — Preserve original case for custom names
		result := ParseUserSpecifiedModel("MyCustomModel-v2")
		if result != "MyCustomModel-v2" {
			t.Errorf("got %q, want MyCustomModel-v2", result)
		}
	})
}

func TestFirstPartyNameToCanonical(t *testing.T) {
	// Source: model.ts:217-270
	tests := []struct {
		input    string
		expected string
	}{
		{"claude-opus-4-6", "claude-opus-4-6"},
		{"claude-opus-4-5-20251101", "claude-opus-4-5"},
		{"claude-opus-4-1-20250805", "claude-opus-4-1"},
		{"claude-opus-4-20250514", "claude-opus-4"},
		{"claude-sonnet-4-6", "claude-sonnet-4-6"},
		{"claude-sonnet-4-5-20250929", "claude-sonnet-4-5"},
		{"claude-sonnet-4-20250514", "claude-sonnet-4"},
		{"claude-haiku-4-5-20251001", "claude-haiku-4-5"},
		{"claude-3-7-sonnet-20250219", "claude-3-7-sonnet"},
		{"claude-3-5-sonnet-20241022", "claude-3-5-sonnet"},
		{"claude-3-5-haiku-20241022", "claude-3-5-haiku"},
		// Bedrock format
		{"us.anthropic.claude-opus-4-6-v1", "claude-opus-4-6"},
		{"us.anthropic.claude-sonnet-4-5-20250929-v1:0", "claude-sonnet-4-5"},
		// Vertex format
		{"claude-3-7-sonnet@20250219", "claude-3-7-sonnet"},
		// Case insensitive
		{"Claude-Opus-4-6", "claude-opus-4-6"},
	}
	for _, tc := range tests {
		t.Run(tc.input, func(t *testing.T) {
			result := FirstPartyNameToCanonical(tc.input)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}

	// Source: model.ts:221 — order matters: 4-6 before 4-5 before 4
	t.Run("specificity_order", func(t *testing.T) {
		r46 := FirstPartyNameToCanonical("claude-opus-4-6")
		r45 := FirstPartyNameToCanonical("claude-opus-4-5")
		r4 := FirstPartyNameToCanonical("claude-opus-4-20250514")
		if r46 == r45 || r46 == r4 || r45 == r4 {
			t.Errorf("should all be distinct: %q, %q, %q", r46, r45, r4)
		}
	})
}

func TestHas1mContext(t *testing.T) {
	// Source: context.ts:71
	if !Has1mContext("claude-opus-4-6[1m]") {
		t.Error("should detect [1m]")
	}
	if !Has1mContext("claude-opus-4-6[1M]") {
		t.Error("should be case-insensitive")
	}
	if Has1mContext("claude-opus-4-6") {
		t.Error("should not detect without suffix")
	}
}

func TestNormalizeModelStringForAPI(t *testing.T) {
	// Source: model.ts:616-618
	if r := NormalizeModelStringForAPI("claude-opus-4-6[1m]"); r != "claude-opus-4-6" {
		t.Errorf("got %q, want claude-opus-4-6", r)
	}
	if r := NormalizeModelStringForAPI("claude-opus-4-6[2m]"); r != "claude-opus-4-6" {
		t.Errorf("got %q for [2m]", r)
	}
	if r := NormalizeModelStringForAPI("claude-opus-4-6"); r != "claude-opus-4-6" {
		t.Errorf("no-op case: %q", r)
	}
}

func TestGetDefaultModels(t *testing.T) {
	// Source: model.ts:105-138
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
	os.Unsetenv("ANTHROPIC_DEFAULT_SONNET_MODEL")
	os.Unsetenv("ANTHROPIC_DEFAULT_OPUS_MODEL")
	os.Unsetenv("ANTHROPIC_DEFAULT_HAIKU_MODEL")

	t.Run("default_opus_is_4_6", func(t *testing.T) {
		// Source: model.ts:115 — opus46 for firstParty
		result := GetDefaultOpusModel()
		if !strings.Contains(result, "opus-4-6") {
			t.Errorf("expected opus-4-6, got %q", result)
		}
	})

	t.Run("default_sonnet_firstParty_is_4_6", func(t *testing.T) {
		// Source: model.ts:127 — sonnet46 for firstParty
		result := GetDefaultSonnetModel()
		if !strings.Contains(result, "sonnet-4-6") {
			t.Errorf("expected sonnet-4-6, got %q", result)
		}
	})

	t.Run("default_haiku_is_4_5", func(t *testing.T) {
		// Source: model.ts:137
		result := GetDefaultHaikuModel()
		if !strings.Contains(result, "haiku-4-5") {
			t.Errorf("expected haiku-4-5, got %q", result)
		}
	})

	t.Run("env_override_opus", func(t *testing.T) {
		os.Setenv("ANTHROPIC_DEFAULT_OPUS_MODEL", "my-custom-opus")
		defer os.Unsetenv("ANTHROPIC_DEFAULT_OPUS_MODEL")
		if r := GetDefaultOpusModel(); r != "my-custom-opus" {
			t.Errorf("expected my-custom-opus, got %q", r)
		}
	})

	t.Run("env_override_sonnet", func(t *testing.T) {
		os.Setenv("ANTHROPIC_DEFAULT_SONNET_MODEL", "my-custom-sonnet")
		defer os.Unsetenv("ANTHROPIC_DEFAULT_SONNET_MODEL")
		if r := GetDefaultSonnetModel(); r != "my-custom-sonnet" {
			t.Errorf("expected my-custom-sonnet, got %q", r)
		}
	})

	t.Run("3p_sonnet_defaults_to_4_5", func(t *testing.T) {
		// Source: model.ts:124 — 3P gets sonnet45
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		result := GetDefaultSonnetModel()
		if !strings.Contains(result, "sonnet-4-5") {
			t.Errorf("expected sonnet-4-5 for 3P, got %q", result)
		}
	})
}

func TestGetSmallFastModel(t *testing.T) {
	// Source: model.ts:36-38
	os.Unsetenv("ANTHROPIC_SMALL_FAST_MODEL")
	result := GetSmallFastModel()
	if !strings.Contains(result, "haiku") {
		t.Errorf("expected haiku, got %q", result)
	}

	os.Setenv("ANTHROPIC_SMALL_FAST_MODEL", "custom-fast")
	defer os.Unsetenv("ANTHROPIC_SMALL_FAST_MODEL")
	if r := GetSmallFastModel(); r != "custom-fast" {
		t.Errorf("expected custom-fast, got %q", r)
	}
}

func TestGetPublicModelDisplayName(t *testing.T) {
	// Source: model.ts:349-384
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	tests := []struct {
		model    string
		expected string
	}{
		{"claude-opus-4-6", "Opus 4.6"},
		{"claude-opus-4-6[1m]", "Opus 4.6 (1M context)"},
		{"claude-sonnet-4-6", "Sonnet 4.6"},
		{"claude-haiku-4-5-20251001", "Haiku 4.5"},
		{"unknown-model", ""},
	}
	for _, tc := range tests {
		t.Run(tc.model, func(t *testing.T) {
			result := GetPublicModelDisplayName(tc.model)
			if result != tc.expected {
				t.Errorf("got %q, want %q", result, tc.expected)
			}
		})
	}
}

func TestGetMarketingNameForModel(t *testing.T) {
	// Source: model.ts:570-614
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")

	t.Run("opus_4_6", func(t *testing.T) {
		if r := GetMarketingNameForModel("claude-opus-4-6"); r != "Opus 4.6" {
			t.Errorf("got %q", r)
		}
	})
	t.Run("opus_4_6_1m", func(t *testing.T) {
		if r := GetMarketingNameForModel("claude-opus-4-6[1m]"); r != "Opus 4.6 (with 1M context)" {
			t.Errorf("got %q", r)
		}
	})
	t.Run("sonnet_3_7", func(t *testing.T) {
		if r := GetMarketingNameForModel("claude-3-7-sonnet-20250219"); r != "Claude 3.7 Sonnet" {
			t.Errorf("got %q", r)
		}
	})
	t.Run("foundry_returns_empty", func(t *testing.T) {
		// Source: model.ts:571-574 — Foundry deployment IDs are user-defined
		os.Setenv("CLAUDE_CODE_USE_FOUNDRY", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
		if r := GetMarketingNameForModel("claude-opus-4-6"); r != "" {
			t.Errorf("foundry should return empty, got %q", r)
		}
	})
}

func TestGetPublicModelName(t *testing.T) {
	// Source: model.ts:425-431
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("known_model", func(t *testing.T) {
		if r := GetPublicModelName("claude-opus-4-6"); r != "Claude Opus 4.6" {
			t.Errorf("got %q", r)
		}
	})
	t.Run("unknown_model", func(t *testing.T) {
		if r := GetPublicModelName("custom-model"); r != "Claude (custom-model)" {
			t.Errorf("got %q", r)
		}
	})
}

func TestLegacyOpusRemap(t *testing.T) {
	// Source: model.ts:477-483
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
	os.Unsetenv("CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP")

	t.Run("opus_4_0_remapped", func(t *testing.T) {
		result := ParseUserSpecifiedModel("claude-opus-4-20250514")
		expected := GetDefaultOpusModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("opus_4_1_remapped", func(t *testing.T) {
		result := ParseUserSpecifiedModel("claude-opus-4-1-20250805")
		expected := GetDefaultOpusModel()
		if result != expected {
			t.Errorf("got %q, want %q", result, expected)
		}
	})

	t.Run("remap_disabled_by_env", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP", "1")
		defer os.Unsetenv("CLAUDE_CODE_DISABLE_LEGACY_MODEL_REMAP")
		result := ParseUserSpecifiedModel("claude-opus-4-20250514")
		if result != "claude-opus-4-20250514" {
			t.Errorf("expected passthrough when remap disabled, got %q", result)
		}
	})
}

func TestResolveModelInProvider(t *testing.T) {
	// Test that resolveModel correctly combines ParseUserSpecifiedModel + NormalizeModelStringForAPI
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("alias_resolved", func(t *testing.T) {
		result := resolveModel("sonnet")
		if strings.Contains(result, "[") {
			t.Errorf("should not have bracket suffix: %q", result)
		}
		if !strings.Contains(result, "sonnet") {
			t.Errorf("should contain sonnet: %q", result)
		}
	})

	t.Run("1m_stripped_for_api", func(t *testing.T) {
		result := resolveModel("opus[1m]")
		if strings.Contains(result, "[1m]") {
			t.Errorf("[1m] should be stripped for API: %q", result)
		}
	})
}
