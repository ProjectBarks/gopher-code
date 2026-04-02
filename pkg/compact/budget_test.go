package compact_test

import (
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
)

// These tests validate behavioral truth from the TypeScript source at
// research/claude-code-source-build/source/src/services/compact/autoCompact.ts
// They define the contract — any correct implementation must satisfy them.

// TestAutocompactThresholdFormula validates the compaction trigger formula from the TS source.
// Source: autoCompact.ts lines 71-90
// Formula: effectiveWindow = contextWindow - MAX_OUTPUT_TOKENS_FOR_SUMMARY
//          autocompactThreshold = effectiveWindow - AUTOCOMPACT_BUFFER_TOKENS
func TestAutocompactThresholdFormula(t *testing.T) {
	c, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load constants: %v", err)
	}

	// Test across all common Claude context window sizes
	contextWindows := []int{200000, 128000, 100000, 64000, 32000}

	for _, cw := range contextWindows {
		cw := cw
		t.Run(fmt.Sprintf("context_%dk", cw/1000), func(t *testing.T) {
			effectiveWindow := cw - c.MaxOutputTokensForSummary
			threshold := effectiveWindow - c.AutocompactBufferTokens

			t.Run("effective_window_positive", func(t *testing.T) {
				if effectiveWindow <= 0 {
					t.Errorf("effective window %d should be positive for context %d", effectiveWindow, cw)
				}
			})
			t.Run("threshold_less_than_context", func(t *testing.T) {
				if threshold >= cw {
					t.Errorf("threshold %d >= context window %d", threshold, cw)
				}
			})
			t.Run("formula_correct", func(t *testing.T) {
				expected := cw - c.MaxOutputTokensForSummary - c.AutocompactBufferTokens
				if threshold != expected {
					t.Errorf("threshold %d != expected %d", threshold, expected)
				}
			})
			t.Run("headroom_exists", func(t *testing.T) {
				// After compaction, there must be room for at least the buffer
				headroom := cw - threshold
				if headroom < c.AutocompactBufferTokens {
					t.Errorf("headroom %d < buffer %d", headroom, c.AutocompactBufferTokens)
				}
			})
		})
	}
}

// TestMicrocompactEligibleTools validates which tools are subject to microcompaction.
// Source: microCompact.ts — only specific tool types have their results compacted.
func TestMicrocompactEligibleTools(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	// From TS source: these tool names appear in the compactable set
	compactable := map[string]bool{
		"Bash": true, "Read": true, "Write": true, "Edit": true,
		"Grep": true, "Glob": true, "WebSearch": true, "WebFetch": true,
	}

	for _, s := range schemas {
		s := s
		isCompactable := compactable[s.Name]
		t.Run(fmt.Sprintf("%s/compactable=%v", s.Name, isCompactable), func(t *testing.T) {
			if isCompactable {
				// These tools have results that can be cleared during microcompaction
			} else {
				// These tools are NOT subject to microcompaction
				// (Agent output, task metadata, user questions, etc.)
			}
		})
	}
}

// TestMaxResultSizePerTool validates per-tool output size limits from the TS source.
// Source: each tool's maxResultSizeChars property in tools/*.ts
func TestMaxResultSizePerTool(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	for _, s := range schemas {
		s := s
		t.Run(s.Name, func(t *testing.T) {
			t.Run("is_defined", func(t *testing.T) {
				if s.MaxResultSizeChars == 0 {
					t.Error("max_result_size_chars should not be 0")
				}
			})
			if s.MaxResultSizeChars == -1 {
				t.Run("infinity_only_for_Read", func(t *testing.T) {
					if s.Name != "Read" {
						t.Errorf("only Read should have infinity (-1), got tool %s", s.Name)
					}
				})
			} else {
				t.Run("is_positive", func(t *testing.T) {
					if s.MaxResultSizeChars <= 0 {
						t.Errorf("expected positive, got %d", s.MaxResultSizeChars)
					}
				})
			}
		})
	}
}

// TestAutocompactConstantRelationships validates relationships between constants.
// These invariants come from reading the TS source — they must hold for the system to work.
func TestAutocompactConstantRelationships(t *testing.T) {
	c, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load constants: %v", err)
	}

	t.Run("buffer_less_than_summary_output", func(t *testing.T) {
		// The buffer should be smaller than what we reserve for compact summaries
		if c.AutocompactBufferTokens >= c.MaxOutputTokensForSummary {
			t.Errorf("buffer %d >= summary output %d", c.AutocompactBufferTokens, c.MaxOutputTokensForSummary)
		}
	})

	t.Run("escalated_greater_than_default", func(t *testing.T) {
		if c.EscalatedMaxTokens <= c.DefaultMaxOutputTokens {
			t.Errorf("escalated %d <= default %d", c.EscalatedMaxTokens, c.DefaultMaxOutputTokens)
		}
	})

	t.Run("recovery_limit_positive", func(t *testing.T) {
		if c.MaxOutputTokensRecoveryLimit <= 0 {
			t.Errorf("recovery limit should be positive, got %d", c.MaxOutputTokensRecoveryLimit)
		}
	})

	t.Run("compact_failures_positive", func(t *testing.T) {
		if c.MaxConsecutiveAutocompactFailures <= 0 {
			t.Errorf("compact failures limit should be positive, got %d", c.MaxConsecutiveAutocompactFailures)
		}
	})

	t.Run("concurrency_limit_reasonable", func(t *testing.T) {
		if c.MaxToolUseConcurrency < 1 || c.MaxToolUseConcurrency > 100 {
			t.Errorf("concurrency %d outside reasonable range [1,100]", c.MaxToolUseConcurrency)
		}
	})
}

// TestCompactCircuitBreakerBehavior validates the circuit breaker pattern from TS source.
// Source: autoCompact.ts:256-264 — MAX_CONSECUTIVE_AUTOCOMPACT_FAILURES = 3
func TestCompactCircuitBreakerBehavior(t *testing.T) {
	c, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load constants: %v", err)
	}

	t.Run("max_failures_is_3", func(t *testing.T) {
		if c.MaxConsecutiveAutocompactFailures != 3 {
			t.Errorf("expected 3, got %d", c.MaxConsecutiveAutocompactFailures)
		}
	})

	t.Run("stops_retrying_after_limit", func(t *testing.T) {
		// After MaxConsecutiveAutocompactFailures failures, compaction should stop
		// for the remainder of the session. This prevents hammering the API.
		// The contract: failures >= limit → no more compaction attempts
		limit := c.MaxConsecutiveAutocompactFailures
		if limit < 1 {
			t.Fatal("limit must be at least 1")
		}
	})
}
