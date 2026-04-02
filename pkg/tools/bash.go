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

// BashTool executes shell commands.
type BashTool struct{}

type bashInput struct {
	Command string `json:"command"`
	Timeout int    `json:"timeout"`
}

func (b *BashTool) Name() string        { return "Bash" }
func (b *BashTool) Description() string { return "Execute a bash command" }
func (b *BashTool) IsReadOnly() bool    { return false }

func (b *BashTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"command": {"type": "string", "description": "The bash command to execute"},
			"timeout": {"type": "integer", "description": "Timeout in seconds (default 30)"}
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

	timeout := 30
	if in.Timeout > 0 {
		timeout = in.Timeout
	}

	cmdCtx, cancel := context.WithTimeout(ctx, time.Duration(timeout)*time.Second)
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
			return ErrorOutput(fmt.Sprintf("command timed out after %d seconds", timeout)), nil
		}
		return ErrorOutput(fmt.Sprintf("command failed: %s\n%s", err, output)), nil
	}

	return SuccessOutput(output), nil
}
