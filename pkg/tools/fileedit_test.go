package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestFileEditTool(t *testing.T) {
	tool := &tools.FileEditTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Edit" {
			t.Errorf("expected 'Edit', got %q", tool.Name())
		}
	})

	t.Run("is_not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("FileEditTool should not be read-only")
		}
	})

	t.Run("happy_path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "world", "new_string": "gopher"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != "hello gopher" {
			t.Errorf("expected 'hello gopher', got %q", string(data))
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "rel.txt"), []byte("foo bar baz"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"file_path": "rel.txt", "old_string": "bar", "new_string": "qux"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filepath.Join(dir, "rel.txt"))
		if string(data) != "foo qux baz" {
			t.Errorf("expected 'foo qux baz', got %q", string(data))
		}
	})

	t.Run("string_not_found", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "missing", "new_string": "x"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when old_string not found")
		}
	})

	t.Run("string_not_unique", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("aa bb aa cc"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "aa", "new_string": "zz"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when old_string appears multiple times")
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"file_path": "nonexistent.txt", "old_string": "a", "new_string": "b"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("same_old_and_new_string", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "hello", "new_string": "hello"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when old_string equals new_string")
		}
	})

	t.Run("empty_old_string", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "x"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty old_string")
		}
	})

	t.Run("invalid_json", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{bad}`)
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
	})
}
