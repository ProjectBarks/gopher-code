package provider_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"
)

func modelSystemPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "model_system.json")
}

type ModelSystem struct {
	ModelAliases struct {
		AllAliases          []string `json:"all_aliases"`
		FamilyAliases       []string `json:"family_aliases"`
		OneMSuffix          string   `json:"1m_suffix_pattern"`
		FamilyIsWildcard    bool     `json:"family_alias_is_wildcard"`
	} `json:"model_aliases"`
	CanonicalNames struct {
		Opus46    string `json:"claude_opus_4_6"`
		Opus45    string `json:"claude_opus_4_5"`
		Opus41    string `json:"claude_opus_4_1"`
		Opus4     string `json:"claude_opus_4"`
		Sonnet46  string `json:"claude_sonnet_4_6"`
		Sonnet45  string `json:"claude_sonnet_4_5"`
		Sonnet4   string `json:"claude_sonnet_4"`
		Order     string `json:"resolution_order"`
	} `json:"canonical_model_names"`
	ContextWindows struct {
		DefaultTokens        int      `json:"default_tokens"`
		OneMTokens           int      `json:"1m_tokens"`
		OneMSupportedModels  []string `json:"1m_supported_models"`
		OneMDisabledEnv      string   `json:"1m_disabled_env"`
		OverrideEnv          string   `json:"override_env"`
		OneMSuffixPrecedence bool     `json:"1m_suffix_takes_precedence"`
	} `json:"context_windows"`
	OutputTokens struct {
		Default      int `json:"default"`
		UpperLimit   int `json:"upper_limit"`
		CappedDefault int `json:"capped_default"`
		Escalated    int `json:"escalated"`
		CompactMax   int `json:"compact_max"`
	} `json:"output_tokens"`
	FastMode struct {
		DisableEnv          string   `json:"disable_env"`
		SameModelFaster     bool     `json:"same_model_faster_output"`
		NotDifferentModel   bool     `json:"not_a_different_model"`
		CooldownReasons     []string `json:"cooldown_reasons"`
	} `json:"fast_mode"`
	SubagentModels struct {
		ExploreDefault    string `json:"explore_external_default"`
		PlanDefault       string `json:"plan_default"`
		StatuslineSetup   string `json:"statusline_setup"`
		ClaudeCodeGuide   string `json:"claude_code_guide"`
	} `json:"subagent_models"`
}

func loadModelSystem(t *testing.T) *ModelSystem {
	t.Helper()
	data, err := os.ReadFile(modelSystemPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var ms ModelSystem
	if err := json.Unmarshal(data, &ms); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &ms
}

// TestModelAliases validates model alias system from TS source.
// Source: model/aliases.ts
func TestModelAliases(t *testing.T) {
	ms := loadModelSystem(t)
	ma := ms.ModelAliases

	t.Run("alias_count_7", func(t *testing.T) {
		if len(ma.AllAliases) != 7 {
			t.Errorf("expected 7 aliases, got %d", len(ma.AllAliases))
		}
	})

	expectedAliases := []string{"sonnet", "opus", "haiku", "best", "sonnet[1m]", "opus[1m]", "opusplan"}
	for _, alias := range expectedAliases {
		alias := alias
		t.Run(fmt.Sprintf("alias_%s", alias), func(t *testing.T) {
			found := false
			for _, a := range ma.AllAliases {
				if a == alias {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("alias %q not found", alias)
			}
		})
	}

	t.Run("family_aliases_3", func(t *testing.T) {
		if len(ma.FamilyAliases) != 3 {
			t.Fatalf("expected 3 family aliases, got %d", len(ma.FamilyAliases))
		}
		expected := []string{"sonnet", "opus", "haiku"}
		for i, e := range expected {
			if ma.FamilyAliases[i] != e {
				t.Errorf("family[%d] = %q, want %q", i, ma.FamilyAliases[i], e)
			}
		}
	})

	t.Run("1m_suffix", func(t *testing.T) {
		if ma.OneMSuffix != "[1m]" {
			t.Errorf("expected [1m], got %s", ma.OneMSuffix)
		}
	})

	t.Run("family_is_wildcard", func(t *testing.T) {
		// When "opus" is in allowlist, ANY opus model is allowed
		if !ma.FamilyIsWildcard {
			t.Error("family aliases must act as wildcards in allowlists")
		}
	})
}

// TestCanonicalModelNames validates model name resolution.
// Source: model/model.ts:221-240
func TestCanonicalModelNames(t *testing.T) {
	ms := loadModelSystem(t)
	cn := ms.CanonicalNames

	models := []struct {
		name     string
		expected string
	}{
		{"opus_4_6", cn.Opus46},
		{"opus_4_5", cn.Opus45},
		{"opus_4_1", cn.Opus41},
		{"opus_4", cn.Opus4},
		{"sonnet_4_6", cn.Sonnet46},
		{"sonnet_4_5", cn.Sonnet45},
		{"sonnet_4", cn.Sonnet4},
	}

	for _, m := range models {
		m := m
		t.Run(m.name, func(t *testing.T) {
			if m.expected == "" {
				t.Error("canonical name is empty")
			}
			if !strings.HasPrefix(m.expected, "claude-") {
				t.Errorf("expected claude- prefix, got %s", m.expected)
			}
		})
	}

	// Verify resolution order: more specific first
	t.Run("46_before_45_before_4", func(t *testing.T) {
		// Source: model.ts checks claude-opus-4-6 before claude-opus-4-5 before claude-opus-4
		if cn.Opus46 == cn.Opus4 || cn.Opus45 == cn.Opus4 {
			t.Error("specific versions must be distinct from base")
		}
	})
}

// TestContextWindowConstants validates context window configuration.
// Source: context.ts:9,60-100
func TestContextWindowConstants(t *testing.T) {
	ms := loadModelSystem(t)
	cw := ms.ContextWindows

	t.Run("default_200k", func(t *testing.T) {
		if cw.DefaultTokens != 200000 {
			t.Errorf("expected 200000, got %d", cw.DefaultTokens)
		}
	})
	t.Run("1m_is_1000000", func(t *testing.T) {
		if cw.OneMTokens != 1000000 {
			t.Errorf("expected 1000000, got %d", cw.OneMTokens)
		}
	})
	t.Run("1m_disabled_env_var", func(t *testing.T) {
		if cw.OneMDisabledEnv != "CLAUDE_CODE_DISABLE_1M_CONTEXT" {
			t.Errorf("expected CLAUDE_CODE_DISABLE_1M_CONTEXT, got %s", cw.OneMDisabledEnv)
		}
	})
	t.Run("override_env_var", func(t *testing.T) {
		if cw.OverrideEnv != "CLAUDE_CODE_MAX_CONTEXT_TOKENS" {
			t.Errorf("expected CLAUDE_CODE_MAX_CONTEXT_TOKENS, got %s", cw.OverrideEnv)
		}
	})
	t.Run("1m_suffix_takes_precedence", func(t *testing.T) {
		// Source: context.ts:71 — [1m] suffix checked before model capabilities
		if !cw.OneMSuffixPrecedence {
			t.Error("[1m] suffix must take precedence over all detection")
		}
	})
	t.Run("1m_supported_models_exist", func(t *testing.T) {
		if len(cw.OneMSupportedModels) < 2 {
			t.Errorf("expected at least 2 1M-supported model patterns, got %d", len(cw.OneMSupportedModels))
		}
	})
}

// TestOutputTokenConstants validates output token configuration.
// Source: context.ts:12-25
func TestOutputTokenConstants(t *testing.T) {
	ms := loadModelSystem(t)
	ot := ms.OutputTokens

	t.Run("default_32k", func(t *testing.T) {
		if ot.Default != 32000 {
			t.Errorf("expected 32000, got %d", ot.Default)
		}
	})
	t.Run("upper_limit_64k", func(t *testing.T) {
		if ot.UpperLimit != 64000 {
			t.Errorf("expected 64000, got %d", ot.UpperLimit)
		}
	})
	t.Run("capped_default_8k", func(t *testing.T) {
		if ot.CappedDefault != 8000 {
			t.Errorf("expected 8000, got %d", ot.CappedDefault)
		}
	})
	t.Run("escalated_64k", func(t *testing.T) {
		if ot.Escalated != 64000 {
			t.Errorf("expected 64000, got %d", ot.Escalated)
		}
	})
	t.Run("compact_max_20k", func(t *testing.T) {
		if ot.CompactMax != 20000 {
			t.Errorf("expected 20000, got %d", ot.CompactMax)
		}
	})
	t.Run("escalated_equals_upper_limit", func(t *testing.T) {
		if ot.Escalated != ot.UpperLimit {
			t.Errorf("escalated %d != upper_limit %d", ot.Escalated, ot.UpperLimit)
		}
	})
	t.Run("capped_less_than_default", func(t *testing.T) {
		if ot.CappedDefault >= ot.Default {
			t.Errorf("capped %d >= default %d", ot.CappedDefault, ot.Default)
		}
	})
}

// TestFastMode validates fast mode configuration.
// Source: fastMode.ts:38-46
func TestFastMode(t *testing.T) {
	ms := loadModelSystem(t)
	fm := ms.FastMode

	t.Run("disable_env", func(t *testing.T) {
		if fm.DisableEnv != "CLAUDE_CODE_DISABLE_FAST_MODE" {
			t.Errorf("expected CLAUDE_CODE_DISABLE_FAST_MODE, got %s", fm.DisableEnv)
		}
	})
	t.Run("same_model_not_different", func(t *testing.T) {
		// Fast mode uses the same model with faster output, NOT a different model
		if !fm.SameModelFaster {
			t.Error("fast mode must use same model")
		}
		if !fm.NotDifferentModel {
			t.Error("fast mode must NOT be a different model")
		}
	})
	t.Run("cooldown_reasons", func(t *testing.T) {
		expected := []string{"rate_limit", "overloaded"}
		if len(fm.CooldownReasons) != len(expected) {
			t.Fatalf("expected %d cooldown reasons, got %d", len(expected), len(fm.CooldownReasons))
		}
		for i, e := range expected {
			if fm.CooldownReasons[i] != e {
				t.Errorf("reason[%d] = %q, want %q", i, fm.CooldownReasons[i], e)
			}
		}
	})
}

// TestSubagentModelDefaults validates default models for built-in agents.
// Source: AgentTool built-in agent definitions
func TestSubagentModelDefaults(t *testing.T) {
	ms := loadModelSystem(t)
	sm := ms.SubagentModels

	t.Run("explore_uses_haiku_external", func(t *testing.T) {
		if sm.ExploreDefault != "haiku" {
			t.Errorf("expected haiku, got %s", sm.ExploreDefault)
		}
	})
	t.Run("plan_uses_inherit", func(t *testing.T) {
		if sm.PlanDefault != "inherit" {
			t.Errorf("expected inherit, got %s", sm.PlanDefault)
		}
	})
	t.Run("statusline_uses_sonnet", func(t *testing.T) {
		if sm.StatuslineSetup != "sonnet" {
			t.Errorf("expected sonnet, got %s", sm.StatuslineSetup)
		}
	})
	t.Run("guide_uses_haiku", func(t *testing.T) {
		if sm.ClaudeCodeGuide != "haiku" {
			t.Errorf("expected haiku, got %s", sm.ClaudeCodeGuide)
		}
	})
}
