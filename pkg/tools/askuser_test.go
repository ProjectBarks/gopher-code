package tools_test

import (
	"context"
	"encoding/json"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestAskUserQuestionTool(t *testing.T) {
	tool := &tools.AskUserQuestionTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "AskUserQuestion" {
			t.Errorf("expected 'AskUserQuestion', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("AskUserQuestionTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
		props, ok := parsed["properties"].(map[string]interface{})
		if !ok {
			t.Fatal("schema missing properties")
		}
		if _, ok := props["question"]; !ok {
			t.Error("schema missing 'question' property")
		}
	})

	t.Run("returns_question_in_output", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"question": "What is your name?"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "What is your name?") {
			t.Errorf("output should contain the question, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "[Question for user]") {
			t.Errorf("output should contain prefix, got %q", out.Content)
		}
	})

	t.Run("empty_question_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"question": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error output for empty question")
		}
	})

	t.Run("invalid_json_returns_error", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{invalid}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error output for invalid JSON")
		}
	})

	t.Run("non_interactive_disclaimer", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"question": "test?"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "non-interactive") {
			t.Errorf("output should mention non-interactive mode, got %q", out.Content)
		}
	})
}
