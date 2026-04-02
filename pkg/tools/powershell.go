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

// PowerShellTool executes PowerShell commands. Primarily for Windows, but
// also works on other platforms if pwsh (cross-platform PowerShell) is installed.
type PowerShellTool struct{}

func (t *PowerShellTool) Name() string        { return "PowerShell" }
func (t *PowerShellTool) Description() string { return "Execute PowerShell commands (Windows)" }
func (t *PowerShellTool) IsReadOnly() bool    { return false }

func (t *PowerShellTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "PowerShell command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default 30)"}
		},
		"required": ["command"],
		"additionalProperties": false
	}`)
}

func (t *PowerShellTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Command string `json:"command"`
		Timeout int    `json:"timeout"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Command == "" {
		return ErrorOutput("command is required"), nil
	}
	if params.Timeout <= 0 {
		params.Timeout = 30
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

	tCtx, cancel := context.WithTimeout(ctx, time.Duration(params.Timeout)*time.Second)
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
		return ErrorOutput(result + "\n" + err.Error()), nil
	}
	return SuccessOutput(result), nil
}
