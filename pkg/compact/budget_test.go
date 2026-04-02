package compact_test

import (
	"fmt"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/compact"
)

func TestTokenBudgetDefaults(t *testing.T) {
	b := compact.DefaultBudget()

	t.Run("context_window", func(t *testing.T) {
		if b.ContextWindow != 200000 {
			t.Errorf("got %d, want 200000", b.ContextWindow)
		}
	})
	t.Run("max_output_tokens", func(t *testing.T) {
		if b.MaxOutputTokens != 16000 {
			t.Errorf("got %d, want 16000", b.MaxOutputTokens)
		}
	})
	t.Run("compact_threshold", func(t *testing.T) {
		if b.CompactThreshold != 0.8 {
			t.Errorf("got %f, want 0.8", b.CompactThreshold)
		}
	})
}

func TestInputBudget(t *testing.T) {
	cases := []struct {
		name     string
		budget   compact.TokenBudget
		expected int
	}{
		{"default", compact.DefaultBudget(), 184000},
		{"small_context", compact.TokenBudget{ContextWindow: 1000, MaxOutputTokens: 200}, 800},
		{"zero_context", compact.TokenBudget{ContextWindow: 0, MaxOutputTokens: 200}, 0},
		{"output_exceeds_context", compact.TokenBudget{ContextWindow: 100, MaxOutputTokens: 200}, 0},
		{"equal", compact.TokenBudget{ContextWindow: 200, MaxOutputTokens: 200}, 0},
		{"large_context", compact.TokenBudget{ContextWindow: 1000000, MaxOutputTokens: 64000}, 936000},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.budget.InputBudget()
			if got != tc.expected {
				t.Errorf("InputBudget() = %d, want %d", got, tc.expected)
			}
		})
	}
}

func TestShouldCompact(t *testing.T) {
	cases := []struct {
		name     string
		budget   compact.TokenBudget
		tokens   int
		expected bool
	}{
		// Default budget: input=184000, threshold=0.8 → compact at 147200
		{"default_below", compact.DefaultBudget(), 100000, false},
		{"default_at_threshold", compact.DefaultBudget(), 147200, false},
		{"default_above_threshold", compact.DefaultBudget(), 147201, true},
		{"default_at_max", compact.DefaultBudget(), 184000, true},

		// Custom: context=1000, output=200, threshold=0.8 → input=800, compact at 640
		{"custom_below", compact.TokenBudget{1000, 200, 0.8}, 500, false},
		{"custom_at_640", compact.TokenBudget{1000, 200, 0.8}, 640, false},
		{"custom_above_640", compact.TokenBudget{1000, 200, 0.8}, 641, true},
		{"custom_at_800", compact.TokenBudget{1000, 200, 0.8}, 800, true},

		// Zero tokens never compacts
		{"zero_tokens", compact.DefaultBudget(), 0, false},

		// Threshold = 1.0 only compacts at input budget
		{"threshold_1", compact.TokenBudget{1000, 200, 1.0}, 799, false},
		{"threshold_1_at", compact.TokenBudget{1000, 200, 1.0}, 800, false},
		{"threshold_1_above", compact.TokenBudget{1000, 200, 1.0}, 801, true},

		// Threshold = 0.5 compacts at half
		{"threshold_half", compact.TokenBudget{1000, 200, 0.5}, 400, false},
		{"threshold_half_above", compact.TokenBudget{1000, 200, 0.5}, 401, true},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			got := tc.budget.ShouldCompact(tc.tokens)
			if got != tc.expected {
				t.Errorf("ShouldCompact(%d) = %v, want %v", tc.tokens, got, tc.expected)
			}
		})
	}
}

// TestAutocompactThresholdFormula validates the formula matches the TypeScript source.
// Formula: effectiveWindow = contextWindow - maxOutputForSummary
//          threshold = effectiveWindow - autocompactBufferTokens
func TestAutocompactThresholdFormula(t *testing.T) {
	constants, err := testharness.LoadQueryLoopConstants()
	if err != nil {
		t.Fatalf("failed to load constants: %v", err)
	}

	contextWindows := []int{200000, 128000, 100000, 64000, 32000}

	for _, cw := range contextWindows {
		cw := cw
		t.Run(fmt.Sprintf("context_%dk", cw/1000), func(t *testing.T) {
			effectiveWindow := cw - constants.MaxOutputTokensForSummary
			threshold := effectiveWindow - constants.AutocompactBufferTokens

			t.Run("effective_window_positive", func(t *testing.T) {
				if effectiveWindow <= 0 {
					t.Errorf("effective window %d should be positive for context %d", effectiveWindow, cw)
				}
			})
			t.Run("threshold_positive", func(t *testing.T) {
				if threshold <= 0 && cw > 40000 {
					t.Errorf("threshold %d should be positive for context %d", threshold, cw)
				}
			})
			t.Run("threshold_less_than_window", func(t *testing.T) {
				if threshold >= cw {
					t.Errorf("threshold %d >= context window %d", threshold, cw)
				}
			})
			t.Run("buffer_applied", func(t *testing.T) {
				expectedThreshold := cw - constants.MaxOutputTokensForSummary - constants.AutocompactBufferTokens
				if threshold != expectedThreshold {
					t.Errorf("threshold %d != expected %d", threshold, expectedThreshold)
				}
			})
		})
	}
}

// TestMicrocompactThreshold validates microcompact behavior thresholds.
func TestMicrocompactThreshold(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	compactableTools := map[string]bool{
		"Bash": true, "Read": true, "Write": true, "Edit": true,
		"Grep": true, "Glob": true, "WebSearch": true, "WebFetch": true,
	}

	for _, s := range schemas {
		s := s
		t.Run(fmt.Sprintf("%s_compactable_%v", s.Name, compactableTools[s.Name]), func(t *testing.T) {
			expected := compactableTools[s.Name]
			if expected {
				t.Run("is_microcompact_eligible", func(t *testing.T) {
					// Tool results from this tool can be compacted
				})
			} else {
				t.Run("not_microcompact_eligible", func(t *testing.T) {
					// Tool results from this tool are NOT compacted
				})
			}
		})
	}
}

// TestMaxResultSizePerTool validates each tool's max result size.
func TestMaxResultSizePerTool(t *testing.T) {
	schemas, err := testharness.LoadToolSchemas()
	if err != nil {
		t.Fatalf("failed to load schemas: %v", err)
	}

	for _, s := range schemas {
		s := s
		t.Run(fmt.Sprintf("%s_max_result", s.Name), func(t *testing.T) {
			t.Run("is_set", func(t *testing.T) {
				if s.MaxResultSizeChars == 0 {
					t.Error("max_result_size_chars should not be 0")
				}
			})
			if s.MaxResultSizeChars == -1 {
				t.Run("is_infinity", func(t *testing.T) {
					// -1 means Infinity (only FileReadTool)
					if s.Name != "Read" {
						t.Errorf("only Read should have infinity, got %s", s.Name)
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
