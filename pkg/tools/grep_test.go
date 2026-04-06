package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/tools"
)

func requireRg(t *testing.T) {
	t.Helper()
	if _, err := exec.LookPath("rg"); err != nil {
		t.Skip("rg (ripgrep) not installed, skipping rg-specific test")
	}
}

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

	t.Run("search_hint", func(t *testing.T) {
		// Source: GrepTool.ts:162
		hint := tool.SearchHint()
		if hint != "search file contents with regex (ripgrep)" {
			t.Errorf("unexpected search hint: %q", hint)
		}
	})

	t.Run("concurrency_safe", func(t *testing.T) {
		// Source: GrepTool.ts:184-186
		if !tool.IsConcurrencySafe(nil) {
			t.Error("GrepTool should be concurrency-safe")
		}
	})

	t.Run("description_has_usage_bullets", func(t *testing.T) {
		// Source: prompt.ts:6-18
		desc := tool.Description()
		expected := []string{
			"A powerful search tool built on ripgrep",
			"ALWAYS use Grep for search tasks",
			"NEVER invoke `grep` or `rg` as a Bash command",
			"Supports full regex syntax",
			`"content" shows matching lines`,
			"Use Agent tool for open-ended searches",
			"literal braces need escaping",
			"`multiline: true`",
		}
		for _, s := range expected {
			if !strings.Contains(desc, s) {
				t.Errorf("description missing %q", s)
			}
		}
	})

	t.Run("prompt_matches_description", func(t *testing.T) {
		// Source: GrepTool.ts:241-243
		if tool.Prompt() != tool.Description() {
			t.Error("Prompt() should match Description()")
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
		// Default mode is files_with_matches — returns header + filenames
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
		// Source: GrepTool.ts:478-530 — count mode shows file:count + summary
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
		// Source: GrepTool.ts:285 — count summary
		if !strings.Contains(out.Content, "Found 2 total occurrences across 1 file.") {
			t.Errorf("expected count summary, got %q", out.Content)
		}
	})

	t.Run("count_mode_singular", func(t *testing.T) {
		// Source: GrepTool.ts:285 — plural helper
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "one.txt"), []byte("match\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "match", "path": %q, "output_mode": "count"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "1 total occurrence across 1 file.") {
			t.Errorf("expected singular count summary, got %q", out.Content)
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
		// Source: GrepTool.ts:295-300 — files_with_matches says "No files found"
		if !strings.Contains(out.Content, "No files found") {
			t.Errorf("expected 'No files found', got %q", out.Content)
		}
	})

	t.Run("no_matches_content_mode", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "file.txt"), []byte("hello\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "zzz_no_match", "path": %q, "output_mode": "content"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Source: GrepTool.ts:269 — content mode says "No matches found"
		if !strings.Contains(out.Content, "No matches found") {
			t.Errorf("expected 'No matches found', got %q", out.Content)
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

	t.Run("path_does_not_exist", func(t *testing.T) {
		// Source: GrepTool.ts:216-218 — errorCode 1
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(`{"pattern": "hello", "path": "/nonexistent/path/xyz"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for nonexistent path")
		}
		if !strings.Contains(out.Content, "Path does not exist") {
			t.Errorf("expected 'Path does not exist' message, got %q", out.Content)
		}
	})

	// --- New tests for T516 ---

	t.Run("context_lines_A", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:363-376 — context after
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte("line1\nMATCH\nafter1\nafter2\nline5\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "MATCH", "path": %q, "output_mode": "content", "-A": 2}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "MATCH") {
			t.Errorf("missing match line, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "after1") {
			t.Errorf("missing context-after line 'after1', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "after2") {
			t.Errorf("missing context-after line 'after2', got %q", out.Content)
		}
	})

	t.Run("context_lines_B", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:363-376 — context before
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte("before1\nbefore2\nMATCH\nline4\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "MATCH", "path": %q, "output_mode": "content", "-B": 2}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "before1") {
			t.Errorf("missing context-before line 'before1', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "before2") {
			t.Errorf("missing context-before line 'before2', got %q", out.Content)
		}
	})

	t.Run("context_lines_C", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:363-376 — -C context both sides
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte("b1\nb2\nMATCH\na1\na2\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "MATCH", "path": %q, "output_mode": "content", "-C": 1}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "b2") {
			t.Errorf("missing context-before line 'b2', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "a1") {
			t.Errorf("missing context-after line 'a1', got %q", out.Content)
		}
	})

	t.Run("context_keyword_takes_precedence", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:364 — `context` takes precedence over -C/-B/-A
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "ctx.txt"), []byte("b1\nb2\nMATCH\na1\na2\nfar\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "MATCH", "path": %q, "output_mode": "content", "context": 1, "-A": 3}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		// context=1 wins over -A=3, so "far" should NOT appear
		if strings.Contains(out.Content, "far") {
			t.Errorf("context should override -A, but 'far' appeared: %q", out.Content)
		}
	})

	t.Run("files_with_matches_header", func(t *testing.T) {
		// Source: GrepTool.ts:302-304 — "Found N file(s)\n..."
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.txt"), []byte("hello\n"), 0644)
		os.WriteFile(filepath.Join(dir, "b.txt"), []byte("hello\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "hello", "path": %q}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Found 2 files") {
			t.Errorf("expected 'Found 2 files' header, got %q", out.Content)
		}
	})

	t.Run("files_with_matches_singular", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "solo.txt"), []byte("hello\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "hello", "path": %q}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.Contains(out.Content, "Found 1 file\n") {
			t.Errorf("expected 'Found 1 file' (singular), got %q", out.Content)
		}
	})

	t.Run("gitignore_exclusion_vcs_dirs", func(t *testing.T) {
		// Source: GrepTool.ts:332-335 — VCS directories excluded
		dir := t.TempDir()
		// Create a .git directory with a file that matches
		os.MkdirAll(filepath.Join(dir, ".git"), 0755)
		os.WriteFile(filepath.Join(dir, ".git", "config"), []byte("searchable\n"), 0644)
		os.WriteFile(filepath.Join(dir, "real.txt"), []byte("searchable\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"pattern": "searchable", "path": %q}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "real.txt") {
			t.Errorf("expected real.txt, got %q", out.Content)
		}
		// .git/config should be excluded
		if strings.Contains(out.Content, ".git") {
			t.Errorf(".git directory should be excluded, got %q", out.Content)
		}
	})

	t.Run("head_limit_truncation", func(t *testing.T) {
		// Source: GrepTool.ts:450-454, 556-560 — head_limit with truncation
		dir := t.TempDir()
		// Create 5 files
		for i := 0; i < 5; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)),
				[]byte("needle\n"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "needle", "path": %q, "head_limit": 2}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		// Should show exactly 2 files and indicate truncation
		if !strings.Contains(out.Content, "Found 2 files") {
			t.Errorf("expected 'Found 2 files', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "limit: 2") {
			t.Errorf("expected 'limit: 2' pagination info, got %q", out.Content)
		}
	})

	t.Run("head_limit_with_offset", func(t *testing.T) {
		// Source: GrepTool.ts:556-560 — offset pagination
		dir := t.TempDir()
		for i := 0; i < 5; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)),
				[]byte("needle\n"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "needle", "path": %q, "head_limit": 2, "offset": 2}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "offset: 2") {
			t.Errorf("expected 'offset: 2' in pagination info, got %q", out.Content)
		}
	})

	t.Run("head_limit_zero_unlimited", func(t *testing.T) {
		// Source: GrepTool.ts:116-118 — 0 means unlimited
		dir := t.TempDir()
		for i := 0; i < 300; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%03d.txt", i)),
				[]byte("needle\n"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "needle", "path": %q, "head_limit": 0}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		// With head_limit=0 (unlimited), should show all 300
		if !strings.Contains(out.Content, "Found 300 files") {
			t.Errorf("expected all 300 files with head_limit=0, got %q", out.Content)
		}
		// Should NOT have truncation info
		if strings.Contains(out.Content, "limit:") {
			t.Errorf("head_limit=0 should not show limit info, got %q", out.Content)
		}
	})

	t.Run("content_pagination_suffix", func(t *testing.T) {
		// Source: GrepTool.ts:271-273 — "[Showing results with pagination = ...]"
		dir := t.TempDir()
		var lines []string
		for i := 0; i < 10; i++ {
			lines = append(lines, fmt.Sprintf("match line %d", i))
		}
		os.WriteFile(filepath.Join(dir, "big.txt"), []byte(strings.Join(lines, "\n")+"\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "match line", "path": %q, "output_mode": "content", "head_limit": 3}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "[Showing results with pagination = (limit: 3)]") {
			t.Errorf("expected content pagination suffix, got %q", out.Content)
		}
	})

	t.Run("case_insensitive", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "mixed.txt"), []byte("Hello\nworld\nHELLO\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "hello", "path": %q, "output_mode": "content", "-i": true}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "Hello") || !strings.Contains(out.Content, "HELLO") {
			t.Errorf("case-insensitive search should find both, got %q", out.Content)
		}
	})

	t.Run("dash_prefixed_pattern", func(t *testing.T) {
		// Source: GrepTool.ts:379-384 — patterns starting with dash use -e flag
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "opts.txt"), []byte("-verbose flag\nnormal line\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "-verbose", "path": %q, "output_mode": "content"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "-verbose") {
			t.Errorf("expected match for dash-prefixed pattern, got %q", out.Content)
		}
	})

	t.Run("brace_glob_preserved", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:392-409 — brace patterns preserved
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "a.ts"), []byte("hello\n"), 0644)
		os.WriteFile(filepath.Join(dir, "b.tsx"), []byte("hello\n"), 0644)
		os.WriteFile(filepath.Join(dir, "c.js"), []byte("hello\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "hello", "path": %q, "glob": "*.{ts,tsx}"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		// Should match .ts and .tsx but not .js
		if strings.Contains(out.Content, "c.js") {
			t.Errorf("brace glob should exclude .js files, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "a.ts") {
			t.Errorf("expected a.ts in results, got %q", out.Content)
		}
	})

	t.Run("multiline_search", func(t *testing.T) {
		requireRg(t)
		// Source: GrepTool.ts:341-343 — multiline mode
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "multi.txt"), []byte("func hello(\n  world int,\n)\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "hello\\(\\n\\s+world", "path": %q, "output_mode": "content", "multiline": true}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "hello") {
			t.Errorf("expected multiline match, got %q", out.Content)
		}
	})

	t.Run("count_with_pagination", func(t *testing.T) {
		// Source: GrepTool.ts:280-290 — count summary with pagination
		dir := t.TempDir()
		for i := 0; i < 5; i++ {
			os.WriteFile(filepath.Join(dir, fmt.Sprintf("f%d.txt", i)),
				[]byte("needle\nneedle\n"), 0644)
		}

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "needle", "path": %q, "output_mode": "count", "head_limit": 2}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "with pagination = (limit: 2)") {
			t.Errorf("expected count pagination info, got %q", out.Content)
		}
	})

	t.Run("line_numbers_default_on", func(t *testing.T) {
		// Source: GrepTool.ts:358-360 — line numbers default true in content mode
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "nums.txt"), []byte("first\nsecond\nthird\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "second", "path": %q, "output_mode": "content"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Line numbers should be present — "2:second" format
		if !strings.Contains(out.Content, "2:second") {
			t.Errorf("expected line number in output, got %q", out.Content)
		}
	})

	t.Run("line_numbers_disabled", func(t *testing.T) {
		// Source: GrepTool.ts:358-360 — -n=false disables line numbers
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "nums.txt"), []byte("first\nsecond\nthird\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "second", "path": %q, "output_mode": "content", "-n": false}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		// Without line numbers, should NOT have "2:" prefix
		if strings.Contains(out.Content, "2:second") {
			t.Errorf("line numbers should be disabled, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "second") {
			t.Errorf("should still contain the match, got %q", out.Content)
		}
	})

	t.Run("type_filter", func(t *testing.T) {
		requireRg(t)
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "code.go"), []byte("package main\n"), 0644)
		os.WriteFile(filepath.Join(dir, "data.py"), []byte("package = True\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"pattern": "package", "path": %q, "type": "go"}`, dir))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("tool error: %s", out.Content)
		}
		if !strings.Contains(out.Content, "code.go") {
			t.Errorf("expected code.go, got %q", out.Content)
		}
		if strings.Contains(out.Content, "data.py") {
			t.Errorf("type=go should exclude .py, got %q", out.Content)
		}
	})
}

// TestGrepToolNative forces the native (non-rg) fallback path.
func TestGrepToolNative(t *testing.T) {
	tool := &tools.GrepTool{}

	t.Run("regex_pattern", func(t *testing.T) {
		dir := t.TempDir()
		os.WriteFile(filepath.Join(dir, "data.txt"), []byte("abc123\ndef456\nabc789\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
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

func TestApplyGrepHeadLimit(t *testing.T) {
	t.Run("default_250", func(t *testing.T) {
		// When head_limit is nil, default 250 applies
		items := make([]string, 300)
		for i := range items {
			items[i] = fmt.Sprintf("item%d", i)
		}
		result, info := tools.ApplyGrepHeadLimitForTest(items, nil, 0)
		if len(result) != 250 {
			t.Errorf("expected 250 items, got %d", len(result))
		}
		if !strings.Contains(info, "limit: 250") {
			t.Errorf("expected limit info, got %q", info)
		}
	})

	t.Run("zero_unlimited", func(t *testing.T) {
		items := make([]string, 300)
		for i := range items {
			items[i] = fmt.Sprintf("item%d", i)
		}
		zero := 0
		result, info := tools.ApplyGrepHeadLimitForTest(items, &zero, 0)
		if len(result) != 300 {
			t.Errorf("expected all 300 items, got %d", len(result))
		}
		if info != "" {
			t.Errorf("expected no limit info for unlimited, got %q", info)
		}
	})

	t.Run("no_truncation_no_limit_info", func(t *testing.T) {
		// Source: GrepTool.ts:123-127 — only report appliedLimit on actual truncation
		items := []string{"a", "b", "c"}
		five := 5
		result, info := tools.ApplyGrepHeadLimitForTest(items, &five, 0)
		if len(result) != 3 {
			t.Errorf("expected 3 items, got %d", len(result))
		}
		if info != "" {
			t.Errorf("no truncation should mean no limit info, got %q", info)
		}
	})

	t.Run("offset_with_limit", func(t *testing.T) {
		items := []string{"a", "b", "c", "d", "e"}
		two := 2
		result, info := tools.ApplyGrepHeadLimitForTest(items, &two, 1)
		if len(result) != 2 {
			t.Errorf("expected 2 items, got %d", len(result))
		}
		if result[0] != "b" || result[1] != "c" {
			t.Errorf("expected [b, c], got %v", result)
		}
		if !strings.Contains(info, "limit: 2") || !strings.Contains(info, "offset: 1") {
			t.Errorf("expected limit and offset info, got %q", info)
		}
	})
}

func TestSplitGlobPatterns(t *testing.T) {
	tests := []struct {
		input    string
		expected []string
	}{
		{"*.go", []string{"*.go"}},
		{"*.go *.ts", []string{"*.go", "*.ts"}},
		{"*.{ts,tsx}", []string{"*.{ts,tsx}"}},
		{"*.go,*.ts", []string{"*.go", "*.ts"}},
		{"*.{ts,tsx} *.go", []string{"*.{ts,tsx}", "*.go"}},
	}
	for _, tt := range tests {
		t.Run(tt.input, func(t *testing.T) {
			got := tools.SplitGlobPatternsForTest(tt.input)
			if len(got) != len(tt.expected) {
				t.Fatalf("expected %v, got %v", tt.expected, got)
			}
			for i := range got {
				if got[i] != tt.expected[i] {
					t.Errorf("index %d: expected %q, got %q", i, tt.expected[i], got[i])
				}
			}
		})
	}
}

func TestPlural(t *testing.T) {
	if tools.PluralForTest(1, "file") != "file" {
		t.Error("1 should be singular")
	}
	if tools.PluralForTest(2, "file") != "files" {
		t.Error("2 should be plural")
	}
	if tools.PluralForTest(0, "file") != "files" {
		t.Error("0 should be plural")
	}
}
