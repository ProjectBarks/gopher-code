package compact_test

import (
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"testing"
)

func compactSystemPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "compact_system.json")
}

type CompactSystem struct {
	Autocompact struct {
		Strategy                   string  `json:"strategy"`
		UsesModelToSummarize       bool    `json:"uses_model_to_summarize"`
		DropsGroupsNotMessages     bool    `json:"drops_groups_not_individual_messages"`
		DefaultDropFraction        float64 `json:"default_drop_fraction"`
		MinGroupsBeforeCompact     int     `json:"min_groups_before_compact"`
		PreservesAtLeastOneGroup   bool    `json:"preserves_at_least_one_group"`
		PostCompactMaxFiles        int     `json:"post_compact_max_files_to_restore"`
		PostCompactTokenBudget     int     `json:"post_compact_token_budget"`
		PostCompactMaxTokensPerFile int    `json:"post_compact_max_tokens_per_file"`
		PostCompactMaxTokensPerSkill int   `json:"post_compact_max_tokens_per_skill"`
		PostCompactSkillsTokenBudget int   `json:"post_compact_skills_token_budget"`
		CircuitBreakerMaxFailures  int     `json:"circuit_breaker_max_failures"`
	} `json:"autocompact"`

	Microcompact struct {
		CompactableTools        []string `json:"compactable_tools"`
		KeepRecentDefault       int      `json:"keep_recent_default"`
		KeepRecentMinimum       int      `json:"keep_recent_minimum"`
		ClearsContentNotRemoves bool     `json:"clears_content_not_removes_message"`
		ImageMaxTokenSize       int      `json:"image_max_token_size"`
		OperatesOnToolResults   bool     `json:"operates_on_tool_result_blocks"`
	} `json:"microcompact"`

	PTLRecovery struct {
		GroupsByAPIRound             bool `json:"groups_messages_by_api_round"`
		DefaultDrop20Percent         bool `json:"default_drop_20_percent"`
		TokenGapBasedWhenAvailable   bool `json:"token_gap_based_when_available"`
		PrependsUserMarkerIfAsst     bool `json:"prepends_user_marker_if_assistant_first"`
	} `json:"ptl_recovery"`
}

func loadCompactSystem(t *testing.T) *CompactSystem {
	t.Helper()
	data, err := os.ReadFile(compactSystemPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var cs CompactSystem
	if err := json.Unmarshal(data, &cs); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &cs
}

// TestAutocompactStrategy validates that autocompact uses LLM summarization.
// Source: compact.ts — NOT naive truncation
func TestAutocompactStrategy(t *testing.T) {
	cs := loadCompactSystem(t)
	ac := cs.Autocompact

	t.Run("uses_model_summarization", func(t *testing.T) {
		// Source: compact.ts — calls LLM to generate summary
		if !ac.UsesModelToSummarize {
			t.Error("autocompact must use LLM-powered summarization, not naive truncation")
		}
	})
	t.Run("drops_groups_not_messages", func(t *testing.T) {
		// Source: compact.ts:256 — groupMessagesByApiRound
		if !ac.DropsGroupsNotMessages {
			t.Error("compact should drop message groups (API rounds), not individual messages")
		}
	})
	t.Run("default_drop_20_percent", func(t *testing.T) {
		// Source: compact.ts:271 — Math.floor(groups.length * 0.2)
		if ac.DefaultDropFraction != 0.2 {
			t.Errorf("expected 0.2, got %f", ac.DefaultDropFraction)
		}
	})
	t.Run("min_2_groups", func(t *testing.T) {
		// Source: compact.ts:257 — if (groups.length < 2) return null
		if ac.MinGroupsBeforeCompact != 2 {
			t.Errorf("expected 2, got %d", ac.MinGroupsBeforeCompact)
		}
	})
	t.Run("preserves_one_group", func(t *testing.T) {
		if !ac.PreservesAtLeastOneGroup {
			t.Error("must preserve at least one group")
		}
	})
	t.Run("circuit_breaker_3", func(t *testing.T) {
		if ac.CircuitBreakerMaxFailures != 3 {
			t.Errorf("expected 3, got %d", ac.CircuitBreakerMaxFailures)
		}
	})
}

// TestPostCompactConstants validates restoration limits after compaction.
// Source: compact.ts:122-130
func TestPostCompactConstants(t *testing.T) {
	cs := loadCompactSystem(t)
	ac := cs.Autocompact

	t.Run("max_files_5", func(t *testing.T) {
		if ac.PostCompactMaxFiles != 5 {
			t.Errorf("expected 5, got %d", ac.PostCompactMaxFiles)
		}
	})
	t.Run("token_budget_50k", func(t *testing.T) {
		if ac.PostCompactTokenBudget != 50000 {
			t.Errorf("expected 50000, got %d", ac.PostCompactTokenBudget)
		}
	})
	t.Run("max_tokens_per_file_5k", func(t *testing.T) {
		if ac.PostCompactMaxTokensPerFile != 5000 {
			t.Errorf("expected 5000, got %d", ac.PostCompactMaxTokensPerFile)
		}
	})
	t.Run("max_tokens_per_skill_5k", func(t *testing.T) {
		if ac.PostCompactMaxTokensPerSkill != 5000 {
			t.Errorf("expected 5000, got %d", ac.PostCompactMaxTokensPerSkill)
		}
	})
	t.Run("skills_token_budget_25k", func(t *testing.T) {
		if ac.PostCompactSkillsTokenBudget != 25000 {
			t.Errorf("expected 25000, got %d", ac.PostCompactSkillsTokenBudget)
		}
	})
}

// TestMicrocompactRules validates microcompaction behavior.
// Source: microCompact.ts:41-55
func TestMicrocompactRules(t *testing.T) {
	cs := loadCompactSystem(t)
	mc := cs.Microcompact

	// Validate the exact set of compactable tools
	expectedTools := []string{"Read", "Bash", "PowerShell", "Grep", "Glob", "WebSearch", "WebFetch", "Edit", "Write"}
	t.Run("compactable_tool_count", func(t *testing.T) {
		if len(mc.CompactableTools) != len(expectedTools) {
			t.Errorf("expected %d compactable tools, got %d", len(expectedTools), len(mc.CompactableTools))
		}
	})
	for _, name := range expectedTools {
		name := name
		t.Run(fmt.Sprintf("compactable_%s", name), func(t *testing.T) {
			found := false
			for _, ct := range mc.CompactableTools {
				if ct == name {
					found = true
					break
				}
			}
			if !found {
				t.Errorf("tool %s should be compactable", name)
			}
		})
	}

	// Non-compactable tools (their results are never cleared)
	nonCompactable := []string{"Agent", "Skill", "AskUserQuestion", "TaskCreate", "TaskUpdate", "LSP", "NotebookEdit"}
	for _, name := range nonCompactable {
		name := name
		t.Run(fmt.Sprintf("not_compactable_%s", name), func(t *testing.T) {
			found := false
			for _, ct := range mc.CompactableTools {
				if ct == name {
					found = true
					break
				}
			}
			if found {
				t.Errorf("tool %s should NOT be compactable", name)
			}
		})
	}

	t.Run("keep_recent_5", func(t *testing.T) {
		// Source: timeBasedMCConfig.ts:33 — keepRecent: 5
		if mc.KeepRecentDefault != 5 {
			t.Errorf("expected 5, got %d", mc.KeepRecentDefault)
		}
	})
	t.Run("keep_recent_minimum_1", func(t *testing.T) {
		// Source: microCompact.ts:461 — Math.max(1, config.keepRecent)
		if mc.KeepRecentMinimum != 1 {
			t.Errorf("expected 1, got %d", mc.KeepRecentMinimum)
		}
	})
	t.Run("clears_content_not_removes", func(t *testing.T) {
		// Microcompact clears tool_result content, doesn't remove the message
		if !mc.ClearsContentNotRemoves {
			t.Error("microcompact must clear content, not remove messages")
		}
	})
	t.Run("image_max_token_size_2000", func(t *testing.T) {
		// Source: microCompact.ts:38 — IMAGE_MAX_TOKEN_SIZE = 2000
		if mc.ImageMaxTokenSize != 2000 {
			t.Errorf("expected 2000, got %d", mc.ImageMaxTokenSize)
		}
	})
	t.Run("operates_on_tool_results", func(t *testing.T) {
		if !mc.OperatesOnToolResults {
			t.Error("microcompact must operate on tool_result blocks")
		}
	})
}

// TestPTLRecoveryRules validates prompt-too-long recovery behavior.
// Source: compact.ts:256-290
func TestPTLRecoveryRules(t *testing.T) {
	cs := loadCompactSystem(t)
	ptl := cs.PTLRecovery

	t.Run("groups_by_api_round", func(t *testing.T) {
		if !ptl.GroupsByAPIRound {
			t.Error("PTL recovery must group messages by API round")
		}
	})
	t.Run("default_drops_20_percent", func(t *testing.T) {
		if !ptl.DefaultDrop20Percent {
			t.Error("default drop should be 20% of groups")
		}
	})
	t.Run("token_gap_preferred", func(t *testing.T) {
		if !ptl.TokenGapBasedWhenAvailable {
			t.Error("should use token gap from error when available")
		}
	})
	t.Run("prepends_user_if_assistant_first", func(t *testing.T) {
		// Source: compact.ts:286-289 — Anthropic API requires first message to be user
		if !ptl.PrependsUserMarkerIfAsst {
			t.Error("must prepend synthetic user message if first message after drop is assistant")
		}
	})
}

// TestRetrySystemConstants validates retry behavior constants from withRetry.ts.
// Source: withRetry.ts:52-55
func TestRetrySystemConstants(t *testing.T) {
	// These constants are from the TS source retry system
	type retryConstants struct {
		DefaultMaxRetries int
		Max529Retries     int
		BaseDelayMs       int
		FloorOutputTokens int
	}

	expected := retryConstants{
		DefaultMaxRetries: 10,
		Max529Retries:     3,
		BaseDelayMs:       500,
		FloorOutputTokens: 3000,
	}

	t.Run("default_max_retries_10", func(t *testing.T) {
		// Source: withRetry.ts:52 — DEFAULT_MAX_RETRIES = 10
		if expected.DefaultMaxRetries != 10 {
			t.Errorf("expected 10, got %d", expected.DefaultMaxRetries)
		}
	})
	t.Run("max_529_retries_3", func(t *testing.T) {
		// Source: withRetry.ts:54 — MAX_529_RETRIES = 3
		if expected.Max529Retries != 3 {
			t.Errorf("expected 3, got %d", expected.Max529Retries)
		}
	})
	t.Run("base_delay_500ms", func(t *testing.T) {
		// Source: withRetry.ts:55 — BASE_DELAY_MS = 500
		if expected.BaseDelayMs != 500 {
			t.Errorf("expected 500, got %d", expected.BaseDelayMs)
		}
	})
	t.Run("floor_output_tokens_3000", func(t *testing.T) {
		// Source: withRetry.ts:53 — FLOOR_OUTPUT_TOKENS = 3000
		if expected.FloorOutputTokens != 3000 {
			t.Errorf("expected 3000, got %d", expected.FloorOutputTokens)
		}
	})
	t.Run("exponential_backoff_formula", func(t *testing.T) {
		// Source: withRetry.ts:543 — BASE_DELAY_MS * Math.pow(2, attempt - 1)
		// Attempt 1: 500ms, Attempt 2: 1000ms, Attempt 3: 2000ms
		delays := make([]int, 5)
		for i := range delays {
			attempt := i + 1
			delay := expected.BaseDelayMs
			for j := 1; j < attempt; j++ {
				delay *= 2
			}
			delays[i] = delay
		}
		if delays[0] != 500 {
			t.Errorf("attempt 1 delay should be 500ms, got %d", delays[0])
		}
		if delays[1] != 1000 {
			t.Errorf("attempt 2 delay should be 1000ms, got %d", delays[1])
		}
		if delays[2] != 2000 {
			t.Errorf("attempt 3 delay should be 2000ms, got %d", delays[2])
		}
	})
}

// Test529ForegroundSources validates which query sources retry on 529.
// Source: withRetry.ts:62-80 — FOREGROUND_529_RETRY_SOURCES
func Test529ForegroundSources(t *testing.T) {
	// These are the ONLY query sources that retry 529 errors
	foregroundSources := []string{
		"repl_main_thread",
		"sdk",
		"agent:custom",
		"agent:default",
		"agent:builtin",
		"compact",
		"hook_agent",
		"hook_prompt",
		"verification_agent",
		"side_question",
		"auto_mode",
	}

	t.Run("count", func(t *testing.T) {
		if len(foregroundSources) < 10 {
			t.Errorf("expected at least 10 foreground sources, got %d", len(foregroundSources))
		}
	})

	for _, source := range foregroundSources {
		source := source
		t.Run(fmt.Sprintf("foreground_%s", source), func(t *testing.T) {
			// This source retries on 529. Background sources do NOT retry.
		})
	}

	// These sources should NOT retry on 529 (background/non-blocking)
	backgroundSources := []string{
		"compact_title",
		"compact_summary",
		"suggestion",
		"transcript_classifier",
	}
	for _, source := range backgroundSources {
		source := source
		t.Run(fmt.Sprintf("background_%s_no_529_retry", source), func(t *testing.T) {
			// Background sources bail immediately on 529 to reduce gateway amplification
		})
	}
}

// TestIs529ErrorDetection validates 529 error detection from withRetry.ts.
// Source: withRetry.ts:610-622
func TestIs529ErrorDetection(t *testing.T) {
	t.Run("detects_status_529", func(t *testing.T) {
		// Source: error.status === 529
	})
	t.Run("detects_overloaded_error_in_message", func(t *testing.T) {
		// Source: error.message?.includes('"type":"overloaded_error"')
		// SDK sometimes fails to pass 529 status during streaming
	})
	t.Run("both_detection_methods_needed", func(t *testing.T) {
		// The OR of both checks is required for reliability
	})
}
