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
		// Offset is 1-indexed: offset=2 means "start reading from line 2".
		// Source: FileReadTool.ts:497 — offset semantics match TS exactly
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		// offset=2, limit=2: read lines 2-3 ("b", "c")
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 2, "limit": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "1\ta") {
			t.Error("expected offset=2 to skip line 1")
		}
		if !strings.Contains(out.Content, "2\tb") {
			t.Errorf("expected line 2, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "3\tc") {
			t.Errorf("expected line 3, got %q", out.Content)
		}
		if strings.Contains(out.Content, "4\td") {
			t.Error("expected limit=2 to exclude line 4")
		}
	})

	t.Run("offset_1_reads_from_start", func(t *testing.T) {
		// offset=1 means "start from line 1" (same as no offset)
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("a\nb\nc\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 1, "limit": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "1\ta") {
			t.Errorf("offset=1 should include line 1, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "2\tb") {
			t.Errorf("offset=1 should include line 2, got %q", out.Content)
		}
		if strings.Contains(out.Content, "3\tc") {
			t.Error("limit=2 should exclude line 3")
		}
	})

	t.Run("empty_file", func(t *testing.T) {
		// Source: FileReadTool.ts:692-708 — empty file returns system warning
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
		if !strings.Contains(out.Content, "contents are empty") {
			t.Errorf("expected empty file warning, got %q", out.Content)
		}
	})

	t.Run("offset_beyond_eof", func(t *testing.T) {
		// Source: FileReadTool.ts:700-706 — offset beyond file returns warning with line count
		dir := t.TempDir()
		filePath := filepath.Join(dir, "short.txt")
		os.WriteFile(filePath, []byte("a\nb\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 100}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "shorter than the provided offset") {
			t.Errorf("expected offset-beyond-eof warning, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "2 lines") {
			t.Errorf("expected line count in warning, got %q", out.Content)
		}
	})

	t.Run("blocked_device_path", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "/dev/zero"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for /dev/zero")
		}
		if !strings.Contains(out.Content, "could cause the process to hang") {
			t.Errorf("expected hang warning, got %q", out.Content)
		}
	})

	t.Run("tilde_expansion", func(t *testing.T) {
		// Tilde should be expanded to home dir
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "~/nonexistent_test_file_12345"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should attempt to read from home dir, not treat ~ literally
		if !strings.Contains(out.Content, "does not exist") {
			t.Errorf("expected file-not-found from home dir, got %q", out.Content)
		}
		// Should NOT contain literal "~/"
		if strings.Contains(out.Content, "~/") {
			t.Error("tilde should be expanded, not treated literally")
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
