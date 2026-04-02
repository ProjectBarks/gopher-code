package tools

import (
	"bytes"
	"context"
	"encoding/json"
	"os/exec"
	"strings"
	"time"
)

// REPLTool starts an interactive REPL session for a programming language.
type REPLTool struct{}

func (t *REPLTool) Name() string        { return "REPL" }
func (t *REPLTool) Description() string { return "Start an interactive REPL session for a programming language" }
func (t *REPLTool) IsReadOnly() bool    { return false }

func (t *REPLTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"language": {"type": "string", "description": "Programming language (python, node, etc.)"},
			"command": {"type": "string", "description": "Command to execute in the REPL"}
		},
		"required": ["language", "command"],
		"additionalProperties": false
	}`)
}

func (t *REPLTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Language string `json:"language"`
		Command  string `json:"command"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput("invalid input: " + err.Error()), nil
	}

	var shell string
	switch strings.ToLower(params.Language) {
	case "python", "python3", "py":
		shell = "python3"
	case "node", "nodejs", "js", "javascript":
		shell = "node"
	case "ruby", "rb":
		shell = "ruby"
	default:
		shell = params.Language
	}

	tCtx, cancel := context.WithTimeout(ctx, 30*time.Second)
	defer cancel()

	cmd := exec.CommandContext(tCtx, shell, "-c", params.Command)
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
