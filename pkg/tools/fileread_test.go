package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func TestFileReadTool(t *testing.T) {
	tool := &tools.FileReadTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Read" {
			t.Errorf("expected 'Read', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("FileReadTool should be read-only")
		}
	})

	t.Run("happy_path", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "1\tline1") {
			t.Errorf("expected line numbers in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "2\tline2") {
			t.Errorf("expected line 2 in output, got %q", out.Content)
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "hello.txt"), []byte("hello world\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"file_path": "hello.txt"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "hello world") {
			t.Errorf("expected file content, got %q", out.Content)
		}
	})

	t.Run("file_not_found", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"file_path": "nonexistent.txt"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent file")
		}
	})

	t.Run("offset", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if strings.Contains(out.Content, "1\tline1") {
			t.Errorf("expected offset to skip line 1, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "3\tline3") {
			t.Errorf("expected line 3 in output, got %q", out.Content)
		}
	})

	t.Run("limit", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("line1\nline2\nline3\nline4\nline5\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "limit": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "1\tline1") {
			t.Errorf("expected line 1, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "2\tline2") {
			t.Errorf("expected line 2, got %q", out.Content)
		}
		if strings.Contains(out.Content, "3\tline3") {
			t.Errorf("expected limit to exclude line 3, got %q", out.Content)
		}
	})

	t.Run("offset_and_limit", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 1, "limit": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "1\ta") {
			t.Error("expected offset to skip first line")
		}
		if !strings.Contains(out.Content, "2\tb") {
			t.Errorf("expected line 2, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "3\tc") {
			t.Errorf("expected line 3, got %q", out.Content)
		}
		if strings.Contains(out.Content, "4\td") {
			t.Error("expected limit to exclude line 4")
		}
	})

	t.Run("empty_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "empty.txt")
		os.WriteFile(filePath, []byte(""), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
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
