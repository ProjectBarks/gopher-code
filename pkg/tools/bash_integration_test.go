package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

// TestBashTool_OrchestratorSecurityIntegration verifies that when the BashTool is
// executed through the orchestrator pipeline, tool-specific permission checks
// (security validation, injection detection) are enforced.
// This tests the full path: Orchestrator -> CheckToolPermissions -> BashTool.CheckPermissions
func TestBashTool_OrchestratorSecurityIntegration(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)
	orch := tools.NewOrchestrator(registry)

	tc := &tools.ToolContext{
		CWD: t.TempDir(),
	}

	t.Run("rejects_command_injection_via_orchestrator", func(t *testing.T) {
		call := tools.ToolCall{
			ID:    "test-inject-1",
			Name:  "Bash",
			Input: json.RawMessage(`{"command": "echo $(whoami)"}`),
		}
		results := orch.ExecuteBatch(context.Background(), []tools.ToolCall{call}, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if !r.Output.IsError {
			t.Fatalf("expected error for command injection, got success: %s", r.Output.Content)
		}
		if !strings.Contains(r.Output.Content, "permission denied") {
			t.Errorf("expected 'permission denied' in error, got: %s", r.Output.Content)
		}
	})

	t.Run("rejects_ld_preload_hijack_via_orchestrator", func(t *testing.T) {
		call := tools.ToolCall{
			ID:    "test-inject-2",
			Name:  "Bash",
			Input: json.RawMessage(`{"command": "LD_PRELOAD=evil.so ls"}`),
		}
		results := orch.ExecuteBatch(context.Background(), []tools.ToolCall{call}, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if !r.Output.IsError {
			t.Fatalf("expected error for binary hijack, got success: %s", r.Output.Content)
		}
		if !strings.Contains(r.Output.Content, "permission denied") {
			t.Errorf("expected 'permission denied' in error, got: %s", r.Output.Content)
		}
	})

	t.Run("allows_safe_command_via_orchestrator", func(t *testing.T) {
		call := tools.ToolCall{
			ID:    "test-safe-1",
			Name:  "Bash",
			Input: json.RawMessage(`{"command": "echo integration-test-ok"}`),
		}
		results := orch.ExecuteBatch(context.Background(), []tools.ToolCall{call}, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Output.IsError {
			t.Fatalf("expected success for safe command, got error: %s", r.Output.Content)
		}
		if !strings.Contains(r.Output.Content, "integration-test-ok") {
			t.Errorf("expected output to contain marker, got: %s", r.Output.Content)
		}
	})
}

// TestBashTool_OrchestratorDestructiveWarning verifies that destructive commands
// trigger an "ask" behavior through the orchestrator pipeline.
func TestBashTool_OrchestratorDestructiveWarning(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)

	tc := &tools.ToolContext{
		CWD: t.TempDir(),
	}

	// Verify destructive check returns true through CheckDestructive
	tool := registry.Get("Bash")
	if tool == nil {
		t.Fatal("BashTool not registered")
	}

	input, _ := json.Marshal(map[string]string{"command": "git reset --hard HEAD"})
	if !tools.CheckDestructive(tool, input) {
		t.Error("expected CheckDestructive to return true for 'git reset --hard'")
	}

	// Verify CheckToolPermissions returns "ask" for destructive commands
	result := tools.CheckToolPermissions(tool, context.Background(), tc, input)
	if result == nil {
		t.Fatal("expected non-nil permission result for destructive command")
	}
	if result.Behavior != "ask" {
		t.Errorf("expected 'ask' behavior for destructive command, got %q", result.Behavior)
	}
}

// TestBashTool_SandboxAwareness verifies that the BashTool is aware of sandbox
// availability and the DangerouslyDisableSandbox flag.
func TestBashTool_SandboxAwareness(t *testing.T) {
	// Verify sandbox detection works (doesn't panic, returns a valid type)
	sandboxType := tools.DetectSandbox()
	t.Logf("detected sandbox type: %s", sandboxType)

	if sandboxType != tools.SandboxNone && sandboxType != tools.SandboxSeatbelt && sandboxType != tools.SandboxBubblewrap {
		t.Errorf("unexpected sandbox type: %s", sandboxType)
	}

	// IsSandboxAvailable should be consistent with DetectSandbox
	available := tools.IsSandboxAvailable()
	if available != (sandboxType != tools.SandboxNone) {
		t.Errorf("IsSandboxAvailable() = %v but DetectSandbox() = %s", available, sandboxType)
	}

	// Verify BashTool executes successfully with dangerouslyDisableSandbox=true
	tool := &tools.BashTool{}
	tc := &tools.ToolContext{CWD: t.TempDir(), SandboxEnabled: true}
	input := json.RawMessage(`{"command": "echo sandbox-disabled-ok", "dangerouslyDisableSandbox": true}`)
	out, err := tool.Execute(context.Background(), tc, input)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError {
		t.Fatalf("unexpected tool error: %s", out.Content)
	}
	if !strings.Contains(out.Content, "sandbox-disabled-ok") {
		t.Errorf("expected output with sandbox disabled, got: %s", out.Content)
	}

	// Verify BashTool executes successfully with sandbox disabled (default)
	tc2 := &tools.ToolContext{CWD: t.TempDir()}
	input2 := json.RawMessage(`{"command": "echo sandbox-default-ok"}`)
	out2, err := tool.Execute(context.Background(), tc2, input2)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out2.IsError {
		t.Fatalf("unexpected tool error: %s", out2.Content)
	}
	if !strings.Contains(out2.Content, "sandbox-default-ok") {
		t.Errorf("expected output with default sandbox, got: %s", out2.Content)
	}

	// Verify SandboxConfig is built correctly when sandbox is enabled
	// (WrapCommand is callable and returns valid output)
	if available {
		cfg := tools.SandboxConfig{
			AllowNetwork: true,
			AllowedPaths: []string{t.TempDir()},
			WorkingDir:   t.TempDir(),
		}
		binPath, binArgs := tools.WrapCommand("echo test", cfg)
		if binPath == "" {
			t.Error("WrapCommand returned empty binary path")
		}
		if len(binArgs) == 0 {
			t.Error("WrapCommand returned empty args")
		}
		t.Logf("WrapCommand: %s %v", binPath, binArgs)
	}
}

// TestBashTool_PlanModeViaOrchestrator verifies plan mode enforcement through
// the full orchestrator pipeline.
func TestBashTool_PlanModeViaOrchestrator(t *testing.T) {
	registry := tools.NewRegistry()
	tools.RegisterDefaults(registry)
	orch := tools.NewOrchestrator(registry)

	tc := &tools.ToolContext{
		CWD:      t.TempDir(),
		PlanMode: true,
	}

	t.Run("rejects_write_in_plan_mode", func(t *testing.T) {
		call := tools.ToolCall{
			ID:    "plan-write-1",
			Name:  "Bash",
			Input: json.RawMessage(`{"command": "rm -rf /tmp/test123"}`),
		}
		results := orch.ExecuteBatch(context.Background(), []tools.ToolCall{call}, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if !r.Output.IsError {
			t.Errorf("expected error for write command in plan mode, got success: %s", r.Output.Content)
		}
	})

	t.Run("allows_read_in_plan_mode", func(t *testing.T) {
		call := tools.ToolCall{
			ID:    "plan-read-1",
			Name:  "Bash",
			Input: json.RawMessage(`{"command": "echo plan-read-ok"}`),
		}
		results := orch.ExecuteBatch(context.Background(), []tools.ToolCall{call}, tc)
		if len(results) != 1 {
			t.Fatalf("expected 1 result, got %d", len(results))
		}
		r := results[0]
		if r.Output.IsError {
			t.Errorf("expected success for read command in plan mode, got error: %s", r.Output.Content)
		}
		if !strings.Contains(r.Output.Content, "plan-read-ok") {
			t.Errorf("expected plan-read-ok in output, got: %s", r.Output.Content)
		}
	})
}
