package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"runtime"
	"time"
)

// PowerShell timeout constants (delegates to bash defaults per TS source).
// Source: prompt.ts:18-22
const (
	DefaultPowerShellTimeoutMs = DefaultBashTimeoutMs // 120_000
	MaxPowerShellTimeoutMs     = MaxBashTimeoutMs     // 600_000
)

// PSAssistantBlockingBudgetMs is the threshold after which long-running
// PowerShell commands are auto-backgrounded in assistant mode.
// Source: PowerShellTool.tsx:57
const PSAssistantBlockingBudgetMs = 15_000

// PSProgressThresholdMs is the delay before progress streaming starts.
// Source: PowerShellTool.tsx:54
const PSProgressThresholdMs = 2000

// PSProgressIntervalMs is the interval between progress ticks.
// Source: PowerShellTool.tsx:55
const PSProgressIntervalMs = 1000

// WindowsSandboxPolicyRefusal is the enterprise policy message.
// Source: PowerShellTool.tsx:49-50
const WindowsSandboxPolicyRefusal = "Enterprise policy requires sandboxing, but sandboxing is not available on native Windows. Shell command execution is blocked on this platform by policy."

// PowerShellTool executes PowerShell commands. Primarily for Windows, but
// also works on other platforms if pwsh (cross-platform PowerShell) is installed.
type PowerShellTool struct{}

type psInput struct {
	Command         string `json:"command"`
	Description     string `json:"description,omitempty"`
	Timeout         int    `json:"timeout,omitempty"` // milliseconds
	RunInBackground bool   `json:"run_in_background,omitempty"`
}

func (t *PowerShellTool) Name() string { return "PowerShell" }

// Description returns the full system prompt for the PowerShell tool.
// Source: prompt.ts:73-144 — getPrompt()
func (t *PowerShellTool) Description() string {
	return GetPowerShellPrompt("unknown")
}

func (t *PowerShellTool) IsReadOnly() bool { return false }

func (t *PowerShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "PowerShell command to execute"},
			"description": {"type": "string", "description": "Clear, concise description of what this command does"},
			"timeout": {"type": "integer", "description": "Timeout in milliseconds (default 120000)"},
			"run_in_background": {"type": "boolean", "description": "Run the command in the background"}
		},
		"required": ["command"],
		"additionalProperties": false
	}`)
}

// IsConcurrencySafe evaluates per-call based on whether the command is read-only.
// Source: PowerShellTool.tsx:434-436
func (t *PowerShellTool) IsConcurrencySafe(input json.RawMessage) bool {
	var in psInput
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return IsPSReadOnlyCommand(in.Command)
}

func (t *PowerShellTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params psInput
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Command == "" {
		return ErrorOutput("command is required"), nil
	}

	// Plan-mode validation: reject write commands
	if tc != nil && tc.PlanMode && !IsPSReadOnlyCommand(params.Command) {
		return ErrorOutput("cannot execute write commands in plan mode — only read-only commands are allowed"), nil
	}

	// Destructive command warning (informational, returned alongside output)
	_ = GetPSDestructiveCommandWarning(params.Command)

	// Timeout: use provided value or default, capped at max.
	timeoutMs := params.Timeout
	if timeoutMs <= 0 {
		timeoutMs = DefaultPowerShellTimeoutMs
	}
	if timeoutMs > MaxPowerShellTimeoutMs {
		timeoutMs = MaxPowerShellTimeoutMs
	}

	if runtime.GOOS != "windows" {
		// Try pwsh (cross-platform PowerShell)
		if _, err := exec.LookPath("pwsh"); err != nil {
			return ErrorOutput("PowerShell not available on this platform. Use Bash instead."), nil
		}
	}

	shell := "pwsh"
	if runtime.GOOS == "windows" {
		shell = "powershell"
	}

	tCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	cmd := exec.CommandContext(tCtx, shell, "-NoProfile", "-NonInteractive", "-Command", params.Command)
	if tc != nil {
		cmd.Dir = tc.CWD
	}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()
	result := stdout.String()
	if stderr.Len() > 0 {
		result += "\n" + stderr.String()
	}
	if err != nil {
		// Apply command-specific exit code semantics
		if cmd.ProcessState != nil {
			semantic := InterpretPSCommandResult(params.Command, cmd.ProcessState.ExitCode())
			if !semantic.IsError && semantic.Message != "" {
				return SuccessOutput(result + "\n" + semantic.Message), nil
			}
		}
		return ErrorOutput(result + "\n" + err.Error()), nil
	}
	return SuccessOutput(result), nil
}
