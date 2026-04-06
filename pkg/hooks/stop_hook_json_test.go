package hooks

import (
	"testing"
)

// T53: Hook output JSON parsing — decision "continue"/"stop" for Stop hooks.

func TestParseHookJSONOutput_StopDecision(t *testing.T) {
	// Source: query/stopHooks.ts — decision='stop' terminates turn
	t.Run("decision_stop", func(t *testing.T) {
		out := parseHookJSONOutput(`{"decision":"stop","reason":"lint errors found"}`)
		if out == nil {
			t.Fatal("expected non-nil output")
		}
		if out.Decision != "stop" {
			t.Errorf("expected decision=stop, got %q", out.Decision)
		}
		if out.Reason != "lint errors found" {
			t.Errorf("expected reason='lint errors found', got %q", out.Reason)
		}
	})

	// Source: query/stopHooks.ts — decision='continue' resumes loop
	t.Run("decision_continue", func(t *testing.T) {
		out := parseHookJSONOutput(`{"decision":"continue"}`)
		if out == nil {
			t.Fatal("expected non-nil output")
		}
		if out.Decision != "continue" {
			t.Errorf("expected decision=continue, got %q", out.Decision)
		}
	})

	t.Run("continue_false_prevents_continuation", func(t *testing.T) {
		out := parseHookJSONOutput(`{"continue":false,"stopReason":"budget exceeded"}`)
		if out == nil {
			t.Fatal("expected non-nil output")
		}
		if out.ShouldContinue() {
			t.Error("expected ShouldContinue()=false when continue=false")
		}
		if out.StopReason != "budget exceeded" {
			t.Errorf("expected stopReason='budget exceeded', got %q", out.StopReason)
		}
	})

	t.Run("continue_default_true", func(t *testing.T) {
		out := parseHookJSONOutput(`{"decision":"continue"}`)
		if out == nil {
			t.Fatal("expected non-nil output")
		}
		if !out.ShouldContinue() {
			t.Error("expected ShouldContinue()=true by default")
		}
	})

	t.Run("system_message", func(t *testing.T) {
		out := parseHookJSONOutput(`{"systemMessage":"Warning: approaching token limit"}`)
		if out == nil {
			t.Fatal("expected non-nil output")
		}
		if out.SystemMessage != "Warning: approaching token limit" {
			t.Errorf("expected systemMessage, got %q", out.SystemMessage)
		}
	})

	t.Run("malformed_json_returns_nil", func(t *testing.T) {
		out := parseHookJSONOutput(`not json`)
		if out != nil {
			t.Error("expected nil for malformed JSON")
		}
	})

	t.Run("empty_string_returns_nil", func(t *testing.T) {
		out := parseHookJSONOutput(``)
		if out != nil {
			t.Error("expected nil for empty string")
		}
	})
}

func TestApplyJSONOutput_StopDecision(t *testing.T) {
	// T52: decision resolution for Stop hooks

	t.Run("decision_stop_prevents_continuation", func(t *testing.T) {
		result := &HookResult{
			JSONOutput: &HookJSONOutput{Decision: "stop", Reason: "hook says stop"},
		}
		applyJSONOutput(result, "my-hook")
		if !result.PreventContinuation {
			t.Error("expected PreventContinuation=true for decision=stop")
		}
		if result.StopReason != "hook says stop" {
			t.Errorf("expected StopReason='hook says stop', got %q", result.StopReason)
		}
	})

	t.Run("decision_stop_with_stopReason_field", func(t *testing.T) {
		result := &HookResult{
			JSONOutput: &HookJSONOutput{Decision: "stop", StopReason: "via stopReason"},
		}
		applyJSONOutput(result, "my-hook")
		if !result.PreventContinuation {
			t.Error("expected PreventContinuation=true")
		}
		if result.StopReason != "via stopReason" {
			t.Errorf("expected StopReason='via stopReason', got %q", result.StopReason)
		}
	})

	t.Run("decision_continue_allows_continuation", func(t *testing.T) {
		result := &HookResult{
			JSONOutput: &HookJSONOutput{Decision: "continue"},
		}
		applyJSONOutput(result, "my-hook")
		if result.PreventContinuation {
			t.Error("expected PreventContinuation=false for decision=continue")
		}
	})

	t.Run("decision_block_still_works", func(t *testing.T) {
		// Ensure backward compat: PreToolUse "block" still works
		result := &HookResult{
			JSONOutput: &HookJSONOutput{Decision: "block", Reason: "denied"},
		}
		applyJSONOutput(result, "my-hook")
		if !result.Blocked {
			t.Error("expected Blocked=true for decision=block")
		}
		if result.BlockingError == nil {
			t.Fatal("expected non-nil BlockingError")
		}
		if result.BlockingError.BlockingError != "denied" {
			t.Errorf("expected reason='denied', got %q", result.BlockingError.BlockingError)
		}
	})

	t.Run("continue_false_prevents_continuation", func(t *testing.T) {
		f := false
		result := &HookResult{
			JSONOutput: &HookJSONOutput{Continue: &f, StopReason: "explicit false"},
		}
		applyJSONOutput(result, "my-hook")
		if !result.PreventContinuation {
			t.Error("expected PreventContinuation=true when continue=false")
		}
		if result.StopReason != "explicit false" {
			t.Errorf("expected StopReason='explicit false', got %q", result.StopReason)
		}
	})

	t.Run("system_message_applied", func(t *testing.T) {
		result := &HookResult{
			JSONOutput: &HookJSONOutput{SystemMessage: "heads up"},
		}
		applyJSONOutput(result, "my-hook")
		if result.SystemMessage != "heads up" {
			t.Errorf("expected SystemMessage='heads up', got %q", result.SystemMessage)
		}
	})
}
