package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"
	"time"

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

	t.Run("description_has_five_bullets", func(t *testing.T) {
		// Source: prompt.ts — 5-bullet DESCRIPTION
		desc := tool.Description()
		lines := strings.Split(desc, "\n")
		if len(lines) != 5 {
			t.Errorf("expected 5 description bullets, got %d: %q", len(lines), desc)
		}
		if !strings.Contains(desc, "sorted by modification time") {
			t.Error("description must mention mtime sorting")
		}
		if !strings.Contains(desc, "Agent tool") {
			t.Error("description must mention Agent tool delegation")
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

	t.Run("doublestar_pattern_recursive", func(t *testing.T) {
		// ** patterns must match across directory boundaries
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "a", "b", "c"), 0755)
		os.WriteFile(filepath.Join(dir, "top.go"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "a", "mid.go"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "a", "b", "c", "deep.go"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "a", "b", "skip.txt"), []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "**/*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "top.go") {
			t.Errorf("expected top.go, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "mid.go") {
			t.Errorf("expected mid.go, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "deep.go") {
			t.Errorf("expected deep.go, got %q", out.Content)
		}
		if strings.Contains(out.Content, "skip.txt") {
			t.Errorf("did not expect skip.txt, got %q", out.Content)
		}
	})

	t.Run("doublestar_prefix_pattern", func(t *testing.T) {
		// src/**/*.ts should match only under src/
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "src", "lib"), 0755)
		os.MkdirAll(filepath.Join(dir, "other"), 0755)
		os.WriteFile(filepath.Join(dir, "src", "index.ts"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "src", "lib", "util.ts"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "other", "nope.ts"), []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "src/**/*.ts"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "index.ts") {
			t.Errorf("expected index.ts, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "util.ts") {
			t.Errorf("expected util.ts, got %q", out.Content)
		}
		if strings.Contains(out.Content, "nope.ts") {
			t.Errorf("should not include other/nope.ts, got %q", out.Content)
		}
	})

	t.Run("with_subdirectory", func(t *testing.T) {
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "sub"), 0755)
		os.WriteFile(filepath.Join(dir, "sub", "deep.go"), []byte("package sub"), 0644)
		os.WriteFile(filepath.Join(dir, "top.go"), []byte("package main"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		// Use ** pattern to match in subdirs
		input := json.RawMessage(`{"pattern": "**/*.go"}`)
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
		input := json.RawMessage(`{"pattern": "**/*.txt"}`)
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

	t.Run("gitignore_excludes_files", func(t *testing.T) {
		// .gitignore patterns should be respected
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".gitignore"), []byte("node_modules/\n*.log\n"), 0644)
		os.MkdirAll(filepath.Join(dir, "node_modules", "pkg"), 0755)
		os.WriteFile(filepath.Join(dir, "node_modules", "pkg", "index.js"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "app.js"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "debug.log"), []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "**/*"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "index.js") {
			t.Errorf("should exclude node_modules via .gitignore, got %q", out.Content)
		}
		if strings.Contains(out.Content, "debug.log") {
			t.Errorf("should exclude *.log via .gitignore, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "app.js") {
			t.Errorf("expected app.js, got %q", out.Content)
		}
	})

	t.Run("claudeignore_excludes_files", func(t *testing.T) {
		// .claudeignore patterns should also be respected
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, ".claudeignore"), []byte("secret/\n*.env\n"), 0644)
		os.MkdirAll(filepath.Join(dir, "secret"), 0755)
		os.WriteFile(filepath.Join(dir, "secret", "key.txt"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "config.env"), []byte("x"), 0644)
		os.WriteFile(filepath.Join(dir, "main.go"), []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "**/*"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if strings.Contains(out.Content, "key.txt") {
			t.Errorf("should exclude secret/ via .claudeignore, got %q", out.Content)
		}
		if strings.Contains(out.Content, "config.env") {
			t.Errorf("should exclude *.env via .claudeignore, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "main.go") {
			t.Errorf("expected main.go, got %q", out.Content)
		}
	})

	t.Run("mtime_sorting_newest_first", func(t *testing.T) {
		// Results must be sorted by modification time, newest first.
		// Source: prompt.ts — "sorted by modification time"
		dir := t.TempDir()

		// Create files with distinct mtimes (oldest to newest).
		base := time.Now().Add(-3 * time.Second)
		for i, name := range []string{"oldest.go", "middle.go", "newest.go"} {
			p := filepath.Join(dir, name)
			os.WriteFile(p, []byte("x"), 0644)
			mtime := base.Add(time.Duration(i) * time.Second)
			os.Chtimes(p, mtime, mtime)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		lines := strings.Split(strings.TrimSpace(out.Content), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 results, got %d: %v", len(lines), lines)
		}
		// Newest first
		if lines[0] != "newest.go" {
			t.Errorf("expected newest.go first, got %q", lines[0])
		}
		if lines[1] != "middle.go" {
			t.Errorf("expected middle.go second, got %q", lines[1])
		}
		if lines[2] != "oldest.go" {
			t.Errorf("expected oldest.go third, got %q", lines[2])
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

	t.Run("nonexistent_path_error_message", func(t *testing.T) {
		// Source: GlobTool.ts errorCode 1 — "Directory does not exist: ..."
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
		if !strings.Contains(out.Content, "Directory does not exist") {
			t.Errorf("expected 'Directory does not exist' message, got %q", out.Content)
		}
	})

	t.Run("path_is_file_error", func(t *testing.T) {
		// Source: GlobTool.ts errorCode 2 — "Path is not a directory: ..."
		dir := t.TempDir()
		filePath := filepath.Join(dir, "afile.txt")
		os.WriteFile(filePath, []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "*.go", "path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when path is a file")
		}
		if !strings.Contains(out.Content, "Path is not a directory") {
			t.Errorf("expected 'Path is not a directory' message, got %q", out.Content)
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

	t.Run("input_schema_valid_json", func(t *testing.T) {
		schema := tool.InputSchema()
		var parsed map[string]interface{}
		if err := json.Unmarshal(schema, &parsed); err != nil {
			t.Fatalf("InputSchema() is not valid JSON: %v", err)
		}
	})

	t.Run("input_schema_path_description", func(t *testing.T) {
		// Source: GlobTool.ts — path field must include IMPORTANT guidance
		schema := tool.InputSchema()
		raw := string(schema)
		if !strings.Contains(raw, "IMPORTANT") {
			t.Error("path description must include IMPORTANT guidance")
		}
		if !strings.Contains(raw, "DO NOT enter") {
			t.Error("path description must include 'DO NOT enter' warning")
		}
	})

	t.Run("truncates_at_100_results", func(t *testing.T) {
		// Source: GlobTool.ts:157 — default limit 100
		// Source: GlobTool.ts:190-193 — truncation message
		dir := t.TempDir()
		// Create 120 files with distinct mtimes
		base := time.Now().Add(-120 * time.Second)
		for i := 0; i < 120; i++ {
			p := filepath.Join(dir, fmt.Sprintf("file_%03d.txt", i))
			os.WriteFile(p, []byte("x"), 0644)
			mtime := base.Add(time.Duration(i) * time.Second)
			os.Chtimes(p, mtime, mtime)
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

	t.Run("relative_paths_in_output", func(t *testing.T) {
		// Output paths must be relative to searchPath, not absolute.
		// Source: GlobTool.ts — toRelativePath
		dir := t.TempDir()
		os.MkdirAll(filepath.Join(dir, "sub"), 0755)
		os.WriteFile(filepath.Join(dir, "sub", "file.go"), []byte("x"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "**/*.go"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		content := strings.TrimSpace(out.Content)
		// Must be relative, e.g. "sub/file.go", not absolute
		if filepath.IsAbs(content) {
			t.Errorf("output paths should be relative, got %q", content)
		}
		// Should contain the relative path with separator
		if !strings.Contains(content, filepath.Join("sub", "file.go")) {
			t.Errorf("expected sub/file.go, got %q", content)
		}
	})

	t.Run("one_path_per_line", func(t *testing.T) {
		dir := t.TempDir()
		base := time.Now()
		for i, name := range []string{"a.go", "b.go", "c.go"} {
			p := filepath.Join(dir, name)
			os.WriteFile(p, []byte("x"), 0644)
			mtime := base.Add(time.Duration(i) * time.Second)
			os.Chtimes(p, mtime, mtime)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "*.go"}`)
		out, _ := tool.Execute(context.Background(), tc, input)
		lines := strings.Split(strings.TrimSpace(out.Content), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines (one per file), got %d", len(lines))
		}
	})
}
