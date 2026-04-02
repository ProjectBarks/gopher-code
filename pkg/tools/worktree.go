package tools

import (
	"context"
	"encoding/json"
	"fmt"
	"os/exec"
	"path/filepath"
	"strings"
)

// EnterWorktreeTool creates a git worktree for isolated work.
type EnterWorktreeTool struct{}

func (t *EnterWorktreeTool) Name() string        { return "EnterWorktree" }
func (t *EnterWorktreeTool) Description() string { return "Create a git worktree for isolated work" }
func (t *EnterWorktreeTool) IsReadOnly() bool    { return false }

func (t *EnterWorktreeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"name": {"type": "string", "description": "Name for the worktree directory"},
			"branch": {"type": "string", "description": "Branch name to create or check out"}
		},
		"required": [],
		"additionalProperties": false
	}`)
}

func (t *EnterWorktreeTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Name   string `json:"name"`
		Branch string `json:"branch"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}

	if params.Name == "" {
		params.Name = "worktree"
	}
	if params.Branch == "" {
		params.Branch = params.Name
	}

	worktreePath := filepath.Join(tc.CWD, "..", params.Name)

	args := []string{"worktree", "add", "-b", params.Branch, worktreePath}
	cmd := exec.CommandContext(ctx, "git", args...)
	cmd.Dir = tc.CWD
	out, err := cmd.CombinedOutput()
	if err != nil {
		// Try without -b in case the branch already exists
		args = []string{"worktree", "add", worktreePath, params.Branch}
		cmd = exec.CommandContext(ctx, "git", args...)
		cmd.Dir = tc.CWD
		out, err = cmd.CombinedOutput()
		if err != nil {
			return ErrorOutput(fmt.Sprintf("git worktree add failed: %s\n%s", err, strings.TrimSpace(string(out)))), nil
		}
	}

	return SuccessOutput(fmt.Sprintf("Created worktree at %s on branch %s", worktreePath, params.Branch)), nil
}

// ExitWorktreeTool cleans up a git worktree.
type ExitWorktreeTool struct{}

func (t *ExitWorktreeTool) Name() string        { return "ExitWorktree" }
func (t *ExitWorktreeTool) Description() string { return "Exit and clean up a git worktree" }
func (t *ExitWorktreeTool) IsReadOnly() bool    { return false }

func (t *ExitWorktreeTool) InputSchema() json.RawMessage {
	return json.RawMessage(`{
		"type": "object",
		"properties": {
			"path": {"type": "string", "description": "Path to the worktree to remove"}
		},
		"required": ["path"],
		"additionalProperties": false
	}`)
}

func (t *ExitWorktreeTool) Execute(ctx context.Context, tc *ToolContext, input json.RawMessage) (*ToolOutput, error) {
	var params struct {
		Path string `json:"path"`
	}
	if err := json.Unmarshal(input, &params); err != nil {
		return ErrorOutput(fmt.Sprintf("invalid input: %s", err)), nil
	}
	if params.Path == "" {
		return ErrorOutput("path is required"), nil
	}

	cmd := exec.CommandContext(ctx, "git", "worktree", "remove", params.Path)
	cmd.Dir = tc.CWD
	out, err := cmd.CombinedOutput()
	if err != nil {
		return ErrorOutput(fmt.Sprintf("git worktree remove failed: %s\n%s", err, strings.TrimSpace(string(out)))), nil
	}

	return SuccessOutput(fmt.Sprintf("Removed worktree at %s", params.Path)), nil
}
