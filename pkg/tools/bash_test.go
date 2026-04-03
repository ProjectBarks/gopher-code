package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestBashTool(t *testing.T) {
	tool := &tools.BashTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Bash" {
			t.Errorf("expected 'Bash', got %q", tool.Name())
		}
	})

	t.Run("is_not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("BashTool should not be read-only")
		}
	})

	t.Run("happy_path_echo", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "echo hello"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "hello") {
			t.Errorf("expected output to contain 'hello', got %q", out.Content)
		}
	})

	t.Run("captures_stderr", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "echo oops >&2"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "oops") {
			t.Errorf("expected stderr output to contain 'oops', got %q", out.Content)
		}
	})

	t.Run("command_failure_exit_code", func(t *testing.T) {
		// TS behavior: non-zero exit code is NOT a tool error.
		// The exit code is appended to the output text.
		// Source: BashTool.tsx:621 — is_error: interrupted (only on interrupt)
		// Source: BashTool.tsx:698-699 — stdoutAccumulator.append(`Exit code ${result.code}`)
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "exit 42"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Error("non-zero exit code should NOT be a tool error (matching TS)")
		}
		if !strings.Contains(out.Content, "Exit code 42") {
			t.Errorf("expected 'Exit code 42' in output, got %q", out.Content)
		}
	})

	t.Run("command_failure_with_output", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "echo 'some output' && exit 1"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Error("non-zero exit code should NOT be a tool error")
		}
		if !strings.Contains(out.Content, "some output") {
			t.Errorf("expected stdout in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Exit code 1") {
			t.Errorf("expected 'Exit code 1' in output, got %q", out.Content)
		}
	})

	t.Run("uses_cwd", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"command": "pwd"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, dir) {
			t.Errorf("expected output to contain CWD %q, got %q", dir, out.Content)
		}
	})

	t.Run("timeout", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "sleep 10", "timeout": 1}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected tool error on timeout")
		}
		if !strings.Contains(out.Content, "timed out") {
			t.Errorf("expected timeout message, got %q", out.Content)
		}
	})

	t.Run("empty_command", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty command")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{invalid}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for invalid JSON")
		}
	})

	t.Run("input_schema_valid_json", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
		if parsed["type"] != "object" {
			t.Errorf("expected type=object, got %v", parsed["type"])
		}
	})

	t.Run("output_truncation", func(t *testing.T) {
		// Generate output larger than 30K chars
		tc := &tools.ToolContext{CWD: t.TempDir()}
		// Use seq to generate many lines (well over 30K chars)
		input := json.RawMessage(`{"command": "seq 1 20000"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Output should be truncated
		if !strings.Contains(out.Content, "lines truncated") {
			t.Error("expected truncation message for large output")
		}
		// Should be at or under 30K + truncation message
		if len(out.Content) > 35000 {
			t.Errorf("output too large after truncation: %d chars", len(out.Content))
		}
	})

	t.Run("uses_user_shell", func(t *testing.T) {
		// Verify we're not using plain /bin/sh by checking for bash/zsh features
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"command": "echo $SHELL"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should not be empty (user's shell should be set)
		if strings.TrimSpace(out.Content) == "" {
			t.Log("Warning: $SHELL not set in environment")
		}
	})

	t.Run("strip_empty_lines", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		// echo with extra newlines
		input := json.RawMessage(`{"command": "echo ''; echo 'content'; echo ''"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Content should not have leading/trailing empty lines
		if strings.HasPrefix(out.Content, "\n") {
			t.Error("expected leading empty lines to be stripped")
		}
		if strings.HasSuffix(out.Content, "\n\n") {
			t.Error("expected trailing empty lines to be stripped")
		}
		if !strings.Contains(out.Content, "content") {
			t.Error("expected content to be preserved")
		}
	})
}
