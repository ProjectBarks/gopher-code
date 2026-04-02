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

func TestListDirectoryTool(t *testing.T) {
	tool := &tools.ListDirectoryTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "LS" {
			t.Errorf("expected 'LS', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("ListDirectoryTool should be read-only")
		}
	})

	t.Run("valid_schema", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("schema is not valid JSON: %v", err)
		}
	})

	t.Run("list_cwd", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("a"), 0644)
		os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "a.txt") || !strings.Contains(out.Content, "b.txt") {
			t.Errorf("expected both files in output, got %q", out.Content)
		}
	})

	t.Run("list_with_path", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "sub")
		os.Mkdir(subdir, 0755)
		os.WriteFile(filepath.Join(subdir, "file.txt"), []byte("data"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"path": %q}`, subdir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "file.txt") {
			t.Errorf("expected file.txt in output, got %q", out.Content)
		}
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		subdir := filepath.Join(dir, "mydir")
		os.Mkdir(subdir, 0755)
		os.WriteFile(filepath.Join(subdir, "inner.txt"), []byte("data"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"path": "mydir"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "inner.txt") {
			t.Errorf("expected inner.txt, got %q", out.Content)
		}
	})

	t.Run("hidden_files_excluded_by_default", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0644)
		os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("v"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, ".hidden") {
			t.Errorf("hidden files should not appear without all=true, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "visible.txt") {
			t.Errorf("expected visible.txt in output, got %q", out.Content)
		}
	})

	t.Run("hidden_files_included_with_all", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".hidden"), []byte("h"), 0644)
		os.WriteFile(filepath.Join(dir, "visible.txt"), []byte("v"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"all": true}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, ".hidden") {
			t.Errorf("expected .hidden with all=true, got %q", out.Content)
		}
	})

	t.Run("long_format", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"long": true}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Long format should include size
		if !strings.Contains(out.Content, "5") {
			t.Errorf("expected file size in long format, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "test.txt") {
			t.Errorf("expected filename in output, got %q", out.Content)
		}
	})

	t.Run("directories_have_trailing_slash", func(t *testing.T) {
		dir := t.TempDir()
		os.Mkdir(filepath.Join(dir, "subdir"), 0755)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "subdir/") {
			t.Errorf("expected trailing slash on directory, got %q", out.Content)
		}
	})

	t.Run("empty_directory", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "empty") {
			t.Errorf("expected 'empty' in output for empty dir, got %q", out.Content)
		}
	})

	t.Run("nonexistent_directory", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"path": "/nonexistent/directory"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent directory")
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
}
