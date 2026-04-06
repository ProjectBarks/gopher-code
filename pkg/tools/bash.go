package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
	"syscall"
	"time"
)

// Bash timeout constants matching TS source.
// Source: utils/timeouts.ts:2-3
const (
	DefaultBashTimeoutMs = 120_000 // 2 minutes
	MaxBashTimeoutMs     = 600_000 // 10 minutes
)

// Output truncation constants matching TS source.
// Source: utils/shell/outputLimits.ts
const (
	BashMaxOutputDefault    = 30_000  // Default max output length in chars
	BashMaxOutputUpperLimit = 150_000 // Upper limit for env var override
)

// AssistantBlockingBudgetMs is the threshold after which long-running commands
// are auto-backgrounded in assistant mode.
// Source: BashTool.tsx:57
const AssistantBlockingBudgetMs = 15_000 // 15 seconds

// DisallowedAutoBackgroundCommands are commands that should NOT be auto-backgrounded.
// Source: BashTool.tsx:220
var DisallowedAutoBackgroundCommands = []string{"sleep"}

// BashTool executes shell commands.
type BashTool struct{}

type bashInput struct {
	Command                  string `json:"command"`
	Description              string `json:"description,omitempty"`
	Timeout                  int    `json:"timeout,omitempty"` // milliseconds
	RunInBackground          bool   `json:"run_in_background,omitempty"`
	DangerouslyDisableSandbox bool  `json:"dangerouslyDisableSandbox,omitempty"`
}

func (b *BashTool) Name() string        { return "Bash" }
func (b *BashTool) Description() string { return "Executes a given bash command and returns its output." }
func (b *BashTool) IsReadOnly() bool    { return false }

// IsConcurrencySafe evaluates per-call based on whether the command is read-only.
// Source: BashTool.tsx:434-436
func (b *BashTool) IsConcurrencySafe(input json.RawMessage) bool {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return IsReadOnlyCommand(in.Command)
}

// Source: BashTool.tsx:220-259
func (b *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The command to execute"},
			"description": {"type": "string", "description": "Clear, concise description of what this command does in active voice."},
			"timeout": {"type": "integer", "description": "Optional timeout in milliseconds (up to 600000ms / 10 minutes). By default, your command will timeout after 120000ms (2 minutes)."},
			"run_in_background": {"type": "boolean", "description": "Set to true to run this command in the background. Use Read to read the output later."},
			"dangerouslyDisableSandbox": {"type": "boolean", "description": "Set this to true to dangerously override sandbox mode and run commands without sandboxing."}
		},
		"required": ["command"],
		"additionalProperties": false
	}`)
}

// CheckPermissions implements ToolPermissionChecker for BashTool.
// Runs security and validation checks before the generic permission waterfall.
// Source: BashTool.tsx — pre-execution validation pipeline
func (b *BashTool) CheckPermissions(_ context.Context, tc *ToolContext, input json.RawMessage) PermissionCheckResult {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return PermissionCheckResult{Behavior: "deny", Message: fmt.Sprintf("invalid input: %s", err)}
	}

	if rejection := ValidateBashCommand(in.Command, tc.CWD, tc.ProjectDir, tc.PlanMode); rejection != nil {
		return PermissionCheckResult{Behavior: "deny", Message: rejection.Content}
	}

	// Check for destructive commands — surface warning but don't block
	if warning := GetDestructiveCommandWarning(in.Command); warning != "" {
		return PermissionCheckResult{Behavior: "ask", Message: warning}
	}

	return PermissionCheckResult{Behavior: "passthrough"}
}

// IsDestructive implements DestructiveChecker for BashTool.
// Source: BashTool.tsx — destructive command detection
func (b *BashTool) IsDestructive(input json.RawMessage) bool {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return false
	}
	return GetDestructiveCommandWarning(in.Command) != ""
}

func (b *BashTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Command == "" {
		return ErrorOutput("command is required"), nil
	}

	// Run validation pipeline (security + path + mode checks)
	if rejection := ValidateBashCommand(in.Command, tc.CWD, tc.ProjectDir, tc.PlanMode); rejection != nil {
		return rejection, nil
	}

	// Timeout: default 120s, max 600s, matching TS source
	// Source: utils/timeouts.ts:2-3, 12-39
	timeoutMs := DefaultBashTimeoutMs
	if in.Timeout > 0 {
		timeoutMs = in.Timeout
		if timeoutMs > MaxBashTimeoutMs {
			timeoutMs = MaxBashTimeoutMs
		}
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeoutMs)*time.Millisecond)
	defer cancel()

	// Use user's shell, falling back to bash.
	// Source: BashTool.tsx:881 — exec(command, ..., 'bash', ...)
	// Source: utils/shell/bashProvider.ts — uses $SHELL or /bin/bash
	shell := getUserShell()
	cmd := exec.CommandContext(cmdCtx, shell, "-c", in.Command)
	cmd.Dir = tc.CWD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	// Build output: stdout + stderr combined, then truncate.
	// Source: BashTool.tsx:686-699
	stdoutStr := stdout.String()
	stderrStr := stderr.String()
	var outputBuilder strings.Builder
	outputBuilder.WriteString(stdoutStr)
	if stderrStr != "" {
		if stdoutStr != "" && !strings.HasSuffix(stdoutStr, "\n") {
			outputBuilder.WriteByte('\n')
		}
		outputBuilder.WriteString(stderrStr)
	}

	output := stripEmptyLines(outputBuilder.String())

	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return ErrorOutput(fmt.Sprintf("command timed out after %d seconds", timeoutMs/1000)), nil
		}
		// Append exit code to output, matching TS behavior.
		// Source: BashTool.tsx:698-699 — stdoutAccumulator.append(`Exit code ${result.code}`)
		exitCode := getExitCode(err)
		if output != "" {
			output += "\n"
		}
		output += fmt.Sprintf("Exit code %d", exitCode)
	}

	// Truncate output to max length.
	// Source: BashTool/utils.ts:133-165 — formatOutput()
	output = truncateBashOutput(output)

	return SuccessOutput(output), nil
}

// getUserShell returns the user's preferred shell from $SHELL, falling back to bash.
// Source: utils/shell/bashProvider.ts — shell detection
func getUserShell() string {
	if shell := os.Getenv("SHELL"); shell != "" {
		return shell
	}
	// Fallback to bash
	if path, err := exec.LookPath("bash"); err == nil {
		return path
	}
	return "/bin/sh"
}

// getExitCode extracts the exit code from an exec error.
func getExitCode(err error) int {
	if exitErr, ok := err.(*exec.ExitError); ok {
		return exitErr.ExitCode()
	}
	return 1
}

// getMaxBashOutputLength returns the max output length, respecting BASH_MAX_OUTPUT_LENGTH env var.
// Source: utils/shell/outputLimits.ts
func getMaxBashOutputLength() int {
	if envVal := os.Getenv("BASH_MAX_OUTPUT_LENGTH"); envVal != "" {
		if n, err := strconv.Atoi(envVal); err == nil && n > 0 {
			if n > BashMaxOutputUpperLimit {
				return BashMaxOutputUpperLimit
			}
			return n
		}
	}
	return BashMaxOutputDefault
}

// truncateBashOutput truncates output to the configured max length.
// Source: BashTool/utils.ts:133-165 — formatOutput()
func truncateBashOutput(output string) string {
	maxLen := getMaxBashOutputLength()
	if len(output) <= maxLen {
		return output
	}
	truncated := output[:maxLen]
	// Count remaining lines
	remaining := strings.Count(output[maxLen:], "\n") + 1
	return fmt.Sprintf("%s\n\n... [%d lines truncated] ...", truncated, remaining)
}

// stripEmptyLines strips leading and trailing lines that contain only whitespace.
// Source: BashTool/utils.ts:22-44
func stripEmptyLines(content string) string {
	lines := strings.Split(content, "\n")

	start := 0
	for start < len(lines) && strings.TrimSpace(lines[start]) == "" {
		start++
	}
	end := len(lines) - 1
	for end >= 0 && strings.TrimSpace(lines[end]) == "" {
		end--
	}
	if start > end {
		return ""
	}
	return strings.Join(lines[start:end+1], "\n")
}
