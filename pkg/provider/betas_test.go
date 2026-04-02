package provider

import (
	"os"
	"testing"
)

// Source: constants/betas.ts, utils/betas.ts

func TestBetaConstants(t *testing.T) {
	// Source: constants/betas.ts — verify all constants match TS exactly
	tests := []struct {
		name  string
		value string
	}{
		{"claude_code", BetaClaudeCode},
		{"interleaved_thinking", BetaInterleavedThinking},
		{"context_1m", BetaContext1M},
		{"context_management", BetaContextManagement},
		{"structured_outputs", BetaStructuredOutputs},
		{"web_search", BetaWebSearch},
		{"tool_search_1p", BetaToolSearch1P},
		{"tool_search_3p", BetaToolSearch3P},
		{"effort", BetaEffort},
		{"task_budgets", BetaTaskBudgets},
		{"prompt_caching_scope", BetaPromptCachingScope},
		{"fast_mode", BetaFastMode},
		{"redact_thinking", BetaRedactThinking},
		{"advisor", BetaAdvisor},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			if tc.value == "" {
				t.Error("beta constant should not be empty")
			}
		})
	}
}

func TestBetaConstantValues(t *testing.T) {
	// Source: constants/betas.ts:3-31 — exact string values
	if BetaClaudeCode != "claude-code-20250219" {
		t.Errorf("BetaClaudeCode = %q", BetaClaudeCode)
	}
	if BetaInterleavedThinking != "interleaved-thinking-2025-05-14" {
		t.Errorf("BetaInterleavedThinking = %q", BetaInterleavedThinking)
	}
	if BetaContext1M != "context-1m-2025-08-07" {
		t.Errorf("BetaContext1M = %q", BetaContext1M)
	}
	if BetaEffort != "effort-2025-11-24" {
		t.Errorf("BetaEffort = %q", BetaEffort)
	}
	if BetaRedactThinking != "redact-thinking-2026-02-12" {
		t.Errorf("BetaRedactThinking = %q", BetaRedactThinking)
	}
}

func TestBedrockExtraParamsHeaders(t *testing.T) {
	// Source: constants/betas.ts:38-42
	if len(BedrockExtraParamsHeaders) != 3 {
		t.Errorf("expected 3 Bedrock extra params headers, got %d", len(BedrockExtraParamsHeaders))
	}
	expected := []string{BetaInterleavedThinking, BetaContext1M, BetaToolSearch3P}
	for _, e := range expected {
		if !BedrockExtraParamsHeaders[e] {
			t.Errorf("expected %q in Bedrock extra params", e)
		}
	}
}

func TestGetToolSearchBetaHeader(t *testing.T) {
	// Source: utils/betas.ts:202-208
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("1p_uses_advanced_tool_use", func(t *testing.T) {
		if r := GetToolSearchBetaHeader(); r != BetaToolSearch1P {
			t.Errorf("got %q, want %q", r, BetaToolSearch1P)
		}
	})

	t.Run("vertex_uses_tool_search_tool", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_USE_VERTEX", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
		if r := GetToolSearchBetaHeader(); r != BetaToolSearch3P {
			t.Errorf("got %q, want %q", r, BetaToolSearch3P)
		}
	})

	t.Run("bedrock_uses_tool_search_tool", func(t *testing.T) {
		os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
		defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
		if r := GetToolSearchBetaHeader(); r != BetaToolSearch3P {
			t.Errorf("got %q, want %q", r, BetaToolSearch3P)
		}
	})
}

func TestGetAllModelBetas(t *testing.T) {
	// Source: utils/betas.ts:234-369
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
	os.Unsetenv("DISABLE_INTERLEAVED_THINKING")
	os.Unsetenv("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS")
	os.Unsetenv("ANTHROPIC_BETAS")

	t.Run("non_haiku_has_claude_code_beta", func(t *testing.T) {
		// Source: betas.ts:241 — non-Haiku includes claude-code beta
		betas := GetAllModelBetas("claude-opus-4-6")
		found := false
		for _, b := range betas {
			if b == BetaClaudeCode {
				found = true
				break
			}
		}
		if !found {
			t.Error("non-Haiku model should include claude-code beta")
		}
	})

	t.Run("haiku_excludes_claude_code_beta", func(t *testing.T) {
		// Source: betas.ts:236 — Haiku excludes claude-code beta
		betas := GetAllModelBetas("claude-haiku-4-5-20251001")
		for _, b := range betas {
			if b == BetaClaudeCode {
				t.Error("Haiku should NOT include claude-code beta")
			}
		}
	})

	t.Run("1m_model_has_context_1m_beta", func(t *testing.T) {
		// Source: betas.ts:254-256
		betas := GetAllModelBetas("claude-opus-4-6[1m]")
		found := false
		for _, b := range betas {
			if b == BetaContext1M {
				found = true
			}
		}
		if !found {
			t.Error("1M model should include context-1m beta")
		}
	})

	t.Run("non_1m_no_context_1m_beta", func(t *testing.T) {
		betas := GetAllModelBetas("claude-opus-4-6")
		for _, b := range betas {
			if b == BetaContext1M {
				t.Error("non-1M model should NOT include context-1m beta")
			}
		}
	})

	t.Run("opus_has_interleaved_thinking", func(t *testing.T) {
		// Source: betas.ts:257-262
		betas := GetAllModelBetas("claude-opus-4-6")
		found := false
		for _, b := range betas {
			if b == BetaInterleavedThinking {
				found = true
			}
		}
		if !found {
			t.Error("opus should include interleaved thinking beta")
		}
	})

	t.Run("disable_interleaved_thinking_env", func(t *testing.T) {
		os.Setenv("DISABLE_INTERLEAVED_THINKING", "1")
		defer os.Unsetenv("DISABLE_INTERLEAVED_THINKING")
		betas := GetAllModelBetas("claude-opus-4-6")
		for _, b := range betas {
			if b == BetaInterleavedThinking {
				t.Error("should not include interleaved thinking when disabled")
			}
		}
	})

	t.Run("firstParty_has_prompt_caching_scope", func(t *testing.T) {
		// Source: betas.ts:355-357
		betas := GetAllModelBetas("claude-opus-4-6")
		found := false
		for _, b := range betas {
			if b == BetaPromptCachingScope {
				found = true
			}
		}
		if !found {
			t.Error("firstParty should include prompt caching scope beta")
		}
	})

	t.Run("anthropic_betas_env_merged", func(t *testing.T) {
		// Source: betas.ts:360-367
		os.Setenv("ANTHROPIC_BETAS", "custom-beta-1,custom-beta-2")
		defer os.Unsetenv("ANTHROPIC_BETAS")
		betas := GetAllModelBetas("claude-opus-4-6")
		foundCustom1 := false
		foundCustom2 := false
		for _, b := range betas {
			if b == "custom-beta-1" {
				foundCustom1 = true
			}
			if b == "custom-beta-2" {
				foundCustom2 = true
			}
		}
		if !foundCustom1 || !foundCustom2 {
			t.Errorf("ANTHROPIC_BETAS env should be merged: custom1=%v custom2=%v", foundCustom1, foundCustom2)
		}
	})
}

func TestGetModelBetas_BedrockFiltering(t *testing.T) {
	// Source: utils/betas.ts:371-377
	os.Setenv("CLAUDE_CODE_USE_BEDROCK", "1")
	defer os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("DISABLE_INTERLEAVED_THINKING")

	betas := GetModelBetas("claude-opus-4-6")
	for _, b := range betas {
		if BedrockExtraParamsHeaders[b] {
			t.Errorf("Bedrock model betas should not include %q (it belongs in extraBodyParams)", b)
		}
	}

	// Verify the extra params DO have those betas
	extra := GetBedrockExtraBodyParamsBetas("claude-opus-4-6")
	if len(extra) == 0 {
		t.Error("Bedrock extra body params should have some betas")
	}
}

func TestGetMergedBetas_AgenticQuery(t *testing.T) {
	// Source: utils/betas.ts:401-418
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")
	os.Unsetenv("CLAUDE_CODE_DISABLE_EXPERIMENTAL_BETAS")

	t.Run("haiku_agentic_gets_claude_code_beta", func(t *testing.T) {
		// Source: betas.ts:407-409 — agentic query adds claude-code for Haiku
		betas := GetMergedBetas("claude-haiku-4-5-20251001", true)
		found := false
		for _, b := range betas {
			if b == BetaClaudeCode {
				found = true
			}
		}
		if !found {
			t.Error("Haiku agentic query should include claude-code beta")
		}
	})

	t.Run("haiku_non_agentic_no_claude_code", func(t *testing.T) {
		betas := GetMergedBetas("claude-haiku-4-5-20251001", false)
		for _, b := range betas {
			if b == BetaClaudeCode {
				t.Error("Haiku non-agentic should NOT include claude-code beta")
			}
		}
	})
}

func TestModelSupportsISP(t *testing.T) {
	// Source: utils/betas.ts:92-112
	os.Unsetenv("CLAUDE_CODE_USE_BEDROCK")
	os.Unsetenv("CLAUDE_CODE_USE_VERTEX")
	os.Unsetenv("CLAUDE_CODE_USE_FOUNDRY")

	t.Run("opus46_supports", func(t *testing.T) {
		if !ModelSupportsISP("claude-opus-4-6") {
			t.Error("opus 4.6 should support ISP")
		}
	})
	t.Run("sonnet46_supports", func(t *testing.T) {
		if !ModelSupportsISP("claude-sonnet-4-6") {
			t.Error("sonnet 4.6 should support ISP")
		}
	})
	t.Run("haiku45_supports_on_1p", func(t *testing.T) {
		// Source: betas.ts:107 — 1P: all non-claude-3 models
		if !ModelSupportsISP("claude-haiku-4-5-20251001") {
			t.Error("haiku 4.5 should support ISP on 1P")
		}
	})
	t.Run("claude3_no_isp_on_1p", func(t *testing.T) {
		// Source: betas.ts:107
		if ModelSupportsISP("claude-3-5-sonnet-20241022") {
			t.Error("claude 3.x should NOT support ISP on 1P")
		}
	})
}
