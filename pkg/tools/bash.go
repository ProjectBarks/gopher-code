package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"syscall"
	"time"
)

// Bash timeout constants matching TS source.
// Source: utils/timeouts.ts:2-3
const (
	DefaultBashTimeoutMs = 120_000 // 2 minutes
	MaxBashTimeoutMs     = 600_000 // 10 minutes
)

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

func (b *BashTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var in bashInput
	if err := json.Unmarshal(input, &in); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if in.Command == "" {
		return ErrorOutput("command is required"), nil
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

	cmd := exec.CommandContext(cmdCtx, "sh", "-c", in.Command)
	cmd.Dir = tc.CWD
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}

	var stdout, stderr bytes.Buffer
	cmd.Stdout = &stdout
	cmd.Stderr = &stderr

	err := cmd.Run()

	output := stdout.String() + stderr.String()
	if err != nil {
		if cmdCtx.Err() == context.DeadlineExceeded {
			return ErrorOutput(fmt.Sprintf("command timed out after %d seconds", timeoutMs/1000)), nil
		}
		return ErrorOutput(fmt.Sprintf("command failed: %s\n%s", err, output)), nil
	}

	return SuccessOutput(output), nil
}
