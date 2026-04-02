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

func TestFileWriteTool(t *testing.T) {
	tool := &tools.FileWriteTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Write" {
			t.Errorf("expected 'Write', got %q", tool.Name())
		}
	})

	t.Run("is_not_read_only", func(t *testing.T) {
		if tool.IsReadOnly() {
			t.Error("FileWriteTool should not be read-only")
		}
	})

	t.Run("happy_path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "output.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "hello world"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read written file: %v", err)
		}
		if string(data) != "hello world" {
			t.Errorf("expected 'hello world', got %q", string(data))
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"file_path": "relative.txt", "content": "data"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, err := os.ReadFile(filepath.Join(dir, "relative.txt"))
		if err != nil {
			t.Fatalf("file not found: %v", err)
		}
		if string(data) != "data" {
			t.Errorf("expected 'data', got %q", string(data))
		}
	})

	t.Run("creates_parent_directories", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "a", "b", "c", "deep.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "nested"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("file not found: %v", err)
		}
		if string(data) != "nested" {
			t.Errorf("expected 'nested', got %q", string(data))
		}
	})

	t.Run("overwrites_existing_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new"}`, filePath))
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
		if string(data) != "new" {
			t.Errorf("expected 'new', got %q", string(data))
		}
	})

	t.Run("empty_file_path", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"file_path": "", "content": "data"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty file_path")
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
