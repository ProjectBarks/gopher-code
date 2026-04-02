package hooks

import (
	"context"
	"encoding/json"
	"testing"
	"time"
)

func TestPreToolUseHookAllows(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PreToolUse,
			Command: "exit 0",
		},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{"command":"ls"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("expected hook to allow, but it blocked")
	}
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestPreToolUseHookBlocks(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PreToolUse,
			Command: "echo 'not allowed' >&2; exit 1",
		},
	})

	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{"command":"rm -rf /"}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !result.Blocked {
		t.Error("expected hook to block, but it allowed")
	}
	if result.ExitCode != 1 {
		t.Errorf("expected exit code 1, got %d", result.ExitCode)
	}
	if result.Message != "not allowed" {
		t.Errorf("expected message 'not allowed', got %q", result.Message)
	}
}

func TestPostToolUseHookRuns(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PostToolUse,
			Command: "echo $TOOL_NAME $HOOK_TYPE",
		},
	})

	result, err := runner.Run(context.Background(), PostToolUse, "Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("post-tool hooks should never block")
	}
	// PostToolUse with exit 0 should return an empty result since no hook blocked
	if result.ExitCode != 0 {
		t.Errorf("expected exit code 0, got %d", result.ExitCode)
	}
}

func TestHookMatcherFilters(t *testing.T) {
	commandRan := false
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PreToolUse,
			Matcher: "Bash",
			Command: "exit 1", // would block if it matched
		},
	})

	// Should NOT match "Read" tool
	result, err := runner.Run(context.Background(), PreToolUse, "Read", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("hook should not have matched Read tool")
		commandRan = true
	}
	_ = commandRan

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
		{
			Type:    PreToolUse,
			Matcher: "File*",
			Command: "exit 1",
		},
	})

	// Should match FileRead, FileWrite, etc.
	result, _ := runner.Run(context.Background(), PreToolUse, "FileRead", json.RawMessage(`{}`))
	if !result.Blocked {
		t.Error("expected glob File* to match FileRead")
	}

	result, _ = runner.Run(context.Background(), PreToolUse, "FileWrite", json.RawMessage(`{}`))
	if !result.Blocked {
		t.Error("expected glob File* to match FileWrite")
	}

	// Should NOT match Bash
	result, _ = runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if result.Blocked {
		t.Error("expected glob File* to NOT match Bash")
	}
}

func TestHookTimeout(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PreToolUse,
			Command: "sleep 10",
			Timeout: 1,
		},
	})

	start := time.Now()
	_, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	elapsed := time.Since(start)

	if err == nil {
		t.Error("expected timeout error")
	}
	if elapsed > 3*time.Second {
		t.Errorf("hook should have timed out after ~1s, took %v", elapsed)
	}
}

func TestRunForOrchestrator(t *testing.T) {
	runner := NewHookRunner([]HookConfig{
		{
			Type:    PreToolUse,
			Command: "echo 'denied' >&2; exit 1",
		},
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
		{
			Type:    PostToolUse,
			Command: "echo hello",
		},
	})

	// Ask for PreToolUse, but only PostToolUse hooks exist
	result, err := runner.Run(context.Background(), PreToolUse, "Bash", json.RawMessage(`{}`))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if result.Blocked {
		t.Error("should not block when no hooks match")
	}
}
