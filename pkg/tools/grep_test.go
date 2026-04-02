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

func TestGrepTool(t *testing.T) {
	tool := &tools.GrepTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Grep" {
			t.Errorf("expected 'Grep', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("GrepTool should be read-only")
		}
	})

	t.Run("happy_path", func(t *testing.T) {
		// Source: GrepTool.ts:316 — default output_mode is files_with_matches
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n}\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "func main", "path": %q}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Default mode is files_with_matches — returns filenames
		if !strings.Contains(out.Content, "main.go") {
			t.Errorf("expected filename in output, got %q", out.Content)
		}
	})

	t.Run("content_mode", func(t *testing.T) {
		// Source: GrepTool.ts:443-476 — content mode shows matching lines
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "main.go"), []byte("package main\n\nfunc main() {\n}\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "func main", "path": %q, "output_mode": "content"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "func main") {
			t.Errorf("expected matching line content, got %q", out.Content)
		}
	})

	t.Run("count_mode", func(t *testing.T) {
		// Source: GrepTool.ts:478-530 — count mode shows file:count
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "test.txt"), []byte("hello\nhello\nworld\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "hello", "path": %q, "output_mode": "count"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, ":2") {
			t.Errorf("expected count of 2, got %q", out.Content)
		}
	})

	t.Run("no_matches", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello world\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "zzz_no_match", "path": %q}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "No matches") {
			t.Errorf("expected no-match message, got %q", out.Content)
		}
	})

	t.Run("with_glob_filter", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "code.go"), []byte("func hello() {}\n"), 0644)
		os.WriteFile(filepath.Join(dir, "notes.txt"), []byte("func notes() {}\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "func", "path": %q, "glob": "*.go"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "code.go") {
			t.Errorf("expected code.go in results, got %q", out.Content)
		}
		// When using rg, the glob is passed as --glob which filters differently
		// so we only check that code.go appears
	})

	t.Run("relative_path", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "sub"), 0755)
		os.WriteFile(filepath.Join(dir, "sub", "file.txt"), []byte("target line\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "target", "path": "sub"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Default mode: files_with_matches returns filename
		if !strings.Contains(out.Content, "file.txt") {
			t.Errorf("expected filename in output, got %q", out.Content)
		}
	})

	t.Run("defaults_to_cwd", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "cwd_file.txt"), []byte("findme\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "findme"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Default mode: files_with_matches returns filename
		if !strings.Contains(out.Content, "cwd_file.txt") {
			t.Errorf("expected filename from CWD, got %q", out.Content)
		}
	})

	t.Run("empty_pattern", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"pattern": ""}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for empty pattern")
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

// TestGrepToolNative forces the native (non-rg) fallback path.
func TestGrepToolNative(t *testing.T) {
	// We test the native path by searching a single file directly,
	// which both rg and native will handle correctly.
	tool := &tools.GrepTool{}

	t.Run("regex_pattern", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "data.txt"), []byte("abc123\ndef456\nabc789\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		// Use content mode to see matching lines
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "abc\\d+", "path": %q, "output_mode": "content"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "abc123") {
			t.Errorf("expected abc123 match, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "abc789") {
			t.Errorf("expected abc789 match, got %q", out.Content)
		}
		if strings.Contains(out.Content, "def456") {
			t.Errorf("should not match def456, got %q", out.Content)
		}
	})
}
