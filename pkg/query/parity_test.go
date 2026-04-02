package query_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func parityRulesPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "parity_rules.json")
}

type ParityRules struct {
	ToolPartitioning struct {
		Rule     string `json:"rule"`
		Examples []struct {
			Tools   []string `json:"tools"`
			Batches []struct {
				Concurrent bool     `json:"concurrent"`
				Tools      []string `json:"tools"`
			} `json:"batches"`
		} `json:"examples"`
		OnParseFailure string `json:"on_parse_failure"`
		MaxConcurrency int    `json:"max_concurrency"`
	} `json:"tool_partitioning"`

	TurnCounting struct {
		StartsAt       int    `json:"starts_at"`
		IncrementsAfter string `json:"increments_after"`
		MaxTurnsCheck  string `json:"max_turns_check"`
	} `json:"turn_counting"`

	ToolDetection struct {
		Rule   string `json:"rule"`
		Reason string `json:"reason"`
	} `json:"tool_detection"`

	MaxOutputTokensRecovery struct {
		EscalationValue             int    `json:"escalation_value"`
		MultiTurnLimit              int    `json:"multi_turn_limit"`
		ContinuationMessageStartsWith string `json:"continuation_message_starts_with"`
		ContinuationMessageContains   string `json:"continuation_message_contains"`
	} `json:"max_output_tokens_recovery"`

	ContextTooLongRecovery struct {
		MaxAttempts        int  `json:"max_attempts"`
		Strategy           string `json:"strategy"`
		NoStopHooksOnPTL   bool `json:"no_stop_hooks_on_ptl"`
	} `json:"context_too_long_recovery"`

	MessageFormatRules struct {
		ToolResultsRole                 string `json:"tool_results_role"`
		EveryToolUseNeedsExactlyOneResult bool  `json:"every_tool_use_needs_exactly_one_result"`
		MissingResultSynthesized        bool   `json:"missing_result_synthesized"`
		MissingResultText               string `json:"missing_result_text"`
		SystemPromptInSystemField       bool   `json:"system_prompt_in_system_field"`
		SystemPromptNotInMessages       bool   `json:"system_prompt_not_in_messages"`
	} `json:"message_format_rules"`

	StopReasons struct {
		ValidValues []string `json:"valid_values"`
	} `json:"stop_reasons"`

	ErrorClassification struct {
		ContextTooLongKeywords []string `json:"context_too_long_keywords"`
		RateLimitStatus        int      `json:"rate_limit_status"`
		OverloadedStatus       int      `json:"overloaded_status"`
		AuthErrorStatus        int      `json:"auth_error_status"`
		ServerErrorRange       []int    `json:"server_error_range"`
		AuthNeverRetried       bool     `json:"auth_never_retried"`
	} `json:"error_classification"`

	MemoryPrefetch struct {
		FileName            string `json:"file_name"`
		LoadedOnceBeforeLoop bool  `json:"loaded_once_before_loop"`
		MergedIntoSystemPrompt bool `json:"merged_into_system_prompt"`
		MissingFileIsSilent bool   `json:"missing_file_is_silent"`
	} `json:"memory_prefetch"`

	CompactBehavior struct {
		ProactiveCheckBeforeEachTurn bool `json:"proactive_check_before_each_turn"`
		ReactiveOn413Once           bool `json:"reactive_on_413_once"`
		PreservesSystemPrompt       bool `json:"preserves_system_prompt"`
		CircuitBreakerAfter3Failures bool `json:"circuit_breaker_after_3_failures"`
	} `json:"compact_behavior"`
}

func loadParityRules(t *testing.T) *ParityRules {
	t.Helper()
	data, err := os.ReadFile(parityRulesPath())
	if err != nil {
		t.Fatalf("failed to load parity rules: %v", err)
	}
	var rules ParityRules
	if err := json.Unmarshal(data, &rules); err != nil {
		t.Fatalf("failed to parse parity rules: %v", err)
	}
	return &rules
}

// TestToolPartitioningRules validates the tool batching strategy from toolOrchestration.ts.
func TestToolPartitioningRules(t *testing.T) {
	rules := loadParityRules(t)
	tp := rules.ToolPartitioning

	t.Run("max_concurrency_10", func(t *testing.T) {
		if tp.MaxConcurrency != 10 {
			t.Errorf("expected 10, got %d", tp.MaxConcurrency)
		}
	})

	t.Run("parse_failure_conservative", func(t *testing.T) {
		expected := "treat as not concurrency-safe (conservative)"
		if tp.OnParseFailure != expected {
			t.Errorf("expected %q, got %q", expected, tp.OnParseFailure)
		}
	})

	t.Run("has_partitioning_examples", func(t *testing.T) {
		if len(tp.Examples) < 5 {
			t.Errorf("expected at least 5 examples, got %d", len(tp.Examples))
		}
	})

	// Validate each example
	for i, ex := range tp.Examples {
		ex := ex
		t.Run(fmt.Sprintf("example_%d_tools_%v", i, ex.Tools), func(t *testing.T) {
			// Count total tools across batches
			totalInBatches := 0
			for _, batch := range ex.Batches {
				totalInBatches += len(batch.Tools)
			}
			t.Run("tool_count_matches", func(t *testing.T) {
				if totalInBatches != len(ex.Tools) {
					t.Errorf("batches contain %d tools, input has %d", totalInBatches, len(ex.Tools))
				}
			})

			// Verify mutating batches have exactly 1 tool
			for j, batch := range ex.Batches {
				if !batch.Concurrent {
					t.Run(fmt.Sprintf("batch_%d_mutating_size_1", j), func(t *testing.T) {
						if len(batch.Tools) != 1 {
							t.Errorf("mutating batch should have 1 tool, got %d", len(batch.Tools))
						}
					})
				}
			}
		})
	}
}

// TestTurnCountingRules validates turn counter behavior from query.ts.
func TestTurnCountingRules(t *testing.T) {
	rules := loadParityRules(t)
	tc := rules.TurnCounting

	t.Run("starts_at_1", func(t *testing.T) {
		// Source: query.ts:276 — turnCount: 1
		if tc.StartsAt != 1 {
			t.Errorf("expected 1, got %d", tc.StartsAt)
		}
	})

	t.Run("max_turns_uses_strict_gt", func(t *testing.T) {
		// Source: query.ts:1679 — nextTurnCount > maxTurns (not >=)
		expected := "nextTurnCount > maxTurns (strict greater than, not >=)"
		if tc.MaxTurnsCheck != expected {
			t.Errorf("expected %q, got %q", expected, tc.MaxTurnsCheck)
		}
	})
}

// TestToolDetectionRules validates how tool_use is detected.
func TestToolDetectionRules(t *testing.T) {
	rules := loadParityRules(t)
	td := rules.ToolDetection

	t.Run("uses_content_blocks_not_stop_reason", func(t *testing.T) {
		// Source: query.ts:554 — "stop_reason === 'tool_use' is unreliable"
		if td.Reason == "" {
			t.Error("should document why stop_reason is unreliable")
		}
	})
}

// TestMaxOutputTokensRecoveryRules validates recovery behavior.
func TestMaxOutputTokensRecoveryRules(t *testing.T) {
	rules := loadParityRules(t)
	r := rules.MaxOutputTokensRecovery

	t.Run("escalation_to_64k", func(t *testing.T) {
		if r.EscalationValue != 64000 {
			t.Errorf("expected 64000, got %d", r.EscalationValue)
		}
	})
	t.Run("multi_turn_limit_3", func(t *testing.T) {
		if r.MultiTurnLimit != 3 {
			t.Errorf("expected 3, got %d", r.MultiTurnLimit)
		}
	})
	t.Run("continuation_message_starts_with", func(t *testing.T) {
		expected := "Output token limit hit. Resume directly"
		if r.ContinuationMessageStartsWith != expected {
			t.Errorf("expected %q, got %q", expected, r.ContinuationMessageStartsWith)
		}
	})
	t.Run("continuation_message_says_no_apology", func(t *testing.T) {
		if r.ContinuationMessageContains != "no apology, no recap" {
			t.Errorf("expected 'no apology, no recap', got %q", r.ContinuationMessageContains)
		}
	})
}

// TestContextTooLongRecoveryRules validates 413 handling.
func TestContextTooLongRecoveryRules(t *testing.T) {
	rules := loadParityRules(t)
	r := rules.ContextTooLongRecovery

	t.Run("max_attempts_1", func(t *testing.T) {
		if r.MaxAttempts != 1 {
			t.Errorf("expected 1, got %d", r.MaxAttempts)
		}
	})
	t.Run("compact_then_retry", func(t *testing.T) {
		if r.Strategy != "compact then retry once" {
			t.Errorf("expected 'compact then retry once', got %q", r.Strategy)
		}
	})
	t.Run("no_stop_hooks_on_prompt_too_long", func(t *testing.T) {
		if !r.NoStopHooksOnPTL {
			t.Error("should not run stop hooks on prompt_too_long")
		}
	})
}

// TestMessageFormatRules validates API message format contract.
func TestMessageFormatRules(t *testing.T) {
	rules := loadParityRules(t)
	mf := rules.MessageFormatRules

	t.Run("tool_results_in_user_role", func(t *testing.T) {
		if mf.ToolResultsRole != "user" {
			t.Errorf("expected 'user', got %q", mf.ToolResultsRole)
		}
	})
	t.Run("every_tool_use_needs_result", func(t *testing.T) {
		if !mf.EveryToolUseNeedsExactlyOneResult {
			t.Error("every tool_use must have exactly one tool_result")
		}
	})
	t.Run("missing_results_synthesized", func(t *testing.T) {
		if !mf.MissingResultSynthesized {
			t.Error("missing tool results must be synthesized")
		}
	})
	t.Run("missing_result_placeholder_text", func(t *testing.T) {
		expected := "[Tool result missing due to internal error]"
		if mf.MissingResultText != expected {
			t.Errorf("expected %q, got %q", expected, mf.MissingResultText)
		}
	})
	t.Run("system_prompt_separate_field", func(t *testing.T) {
		if !mf.SystemPromptInSystemField {
			t.Error("system prompt must be in system field")
		}
	})
	t.Run("system_prompt_not_in_messages", func(t *testing.T) {
		if !mf.SystemPromptNotInMessages {
			t.Error("system prompt must NOT be in messages array")
		}
	})
}

// TestStopReasons validates all valid stop reasons from the TS source.
func TestStopReasons(t *testing.T) {
	rules := loadParityRules(t)

	expectedReasons := []string{
		"completed", "max_turns", "aborted_streaming", "aborted_tools",
		"hook_stopped", "image_error", "model_error", "prompt_too_long", "blocking_limit",
	}

	t.Run("count", func(t *testing.T) {
		if len(rules.StopReasons.ValidValues) != len(expectedReasons) {
			t.Errorf("expected %d stop reasons, got %d", len(expectedReasons), len(rules.StopReasons.ValidValues))
		}
	})

	for _, reason := range expectedReasons {
		reason := reason
		t.Run(fmt.Sprintf("has_%s", reason), func(t *testing.T) {
			found := false
			for _, v := range rules.StopReasons.ValidValues {
				if v == reason {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("stop reason %q not found", reason)
			}
		})
	}
}

// TestErrorClassificationRules validates error categorization from TS source.
func TestErrorClassificationRules(t *testing.T) {
	rules := loadParityRules(t)
	ec := rules.ErrorClassification

	t.Run("context_too_long_keywords", func(t *testing.T) {
		if len(ec.ContextTooLongKeywords) < 2 {
			t.Errorf("expected at least 2 keywords, got %d", len(ec.ContextTooLongKeywords))
		}
	})
	t.Run("has_context_too_long_keyword", func(t *testing.T) {
		found := false
		for _, k := range ec.ContextTooLongKeywords {
			if k == "context_too_long" {
				found = true
			}
		}
		if !found {
			t.Error("missing 'context_too_long' keyword")
		}
	})
	t.Run("has_prompt_too_long_keyword", func(t *testing.T) {
		found := false
		for _, k := range ec.ContextTooLongKeywords {
			if k == "prompt is too long" {
				found = true
			}
		}
		if !found {
			t.Error("missing 'prompt is too long' keyword")
		}
	})
	t.Run("rate_limit_429", func(t *testing.T) {
		if ec.RateLimitStatus != 429 {
			t.Errorf("expected 429, got %d", ec.RateLimitStatus)
		}
	})
	t.Run("overloaded_529", func(t *testing.T) {
		if ec.OverloadedStatus != 529 {
			t.Errorf("expected 529, got %d", ec.OverloadedStatus)
		}
	})
	t.Run("auth_401", func(t *testing.T) {
		if ec.AuthErrorStatus != 401 {
			t.Errorf("expected 401, got %d", ec.AuthErrorStatus)
		}
	})
	t.Run("server_error_range_500_599", func(t *testing.T) {
		if len(ec.ServerErrorRange) != 2 || ec.ServerErrorRange[0] != 500 || ec.ServerErrorRange[1] != 599 {
			t.Errorf("expected [500,599], got %v", ec.ServerErrorRange)
		}
	})
	t.Run("auth_never_retried", func(t *testing.T) {
		if !ec.AuthNeverRetried {
			t.Error("auth errors must never be retried")
		}
	})
}

// TestMemoryPrefetchRules validates CLAUDE.md loading behavior.
func TestMemoryPrefetchRules(t *testing.T) {
	rules := loadParityRules(t)
	mp := rules.MemoryPrefetch

	t.Run("file_name_CLAUDE_md", func(t *testing.T) {
		if mp.FileName != "CLAUDE.md" {
			t.Errorf("expected CLAUDE.md, got %s", mp.FileName)
		}
	})
	t.Run("loaded_once_before_loop", func(t *testing.T) {
		if !mp.LoadedOnceBeforeLoop {
			t.Error("CLAUDE.md must be loaded once before the loop, not per-turn")
		}
	})
	t.Run("merged_into_system_prompt", func(t *testing.T) {
		if !mp.MergedIntoSystemPrompt {
			t.Error("CLAUDE.md content must be merged into system prompt")
		}
	})
	t.Run("missing_file_silent", func(t *testing.T) {
		if !mp.MissingFileIsSilent {
			t.Error("missing CLAUDE.md must not produce an error")
		}
	})
}

// TestCompactBehaviorRules validates compaction strategy.
func TestCompactBehaviorRules(t *testing.T) {
	rules := loadParityRules(t)
	cb := rules.CompactBehavior

	t.Run("proactive_before_each_turn", func(t *testing.T) {
		if !cb.ProactiveCheckBeforeEachTurn {
			t.Error("compaction threshold must be checked before each turn")
		}
	})
	t.Run("reactive_on_413_once", func(t *testing.T) {
		if !cb.ReactiveOn413Once {
			t.Error("reactive compaction on 413 must happen at most once")
		}
	})
	t.Run("preserves_system_prompt", func(t *testing.T) {
		if !cb.PreservesSystemPrompt {
			t.Error("compaction must preserve the system prompt")
		}
	})
	t.Run("circuit_breaker_after_3", func(t *testing.T) {
		if !cb.CircuitBreakerAfter3Failures {
			t.Error("compaction must stop after 3 consecutive failures")
		}
	})
}
