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

func TestGlobTool(t *testing.T) {
	tool := &tools.GlobTool{}

	t.Run("name", func(t *testing.T) {
		if tool.Name() != "Glob" {
			t.Errorf("expected 'Glob', got %q", tool.Name())
		}
	})

	t.Run("is_read_only", func(t *testing.T) {
		if !tool.IsReadOnly() {
			t.Error("GlobTool should be read-only")
		}
	})

	t.Run("happy_path", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "foo.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(dir, "bar.go"), []byte("package main"), 0644)
		os.WriteFile(filepath.Join(dir, "baz.txt"), []byte("text"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "foo.go") {
			t.Errorf("expected foo.go in output, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "bar.go") {
			t.Errorf("expected bar.go in output, got %q", out.Content)
		}
		if strings.Contains(out.Content, "baz.txt") {
			t.Errorf("did not expect baz.txt in output, got %q", out.Content)
		}
	})

	t.Run("with_subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "sub"), 0755)
		os.WriteFile(filepath.Join(dir, "sub", "deep.go"), []byte("package sub"), 0644)
		os.WriteFile(filepath.Join(dir, "top.go"), []byte("package main"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "top.go") {
			t.Errorf("expected top.go, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "deep.go") {
			t.Errorf("expected deep.go in subdirectory, got %q", out.Content)
		}
	})

	t.Run("skips_git_directory", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, ".git"), 0755)
		os.WriteFile(filepath.Join(dir, ".git", "config.txt"), []byte("git config"), 0644)
		os.WriteFile(filepath.Join(dir, "main.txt"), []byte("main"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.txt"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "config.txt") {
			t.Errorf("should not include .git files, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "main.txt") {
			t.Errorf("expected main.txt, got %q", out.Content)
		}
	})

	t.Run("skips_node_modules", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
		os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("module.exports={}"), 0644)
		os.WriteFile(filepath.Join(dir, "app.js"), []byte("console.log()"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.js"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "index.js") {
			t.Errorf("should not include node_modules, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "app.js") {
			t.Errorf("expected app.js, got %q", out.Content)
		}
	})

	t.Run("custom_path", func(t *testing.T) {
		dir := t.TempDir()
		subDir := filepath.Join(dir, "subdir")
		os.MkdirAll(subDir, 0755)
		os.WriteFile(filepath.Join(subDir, "a.txt"), []byte("a"), 0644)
		os.WriteFile(filepath.Join(dir, "b.txt"), []byte("b"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "*.txt", "path": %q}`, subDir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "a.txt") {
			t.Errorf("expected a.txt from subdir, got %q", out.Content)
		}
		if strings.Contains(out.Content, "b.txt") {
			t.Errorf("should not include files outside subdir, got %q", out.Content)
		}
	})

	t.Run("no_matches", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("data"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.xyz"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "No files found") {
			t.Errorf("expected no-match message, got %q", out.Content)
		}
	})

	t.Run("nonexistent_path", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go", "path": "/nonexistent/path/abcdef"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent path")
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

	t.Run("results_are_sorted", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "c.go"), []byte("c"), 0644)
		os.WriteFile(filepath.Join(dir, "a.go"), []byte("a"), 0644)
		os.WriteFile(filepath.Join(dir, "b.go"), []byte("b"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimSpace(out.Content), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 results, got %d", len(lines))
		}
		if lines[0] != "a.go" || lines[1] != "b.go" || lines[2] != "c.go" {
			t.Errorf("expected sorted order [a.go, b.go, c.go], got %v", lines)
		}
	})

	t.Run("input_schema_valid_json", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
	})

	t.Run("truncates_at_100_results", func(t *testing.T) {
		// Source: GlobTool.ts:157 — default limit 100
		// Source: GlobTool.ts:190-193 — truncation message
		dir := t.TempDir()
		// Create 120 files
		for i := 0; i < 120; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("file_%03d.txt", i)), []byte("x"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input, _ := json.Marshal(map[string]string{"pattern": "*.txt"})
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		lines := strings.Split(strings.TrimSpace(out.Content), "\n")
		// Should have 100 file lines + 1 truncation message
		if len(lines) != 101 {
			t.Errorf("expected 101 lines (100 files + truncation), got %d", len(lines))
		}
		// Last line should be the truncation message
		if lines[len(lines)-1] != tools.GlobTruncationMessage {
			t.Errorf("expected truncation message, got %q", lines[len(lines)-1])
		}
	})

	t.Run("no_truncation_under_limit", func(t *testing.T) {
		dir := t.TempDir()
		for i := 0; i < 5; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)), []byte("x"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input, _ := json.Marshal(map[string]string{"pattern": "*.txt"})
		out, _ := tool.Execute(context.Background(), tc, input)
		if strings.Contains(out.Content, tools.GlobTruncationMessage) {
			t.Error("should not have truncation message when under limit")
		}
	})

	t.Run("no_files_message", func(t *testing.T) {
		// Source: GlobTool.ts:178-183
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input, _ := json.Marshal(map[string]string{"pattern": "*.xyz"})
		out, _ := tool.Execute(context.Background(), tc, input)
		if out.Content != "No files found" {
			t.Errorf("expected 'No files found', got %q", out.Content)
		}
	})
}
