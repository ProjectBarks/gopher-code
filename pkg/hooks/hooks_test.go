package hooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

// Source: entrypoints/sdk/coreTypes.ts, schemas/hooks.ts, types/hooks.ts

func TestAllHookEvents(t *testing.T) {
	// Source: entrypoints/sdk/coreTypes.ts:25-53 — 27 events
	if len(AllHookEvents) != 27 {
		t.Errorf("expected 27 hook events, got %d", len(AllHookEvents))
	}

	// Verify all expected events are present
	expected := []HookEvent{
		PreToolUse, PostToolUse, PostToolUseFailure, Notification,
		UserPromptSubmit, SessionStart, SessionEnd, Stop, StopFailure,
		SubagentStart, SubagentStop, PreCompact, PostCompact,
		PermissionRequest, PermissionDenied, Setup, TeammateIdle,
		TaskCreated, TaskCompleted, Elicitation, ElicitationResult,
		ConfigChange, WorktreeCreate, WorktreeRemove, InstructionsLoaded,
		CwdChanged, FileChanged,
	}
	for _, e := range expected {
		found := false
		for _, a := range AllHookEvents {
			if a == e {
				found = true
				break
			}
		}
		if !found {
			t.Errorf("missing hook event %q", e)
		}
	}
}

func TestIsHookEvent(t *testing.T) {
	// Source: types/hooks.ts:22-24
	if !IsHookEvent("PreToolUse") {
		t.Error("PreToolUse should be a valid hook event")
	}
	if !IsHookEvent("SessionStart") {
		t.Error("SessionStart should be a valid hook event")
	}
	if !IsHookEvent("FileChanged") {
		t.Error("FileChanged should be a valid hook event")
	}
	if IsHookEvent("NotAHookEvent") {
		t.Error("NotAHookEvent should not be valid")
	}
}

func TestHookEventConstants(t *testing.T) {
	// Source: entrypoints/sdk/coreTypes.ts:25-53 — exact string values
	if string(PreToolUse) != "PreToolUse" {
		t.Error("wrong string value")
	}
	if string(PostToolUseFailure) != "PostToolUseFailure" {
		t.Error("wrong string value")
	}
	if string(UserPromptSubmit) != "UserPromptSubmit" {
		t.Error("wrong string value")
	}
	if string(SessionStart) != "SessionStart" {
		t.Error("wrong string value")
	}
	if string(SessionEnd) != "SessionEnd" {
		t.Error("wrong string value")
	}
	if string(PermissionRequest) != "PermissionRequest" {
		t.Error("wrong string value")
	}
	if string(PreCompact) != "PreCompact" {
		t.Error("wrong string value")
	}
	if string(PostCompact) != "PostCompact" {
		t.Error("wrong string value")
	}
	if string(FileChanged) != "FileChanged" {
		t.Error("wrong string value")
	}
}

func TestHookCommandTypes(t *testing.T) {
	// Source: schemas/hooks.ts:32-163
	if HookCommandTypeBash != "command" {
		t.Errorf("expected 'command', got %q", HookCommandTypeBash)
	}
	if HookCommandTypePrompt != "prompt" {
		t.Errorf("expected 'prompt', got %q", HookCommandTypePrompt)
	}
	if HookCommandTypeAgent != "agent" {
		t.Errorf("expected 'agent', got %q", HookCommandTypeAgent)
	}
	if HookCommandTypeHTTP != "http" {
		t.Errorf("expected 'http', got %q", HookCommandTypeHTTP)
	}
}

func TestHookCommandJSON(t *testing.T) {
	// Source: schemas/hooks.ts:32-66 — BashCommandHook
	t.Run("bash_command", func(t *testing.T) {
		cmd := HookCommand{
			Type:    HookCommandTypeBash,
			Command: "echo hello",
			Timeout: 30,
			Once:    true,
			If:      "Bash(git *)",
		}
		data, err := json.Marshal(cmd)
		if err != nil {
			t.Fatal(err)
		}
		var parsed HookCommand
		if err := json.Unmarshal(data, &parsed); err != nil {
			t.Fatal(err)
		}
		if parsed.Type != HookCommandTypeBash || parsed.Command != "echo hello" {
			t.Errorf("roundtrip failed: %+v", parsed)
		}
		if parsed.If != "Bash(git *)" {
			t.Errorf("if condition lost: %q", parsed.If)
		}
	})

	// Source: schemas/hooks.ts:67-95 — PromptHook
	t.Run("prompt_hook", func(t *testing.T) {
		cmd := HookCommand{
			Type:   HookCommandTypePrompt,
			Prompt: "Check if $ARGUMENTS is safe",
			Model:  "claude-sonnet-4-6",
		}
		data, _ := json.Marshal(cmd)
		var parsed HookCommand
		json.Unmarshal(data, &parsed)
		if parsed.Type != HookCommandTypePrompt || parsed.Prompt != "Check if $ARGUMENTS is safe" {
			t.Errorf("roundtrip failed: %+v", parsed)
		}
	})

	// Source: schemas/hooks.ts:97-127 — HttpHook
	t.Run("http_hook", func(t *testing.T) {
		cmd := HookCommand{
			Type:           HookCommandTypeHTTP,
			URL:            "https://example.com/webhook",
			Headers:        map[string]string{"Authorization": "Bearer $MY_TOKEN"},
			AllowedEnvVars: []string{"MY_TOKEN"},
		}
		data, _ := json.Marshal(cmd)
		var parsed HookCommand
		json.Unmarshal(data, &parsed)
		if parsed.URL != "https://example.com/webhook" {
			t.Errorf("url lost: %q", parsed.URL)
		}
		if parsed.Headers["Authorization"] != "Bearer $MY_TOKEN" {
			t.Error("headers lost")
		}
		if len(parsed.AllowedEnvVars) != 1 || parsed.AllowedEnvVars[0] != "MY_TOKEN" {
			t.Error("allowedEnvVars lost")
		}
	})

	// Source: schemas/hooks.ts:128-163 — AgentHook
	t.Run("agent_hook", func(t *testing.T) {
		cmd := HookCommand{
			Type:   HookCommandTypeAgent,
			Prompt: "Verify unit tests passed",
			Model:  "claude-haiku-4-5",
		}
		data, _ := json.Marshal(cmd)
		var parsed HookCommand
		json.Unmarshal(data, &parsed)
		if parsed.Type != HookCommandTypeAgent || parsed.Prompt != "Verify unit tests passed" {
			t.Errorf("roundtrip failed: %+v", parsed)
		}
	})
}

func TestHookMatcherJSON(t *testing.T) {
	// Source: schemas/hooks.ts:194-204
	matcher := HookMatcher{
		Matcher: "Bash",
		Hooks: []HookCommand{
			{Type: HookCommandTypeBash, Command: "echo pre-bash"},
		},
	}
	data, err := json.Marshal(matcher)
	if err != nil {
		t.Fatal(err)
	}
	var parsed HookMatcher
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if parsed.Matcher != "Bash" || len(parsed.Hooks) != 1 {
		t.Errorf("roundtrip failed: %+v", parsed)
	}
}

func TestHooksSettingsJSON(t *testing.T) {
	// Source: schemas/hooks.ts:211-213
	settings := HooksSettings{
		PreToolUse: {
			{Matcher: "Bash", Hooks: []HookCommand{{Type: HookCommandTypeBash, Command: "echo check"}}},
		},
		SessionStart: {
			{Hooks: []HookCommand{{Type: HookCommandTypeBash, Command: "echo starting"}}},
		},
	}
	data, err := json.Marshal(settings)
	if err != nil {
		t.Fatal(err)
	}
	var parsed HooksSettings
	if err := json.Unmarshal(data, &parsed); err != nil {
		t.Fatal(err)
	}
	if len(parsed[PreToolUse]) != 1 {
		t.Error("PreToolUse matchers lost")
	}
	if len(parsed[SessionStart]) != 1 {
		t.Error("SessionStart matchers lost")
	}
}

func TestBaseHookInputJSON(t *testing.T) {
	// Source: entrypoints/sdk/coreSchemas.ts:387-411
	input := BaseHookInput{
		SessionID:      "session-123",
		TranscriptPath: "/tmp/transcript.jsonl",
		Cwd:            "/home/user/project",
		PermissionMode: "default",
		HookEventName:  "PreToolUse",
	}
	data, err := json.Marshal(input)
	if err != nil {
		t.Fatal(err)
	}
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)

	if parsed["session_id"] != "session-123" {
		t.Error("session_id wrong")
	}
	if parsed["hook_event_name"] != "PreToolUse" {
		t.Error("hook_event_name wrong")
	}
}

func TestHookInputPreToolUse(t *testing.T) {
	// Source: entrypoints/sdk/coreSchemas.ts:414-423
	input := HookInput{
		BaseHookInput: BaseHookInput{
			SessionID:     "s1",
			TranscriptPath: "/tmp/t.jsonl",
			Cwd:           "/tmp",
			HookEventName: "PreToolUse",
		},
		ToolName:  "Bash",
		ToolInput: json.RawMessage(`{"command":"ls"}`),
		ToolUseID: "tu-1",
	}
	data, _ := json.Marshal(input)
	var parsed map[string]interface{}
	json.Unmarshal(data, &parsed)
	if parsed["tool_name"] != "Bash" {
		t.Error("tool_name missing")
	}
	if parsed["tool_use_id"] != "tu-1" {
		t.Error("tool_use_id missing")
	}
}

func TestHookJSONOutput(t *testing.T) {
	// Source: types/hooks.ts:169-176

	t.Run("sync_continue_true", func(t *testing.T) {
		out := HookJSONOutput{}
		if !out.ShouldContinue() {
			t.Error("default should be continue=true")
		}
		if !out.IsSyncResponse() {
			t.Error("should be sync by default")
		}
	})

	t.Run("sync_continue_false", func(t *testing.T) {
		f := false
		out := HookJSONOutput{Continue: &f, StopReason: "hook stopped"}
		if out.ShouldContinue() {
			t.Error("should be continue=false")
		}
		if out.StopReason != "hook stopped" {
			t.Error("stopReason wrong")
		}
	})

	t.Run("async_response", func(t *testing.T) {
		out := HookJSONOutput{IsAsync: true, AsyncTimeout: 15000}
		if out.IsSyncResponse() {
			t.Error("should be async")
		}
	})

	t.Run("decision_block", func(t *testing.T) {
		// Source: types/hooks.ts:64
		out := HookJSONOutput{Decision: "block", Reason: "dangerous command"}
		if out.Decision != "block" {
			t.Error("decision should be block")
		}
	})

	t.Run("parse_from_json", func(t *testing.T) {
		data := `{"continue":false,"stopReason":"halted","decision":"block","reason":"unsafe"}`
		var out HookJSONOutput
		if err := json.Unmarshal([]byte(data), &out); err != nil {
			t.Fatal(err)
		}
		if out.ShouldContinue() {
			t.Error("should not continue")
		}
		if out.StopReason != "halted" {
			t.Errorf("stopReason = %q", out.StopReason)
		}
		if out.Decision != "block" {
			t.Errorf("decision = %q", out.Decision)
		}
	})

	t.Run("system_message", func(t *testing.T) {
		// Source: types/hooks.ts:68-70
		out := HookJSONOutput{SystemMessage: "Warning: risky operation"}
		if out.SystemMessage != "Warning: risky operation" {
			t.Error("systemMessage wrong")
		}
	})
}

func TestHookOutcomeConstants(t *testing.T) {
	// Source: types/hooks.ts:264
	if OutcomeSuccess != "success" {
		t.Error("wrong")
	}
	if OutcomeBlocking != "blocking" {
		t.Error("wrong")
	}
	if OutcomeNonBlockingError != "non_blocking_error" {
		t.Error("wrong")
	}
	if OutcomeCancelled != "cancelled" {
		t.Error("wrong")
	}
}

func TestPreToolUseHookAllows(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: "exit 0"},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("expected hook to allow")
	}
	if result.Outcome != OutcomeSuccess {
		t.Errorf("expected success outcome, got %s", result.Outcome)
	}
}

func TestPreToolUseHookBlocks(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: "echo 'not allowed' >&2; exit 1"},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("expected hook to block")
	}
	if result.Message != "not allowed" {
		t.Errorf("expected message 'not allowed', got %q", result.Message)
	}
}

func TestExitCode2BlockingError(t *testing.T) {
	// Source: utils/hooks/hooksConfigManager.ts — exit 2 = blocking error
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: "echo 'BLOCKED' >&2; exit 2"},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("exit 2 should block")
	}
	if result.Outcome != OutcomeBlocking {
		t.Errorf("expected blocking outcome, got %s", result.Outcome)
	}
	if result.BlockingError == nil {
		t.Fatal("expected blocking error")
	}
	if result.BlockingError.BlockingError != "BLOCKED" {
		t.Errorf("expected 'BLOCKED', got %q", result.BlockingError.BlockingError)
	}
	if !result.PreventContinuation {
		t.Error("exit 2 should prevent continuation")
	}
}

func TestExitCode3NonBlockingError(t *testing.T) {
	// Source: utils/hooks/hooksConfigManager.ts — exit != 0 and != 2 = non-blocking
	runner := NewHookRunner([]HookConfig{
		{Type: PostToolUse, Command: "echo 'warning' >&2; exit 3"},
	})

	result, err := runner.Run(context.Background(), PostToolUse, "Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	// PostToolUse non-zero should be non-blocking (unlike PreToolUse)
	if result.Outcome != OutcomeNonBlockingError {
		t.Errorf("expected non_blocking_error, got %s", result.Outcome)
	}
}

func TestPostToolUseHookRuns(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PostToolUse, Command: "echo $TOOL_NAME $HOOK_EVENT_NAME"},
	})

	result, err := runner.Run(context.Background(), PostToolUse, "Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("post-tool hooks should never block on exit 0")
	}
	if result.Outcome != OutcomeSuccess {
		t.Errorf("expected success, got %s", result.Outcome)
	}
}

func TestHookMatcherFilters(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Matcher: "Bash", Command: "exit 2"},
	})

	// Should NOT match "Read" tool
	result, err := runner.Run(context.Background(), PreToolUse, "Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("hook should not have matched Read tool")
	}

	// Should match "Bash" tool
	result, err = runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("hook should have matched Bash tool and blocked")
	}
}

func TestHookMatcherGlob(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Matcher: "File*", Command: "exit 2"},
	})

	result, _ := runner.Run(context.Background(), PreToolUse, "FileRead", json.RawMessage(`{}`))
	if !result.Blocked {
		t.Error("expected glob File* to match FileRead")
	}

	result, _ = runner.Run(context.Background(), PreToolUse, "FileWrite", json.RawMessage(`{}`))
	if !result.Blocked {
		t.Error("expected glob File* to match FileWrite")
	}

	result, _ = runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if result.Blocked {
		t.Error("expected glob File* to NOT match Bash")
	}
}

func TestHookTimeout(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: "sleep 10", Timeout: 1},
	})

	start := time.Now()
	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("hook should have timed out after ~1s, took %v", elapsed)
	}
	if result != nil && result.Outcome != OutcomeCancelled {
		t.Errorf("expected cancelled outcome, got %s", result.Outcome)
	}
}

func TestRunForOrchestrator(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: "echo 'denied' >&2; exit 1"},
	})

	blocked, msg, err := runner.RunForOrchestrator(context.Background(), "PreToolUse", "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !blocked {
		t.Error("expected blocked")
	}
	if msg != "denied" {
		t.Errorf("expected message 'denied', got %q", msg)
	}
}

func TestNoMatchingHooksReturnsEmpty(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{Type: PostToolUse, Command: "echo hello"},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("should not block when no hooks match")
	}
	if result.Outcome != OutcomeSuccess {
		t.Errorf("expected success, got %s", result.Outcome)
	}
}

func TestParseHookJSONOutput(t *testing.T) {
	t.Run("valid_json", func(t *testing.T) {
		out := parseHookJSONOutput(`{"continue":false,"stopReason":"done"}`)
		if out == nil {
			t.Fatal("expected parsed output")
		}
		if out.ShouldContinue() {
			t.Error("should be false")
		}
		if out.StopReason != "done" {
			t.Errorf("stopReason = %q", out.StopReason)
		}
	})

	t.Run("non_json_returns_nil", func(t *testing.T) {
		out := parseHookJSONOutput("just some text")
		if out != nil {
			t.Error("non-JSON should return nil")
		}
	})

	t.Run("empty_returns_nil", func(t *testing.T) {
		out := parseHookJSONOutput("")
		if out != nil {
			t.Error("empty should return nil")
		}
	})

	t.Run("async_output", func(t *testing.T) {
		out := parseHookJSONOutput(`{"async":true,"asyncTimeout":15000}`)
		if out == nil {
			t.Fatal("expected parsed output")
		}
		if out.IsSyncResponse() {
			t.Error("should be async")
		}
		if out.AsyncTimeout != 15000 {
			t.Errorf("asyncTimeout = %d", out.AsyncTimeout)
		}
	})
}

func TestHookJSONOutputBlockDecision(t *testing.T) {
	// Source: types/hooks.ts:50-166 — decision field
	runner := NewHookRunner([]HookConfig{
		{Type: PreToolUse, Command: `echo '{"decision":"block","reason":"unsafe command"}'`},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("decision=block should block")
	}
	if result.BlockingError == nil {
		t.Fatal("expected blocking error from JSON output")
	}
	if result.BlockingError.BlockingError != "unsafe command" {
		t.Errorf("reason = %q", result.BlockingError.BlockingError)
	}
}

func TestHookJSONOutputPreventContinuation(t *testing.T) {
	// Source: types/hooks.ts:52-53 — continue:false prevents continuation
	runner := NewHookRunner([]HookConfig{
		{Type: Stop, Command: `echo '{"continue":false,"stopReason":"hook says stop"}'`},
	})

	result, err := runner.Run(context.Background(), Stop, "", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.PreventContinuation {
		t.Error("continue:false should prevent continuation")
	}
	if result.StopReason != "hook says stop" {
		t.Errorf("stopReason = %q", result.StopReason)
	}
}

func TestNewLifecycleEventsWork(t *testing.T) {
	// Test that new lifecycle events can be used with HookRunner
	events := []HookEvent{
		SessionStart, SessionEnd, UserPromptSubmit, PreCompact,
		PostCompact, PostToolUseFailure, PermissionRequest,
	}

	for _, event := range events {
		t.Run(string(event), func(t *testing.T) {
			runner := NewHookRunner([]HookConfig{
				{Type: event, Command: "exit 0"},
			})
			result, err := runner.Run(context.Background(), event, "", json.RawMessage(`{}`))
			if err != nil {
				t.Fatalf("error running %s hook: %v", event, err)
			}
			if result.Outcome != OutcomeSuccess {
				t.Errorf("expected success for %s, got %s", event, result.Outcome)
			}
		})
	}
}

func TestHookEnvVariables(t *testing.T) {
	// Verify HOOK_EVENT_NAME env var is set
	runner := NewHookRunner([]HookConfig{
		{Type: SessionStart, Command: "echo $HOOK_EVENT_NAME"},
	})

	result, err := runner.Run(context.Background(), SessionStart, "", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Outcome != OutcomeSuccess {
		t.Errorf("expected success, got %s", result.Outcome)
	}
}
