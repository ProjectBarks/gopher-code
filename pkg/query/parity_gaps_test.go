package query_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"runtime"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/internal/testharness"
	"github.com/projectbarks/gopher-code/pkg/compact"
	"github.com/projectbarks/gopher-code/pkg/message"
	"github.com/projectbarks/gopher-code/pkg/permissions"
	"github.com/projectbarks/gopher-code/pkg/provider"
	"github.com/projectbarks/gopher-code/pkg/query"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/tools"
)

func gapsPath() string {
	_, filename, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(filename), "..", "..", "testdata", "parity_gaps.json")
}

type ParityGaps struct {
	ContinuationMessage struct {
		TSMessage string `json:"ts_message"`
		GoMessage string `json:"go_message"`
	} `json:"continuation_message"`
	RetryConstants struct {
		TSDefaultMaxRetries int `json:"ts_default_max_retries"`
		TSMax529Retries     int `json:"ts_max_529_retries"`
		TSBaseDelayMs       int `json:"ts_base_delay_ms"`
		TSFloorOutputTokens int `json:"ts_floor_output_tokens"`
		GoMaxRetries        int `json:"go_max_retries"`
	} `json:"retry_constants"`
	Error529 struct {
		TSHandles529     bool   `json:"ts_handles_529"`
		TSDetection      string `json:"ts_529_detection"`
		TSMaxRetries     int    `json:"ts_529_max_retries"`
		TSForegroundOnly bool   `json:"ts_529_foreground_only"`
		GoHandles529     bool   `json:"go_handles_529"`
	} `json:"error_529_overloaded"`
	ContextTooLong struct {
		TSKeywords []string `json:"ts_keywords"`
		GoKeywords []string `json:"go_keywords"`
	} `json:"context_too_long_detection"`
	MissingToolResult struct {
		TSSynthesizes bool   `json:"ts_synthesizes"`
		TSPlaceholder string `json:"ts_placeholder"`
		GoSynthesizes bool   `json:"go_synthesizes"`
	} `json:"missing_tool_result_synthesis"`
}

func loadGaps(t *testing.T) *ParityGaps {
	t.Helper()
	data, err := os.ReadFile(gapsPath())
	if err != nil {
		t.Fatalf("failed to load parity gaps: %v", err)
	}
	var gaps ParityGaps
	if err := json.Unmarshal(data, &gaps); err != nil {
		t.Fatalf("failed to parse: %v", err)
	}
	return &gaps
}

// TestContinuationMessageMatchesTS validates the auto-continue message matches TS source.
// Source: query.ts:1224-1228
// The TS source uses a specific directive: "Output token limit hit. Resume directly..."
// The Go code currently uses: "Please continue from where you left off."
// This test SHOULD FAIL until Go is updated to match TS.
func TestContinuationMessageMatchesTS(t *testing.T) {
	gaps := loadGaps(t)
	expectedMessage := gaps.ContinuationMessage.TSMessage

	// Set up: provider returns max_tokens stop with no tools, then EndTurn
	stopMax := provider.StopReasonMaxTokens
	stopEnd := provider.StopReasonEndTurn
	sp := testharness.NewScriptedProvider(
		testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "partial response"}},
			{Event: &provider.StreamEvent{Type: provider.EventMessageDone, Response: &provider.ModelResponse{
				ID: "r1", Content: []provider.ResponseContent{{Type: "text", Text: "partial"}},
				StopReason: &stopMax, Usage: provider.Usage{},
			}}},
		}},
		testharness.TurnScript{Events: []provider.StreamResult{
			{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "done"}},
			{Event: &provider.StreamEvent{Type: provider.EventMessageDone, Response: &provider.ModelResponse{
				ID: "r2", Content: []provider.ResponseContent{{Type: "text", Text: "done"}},
				StopReason: &stopEnd, Usage: provider.Usage{},
			}}},
		}},
	)

	registry := tools.NewRegistry()
	orch := tools.NewOrchestrator(registry)
	sess := session.New(session.SessionConfig{
		Model: "test", MaxTurns: 100,
		TokenBudget:    compact.DefaultBudget(),
		PermissionMode: permissions.AutoApprove,
	}, os.TempDir())
	sess.PushMessage(message.UserMessage("hello"))

	err := query.Query(context.Background(), sess, sp, registry, orch, nil)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Find the continuation message injected between the two assistant turns
	var continuationText string
	for _, msg := range sess.Messages {
		if msg.Role == message.RoleUser {
			for _, block := range msg.Content {
				if block.Type == message.ContentText && block.Text != "hello" {
					continuationText = block.Text
				}
			}
		}
	}

	t.Run("continuation_message_exists", func(t *testing.T) {
		if continuationText == "" {
			t.Fatal("no continuation message found")
		}
	})

	t.Run("matches_ts_source_text", func(t *testing.T) {
		// TS source: query.ts:1226-1227
		if continuationText != expectedMessage {
			t.Errorf("continuation message does not match TS source.\ngot:  %q\nwant: %q", continuationText, expectedMessage)
		}
	})

	t.Run("starts_with_output_token_limit", func(t *testing.T) {
		if !strings.HasPrefix(continuationText, "Output token limit hit") {
			t.Errorf("should start with 'Output token limit hit', got: %q", continuationText)
		}
	})

	t.Run("contains_no_apology", func(t *testing.T) {
		if !strings.Contains(continuationText, "no apology") {
			t.Errorf("should contain 'no apology', got: %q", continuationText)
		}
	})
}

// TestError529OverloadedHandling validates 529 overloaded error handling.
// Source: withRetry.ts:610-622 — TS handles 529 with specific retry policy
// Go does NOT handle 529 at all — this test documents the gap.
func TestError529OverloadedHandling(t *testing.T) {
	gaps := loadGaps(t)

	t.Run("ts_handles_529", func(t *testing.T) {
		if !gaps.Error529.TSHandles529 {
			t.Error("TS source handles 529 overloaded errors")
		}
	})
	t.Run("ts_529_max_retries_3", func(t *testing.T) {
		if gaps.Error529.TSMaxRetries != 3 {
			t.Errorf("expected 3, got %d", gaps.Error529.TSMaxRetries)
		}
	})
	t.Run("ts_529_foreground_only", func(t *testing.T) {
		if !gaps.Error529.TSForegroundOnly {
			t.Error("529 retries should only happen for foreground queries")
		}
	})

	// Behavioral test: 529 should be retried, not fail immediately
	t.Run("529_should_be_retried", func(t *testing.T) {
		stopEnd := provider.StopReasonEndTurn
		sp := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("529 overloaded")),
			testharness.TurnScript{Events: []provider.StreamResult{
				{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "ok"}},
				{Event: &provider.StreamEvent{Type: provider.EventMessageDone, Response: &provider.ModelResponse{
					ID: "r1", Content: []provider.ResponseContent{{Type: "text", Text: "ok"}},
					StopReason: &stopEnd, Usage: provider.Usage{},
				}}},
			}},
		)

		registry := tools.NewRegistry()
		orch := tools.NewOrchestrator(registry)
		sess := session.New(session.SessionConfig{
			Model: "test", MaxTurns: 100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		}, os.TempDir())
		sess.PushMessage(message.UserMessage("hello"))

		err := query.Query(context.Background(), sess, sp, registry, orch, nil)
		if err != nil {
			t.Errorf("529 should be retried and succeed, got error: %v", err)
		}
		if len(sp.CapturedRequests) < 2 {
			t.Errorf("expected at least 2 requests (1 retry), got %d", len(sp.CapturedRequests))
		}
	})
}

// TestRetryConstantsMatchTS validates retry behavior constants.
// Source: withRetry.ts:52-55
func TestRetryConstantsMatchTS(t *testing.T) {
	gaps := loadGaps(t)

	t.Run("ts_default_max_retries_10", func(t *testing.T) {
		if gaps.RetryConstants.TSDefaultMaxRetries != 10 {
			t.Errorf("expected 10, got %d", gaps.RetryConstants.TSDefaultMaxRetries)
		}
	})
	t.Run("ts_base_delay_500ms", func(t *testing.T) {
		if gaps.RetryConstants.TSBaseDelayMs != 500 {
			t.Errorf("expected 500, got %d", gaps.RetryConstants.TSBaseDelayMs)
		}
	})
	t.Run("ts_floor_output_tokens_3000", func(t *testing.T) {
		if gaps.RetryConstants.TSFloorOutputTokens != 3000 {
			t.Errorf("expected 3000, got %d", gaps.RetryConstants.TSFloorOutputTokens)
		}
	})
}

// TestContextTooLongKeywords validates all keywords that trigger context_too_long handling.
// Source: query.ts error handling
// TS checks for both "context_too_long" and "prompt is too long"
// Go only checks "context_too_long"
func TestContextTooLongKeywords(t *testing.T) {
	gaps := loadGaps(t)

	t.Run("ts_has_context_too_long", func(t *testing.T) {
		found := false
		for _, k := range gaps.ContextTooLong.TSKeywords {
			if k == "context_too_long" {
				found = true
			}
		}
		if !found {
			t.Error("TS should check for 'context_too_long'")
		}
	})

	t.Run("ts_has_prompt_is_too_long", func(t *testing.T) {
		found := false
		for _, k := range gaps.ContextTooLong.TSKeywords {
			if k == "prompt is too long" {
				found = true
			}
		}
		if !found {
			t.Error("TS should check for 'prompt is too long'")
		}
	})

	// Behavioral test: "prompt is too long" should trigger compact+retry
	t.Run("prompt_is_too_long_triggers_compact", func(t *testing.T) {
		stopEnd := provider.StopReasonEndTurn
		sp := testharness.NewScriptedProvider(
			testharness.MakeErrorTurn(fmt.Errorf("prompt is too long: 210000 tokens > 200000 maximum")),
			testharness.TurnScript{Events: []provider.StreamResult{
				{Event: &provider.StreamEvent{Type: provider.EventTextDelta, Text: "ok"}},
				{Event: &provider.StreamEvent{Type: provider.EventMessageDone, Response: &provider.ModelResponse{
					ID: "r1", Content: []provider.ResponseContent{{Type: "text", Text: "ok"}},
					StopReason: &stopEnd, Usage: provider.Usage{},
				}}},
			}},
		)

		registry := tools.NewRegistry()
		orch := tools.NewOrchestrator(registry)
		sess := session.New(session.SessionConfig{
			Model: "test", MaxTurns: 100,
			TokenBudget:    compact.DefaultBudget(),
			PermissionMode: permissions.AutoApprove,
		}, os.TempDir())
		sess.PushMessage(message.UserMessage("hello"))
		// Add padding messages so compact has something to remove
		for i := 0; i < 10; i++ {
			sess.PushMessage(message.Message{Role: message.RoleAssistant, Content: []message.ContentBlock{{Type: message.ContentText, Text: fmt.Sprintf("response %d", i)}}})
			sess.PushMessage(message.UserMessage(fmt.Sprintf("followup %d", i)))
		}

		err := query.Query(context.Background(), sess, sp, registry, orch, nil)
		if err != nil {
			t.Errorf("'prompt is too long' should trigger compact+retry, got error: %v", err)
		}
	})
}

// TestMissingToolResultSynthesis validates that missing tool results are synthesized.
// Source: query.ts:138-143, messages.ts:245
// TS synthesizes placeholder tool_result blocks for any tool_use without a result.
// Go does NOT synthesize — this documents the gap.
func TestMissingToolResultSynthesis(t *testing.T) {
	gaps := loadGaps(t)

	t.Run("ts_synthesizes_missing_results", func(t *testing.T) {
		if !gaps.MissingToolResult.TSSynthesizes {
			t.Error("TS must synthesize missing tool results")
		}
	})
	t.Run("placeholder_text", func(t *testing.T) {
		expected := "[Tool result missing due to internal error]"
		if gaps.MissingToolResult.TSPlaceholder != expected {
			t.Errorf("expected %q, got %q", expected, gaps.MissingToolResult.TSPlaceholder)
		}
	})
}

// TestParityGapsSummary counts known parity gaps.
func TestParityGapsSummary(t *testing.T) {
	data, err := os.ReadFile(gapsPath())
	if err != nil {
		t.Fatalf("failed to load: %v", err)
	}
	var raw map[string]interface{}
	json.Unmarshal(data, &raw)

	gapCount := 0
	for k, v := range raw {
		if strings.HasPrefix(k, "_") {
			continue
		}
		m, ok := v.(map[string]interface{})
		if !ok {
			continue
		}
		if gap, ok := m["gap"].(string); ok && gap != "" {
			if !strings.Contains(gap, "no gap") {
				gapCount++
			}
		}
	}

	t.Run("documented_gaps", func(t *testing.T) {
		t.Logf("Found %d documented parity gaps between Go and TS", gapCount)
		if gapCount == 0 {
			t.Error("should have documented parity gaps")
		}
	})
}
