package tools_test

import (
	"context"
	"encoding/json"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestEnterWorktreeTool(t *testing.T) {
	tool := &tools.EnterWorktreeTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "EnterWorktree" {
			t.Errorf("expected 'EnterWorktree', got %q", tool.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("EnterWorktree should not be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("fails_outside_git_repo", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"name": "test-worktree", "branch": "test-branch"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when not in a git repo")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})
}

func TestExitWorktreeTool(t *testing.T) {
	tool := &tools.ExitWorktreeTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "ExitWorktree" {
			t.Errorf("expected 'ExitWorktree', got %q", tool.Name())
		}
	})

	t.Run("not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("ExitWorktree should not be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		var parsed map[string]interface{}
		if err := json.Unmarshal(tool.InputSchema(), &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("missing_path_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"path": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for missing path")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		out, err := tool.Execute(context.Background(), tc, json.RawMessage(`{bad}`))
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("fails_for_nonexistent_worktree", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"path": "/nonexistent/worktree"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent worktree")
		}
	})
}
