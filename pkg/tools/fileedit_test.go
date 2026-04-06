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

	t.Run("description_matches_ts", func(t *testing.T) {
		// Source: FileEditTool.ts:92 — 'A tool for editing files'
		want := "A tool for editing files"
		if got := tool.Description(); got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	t.Run("search_hint", func(t *testing.T) {
		// Source: FileEditTool.ts:88
		want := "modify file contents in place"
		if got := tool.SearchHint(); got != want {
			t.Errorf("SearchHint() = %q, want %q", got, want)
		}
	})

	t.Run("max_result_size_chars", func(t *testing.T) {
		// Source: FileEditTool.ts:89
		if got := tool.MaxResultSizeChars(); got != 100_000 {
			t.Errorf("MaxResultSizeChars() = %d, want 100000", got)
		}
	})

	t.Run("prompt_contains_usage_bullets", func(t *testing.T) {
		// Source: FileEditTool/prompt.ts — getEditToolDescription()
		prompt := tool.Prompt()
		mustContain := []string{
			"Performs exact string replacements in files.",
			"You must use your `Read` tool at least once",
			"ALWAYS prefer editing existing files",
			"Only use emojis if the user explicitly requests it",
			"The edit will FAIL if `old_string` is not unique",
			"Use `replace_all` for replacing and renaming strings",
			"line number + tab",
		}
		for _, want := range mustContain {
			if !strings.Contains(prompt, want) {
				t.Errorf("Prompt() missing: %q", want)
			}
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

	t.Run("happy_path_result_message", func(t *testing.T) {
		// Source: FileEditTool.ts:589-593 — single edit result message
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "world", "new_string": "gopher"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := fmt.Sprintf("The file %s has been updated successfully.", filePath)
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("replace_all_result_message", func(t *testing.T) {
		// Source: FileEditTool.ts:581-586 — replace_all result message
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("aa bb aa cc"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "aa", "new_string": "zz", "replace_all": true}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		want := fmt.Sprintf("The file %s has been updated. All occurrences were successfully replaced.", filePath)
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "zz bb zz cc" {
			t.Errorf("expected 'zz bb zz cc', got %q", string(data))
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

	t.Run("error_code_1_same_old_and_new_string", func(t *testing.T) {
		// Source: FileEditTool.ts:148-155
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
		want := "No changes to make: old_string and new_string are exactly the same."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_3_file_exists_empty_old", func(t *testing.T) {
		// Source: FileEditTool.ts:249-258
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
			t.Error("expected error for empty old_string on existing file")
		}
		want := "Cannot create new file - file already exists."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_4_file_not_found", func(t *testing.T) {
		// Source: FileEditTool.ts:232-245
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
		// Verbatim: "File does not exist. Please verify the path and try again. Current working directory: <cwd>."
		if !strings.HasPrefix(out.Content, "File does not exist.") {
			t.Errorf("Content should start with 'File does not exist.', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "Please verify the path and try again.") {
			t.Errorf("Content should contain CWD note, got %q", out.Content)
		}
		if !strings.Contains(out.Content, dir) {
			t.Errorf("Content should contain CWD %q, got %q", dir, out.Content)
		}
	})

	t.Run("error_code_4_new_file_creation", func(t *testing.T) {
		// Source: FileEditTool.ts:226-227 — empty old_string on nonexistent = create
		dir := t.TempDir()
		filePath := filepath.Join(dir, "new.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "brand new content"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "brand new content" {
			t.Errorf("expected 'brand new content', got %q", string(data))
		}
	})

	t.Run("error_code_5_notebook_file", func(t *testing.T) {
		// Source: FileEditTool.ts:266-273
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.ipynb")
		os.WriteFile(filePath, []byte(`{"cells":[]}`), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "cells", "new_string": "x"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for .ipynb file")
		}
		want := "File is a Jupyter Notebook. Use the NotebookEdit tool to edit this file."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_6_file_not_read_yet", func(t *testing.T) {
		// Source: FileEditTool.ts:275-287
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		rfs := tools.NewReadFileState()
		// Don't record any read — should trigger "not read yet"
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "hello", "new_string": "bye"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error when file not read yet")
		}
		want := "File has not been read yet. Read it first before writing to it."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_6_partial_view", func(t *testing.T) {
		// Source: FileEditTool.ts:276 — isPartialView check
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "hello", true) // partial view
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "hello", "new_string": "bye"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for partial view")
		}
		want := "File has not been read yet. Read it first before writing to it."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_7_file_modified_since_read", func(t *testing.T) {
		// Source: FileEditTool.ts:290-311
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("original content"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "old stale content", false) // record with different content

		// Wait a moment then modify the file so mtime is after the record
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(filePath, []byte("modified content"), 0644)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "modified", "new_string": "bye"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for modified file")
		}
		want := "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_7_content_unchanged_fallback", func(t *testing.T) {
		// Source: FileEditTool.ts:298-300 — content comparison fallback
		// When mtime is newer but content matches, edit should succeed.
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		content := "hello world"
		os.WriteFile(filePath, []byte(content), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, content, false) // record with same content

		// Touch the file to bump mtime without changing content
		time.Sleep(50 * time.Millisecond)
		os.WriteFile(filePath, []byte(content), 0644)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "world", "new_string": "gopher"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("expected success (content unchanged), got error: %s", out.Content)
		}
	})

	t.Run("error_code_8_string_not_found", func(t *testing.T) {
		// Source: FileEditTool.ts:316-327
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
		want := "String to replace not found in file.\nString: missing"
		if out.Content != want {
			t.Errorf("Content = %q, want %q", out.Content, want)
		}
	})

	t.Run("error_code_9_multiple_matches", func(t *testing.T) {
		// Source: FileEditTool.ts:332-343
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
		// Verify the verbatim message format
		if !strings.Contains(out.Content, "Found 2 matches") {
			t.Errorf("Content should mention 2 matches, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "replace_all is false") {
			t.Errorf("Content should mention replace_all, got %q", out.Content)
		}
		if !strings.Contains(out.Content, "provide more context to uniquely identify") {
			t.Errorf("Content should suggest providing more context, got %q", out.Content)
		}
	})

	t.Run("error_code_9_resolved_with_replace_all", func(t *testing.T) {
		// When replace_all=true, multiple matches should succeed
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("aa bb aa cc"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "aa", "new_string": "zz", "replace_all": true}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("expected success with replace_all, got error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "zz bb zz cc" {
			t.Errorf("expected 'zz bb zz cc', got %q", string(data))
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

	t.Run("quote_normalization", func(t *testing.T) {
		// Source: FileEditTool/utils.ts:73-93 — findActualString
		// File has curly quotes, model sends straight quotes — should match
		dir := t.TempDir()
		filePath := filepath.Join(dir, "quotes.txt")
		// File contains curly quotes: \u201Chello\u201D \u2018world\u2019
		os.WriteFile(filePath, []byte("\u201Chello\u201D \u2018world\u2019\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		// Model sends straight quotes
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "\"hello\" 'world'", "new_string": "replaced"}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("expected success with quote normalization, got error: %s", out.Content)
		}

		// Verify the file was actually edited
		data, _ := os.ReadFile(filePath)
		if string(data) != "replaced\n" {
			t.Errorf("expected 'replaced\\n', got %q", string(data))
		}
	})

	t.Run("preserve_quote_style_double", func(t *testing.T) {
		// Source: FileEditTool/utils.ts:104-136 — preserveQuoteStyle
		// When file has curly quotes, new_string should get curly quotes too
		dir := t.TempDir()
		filePath := filepath.Join(dir, "curly.txt")
		os.WriteFile(filePath, []byte("\u201Chello world\u201D\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "\"hello world\"", "new_string": "\"goodbye world\""}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		content := string(data)
		// Should have curly quotes in the result
		if !strings.Contains(content, "\u201C") || !strings.Contains(content, "\u201D") {
			t.Errorf("expected curly quotes preserved, got %q", content)
		}
	})

	t.Run("preserve_quote_style_single_contraction", func(t *testing.T) {
		// Source: FileEditTool/utils.ts:180-186 — apostrophe in contraction
		dir := t.TempDir()
		filePath := filepath.Join(dir, "contraction.txt")
		os.WriteFile(filePath, []byte("\u2018don\u2019t stop\u2019\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "'don't stop'", "new_string": "'can't stop'"}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		content := string(data)
		// The apostrophe in "can't" should be right single curly
		if !strings.Contains(content, "\u2019") {
			t.Errorf("expected curly apostrophe preserved, got %q", content)
		}
	})

	t.Run("deletion_strips_trailing_newline", func(t *testing.T) {
		// Source: FileEditTool/utils.ts:222-227 — applyEditToFile
		dir := t.TempDir()
		filePath := filepath.Join(dir, "del.txt")
		os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "line2", "new_string": ""}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "line1\nline3\n" {
			t.Errorf("expected 'line1\\nline3\\n', got %q", string(data))
		}
	})

	t.Run("deletion_replace_all", func(t *testing.T) {
		// Delete all occurrences with replace_all
		dir := t.TempDir()
		filePath := filepath.Join(dir, "del.txt")
		os.WriteFile(filePath, []byte("aa\nbb\naa\ncc\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "aa", "new_string": "", "replace_all": true}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		// Both "aa\n" lines stripped
		if string(data) != "bb\ncc\n" {
			t.Errorf("expected 'bb\\ncc\\n', got %q", string(data))
		}
	})

	t.Run("max_file_size_1gib", func(t *testing.T) {
		if tools.MaxEditFileSize != 1024*1024*1024 {
			t.Errorf("MaxEditFileSize = %d, want 1 GiB (%d)", tools.MaxEditFileSize, 1024*1024*1024)
		}
	})

	t.Run("crlf_normalization", func(t *testing.T) {
		// Source: FileEditTool.ts:214 — .replaceAll('\r\n', '\n')
		dir := t.TempDir()
		filePath := filepath.Join(dir, "crlf.txt")
		// Write file with \r\n line endings
		os.WriteFile(filePath, []byte("hello\r\nworld\r\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		// Model sends with \n (normalized)
		input := json.RawMessage(fmt.Sprintf(
			`{"file_path": %q, "old_string": "hello\nworld", "new_string": "goodbye\nworld"}`,
			filePath,
		))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("expected success with CRLF normalization, got error: %s", out.Content)
		}
	})

	t.Run("new_file_in_nested_dir", func(t *testing.T) {
		// Creating a new file in a directory that doesn't exist yet
		dir := t.TempDir()
		filePath := filepath.Join(dir, "sub", "deep", "new.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "nested content"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "nested content" {
			t.Errorf("expected 'nested content', got %q", string(data))
		}
	})

	t.Run("empty_file_with_empty_old", func(t *testing.T) {
		// Empty file + empty old_string should write new content
		dir := t.TempDir()
		filePath := filepath.Join(dir, "empty.txt")
		os.WriteFile(filePath, []byte(""), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "new content"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		data, _ := os.ReadFile(filePath)
		if string(data) != "new content" {
			t.Errorf("expected 'new content', got %q", string(data))
		}
	})

	t.Run("whitespace_only_file_with_empty_old", func(t *testing.T) {
		// Whitespace-only file + empty old_string should write new content
		dir := t.TempDir()
		filePath := filepath.Join(dir, "ws.txt")
		os.WriteFile(filePath, []byte("   \n  \n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "new content"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
	})

	t.Run("preserves_file_mode", func(t *testing.T) {
		// Edit should preserve the original file permissions
		dir := t.TempDir()
		filePath := filepath.Join(dir, "executable.sh")
		os.WriteFile(filePath, []byte("#!/bin/bash\necho old"), 0755)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "echo old", "new_string": "echo new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		info, _ := os.Stat(filePath)
		if info.Mode().Perm()&0111 == 0 {
			t.Error("expected executable permission preserved")
		}
	})

	t.Run("read_file_state_updated_after_edit", func(t *testing.T) {
		// After a successful edit, ReadFileState should be updated
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "hello world", false)
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "world", "new_string": "gopher"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// ReadFileState should now contain the updated content
		entry := rfs.Get(filePath)
		if entry == nil {
			t.Fatal("expected ReadFileState entry after edit")
		}
		if entry.Content != "hello gopher" {
			t.Errorf("ReadFileState content = %q, want 'hello gopher'", entry.Content)
		}
		if entry.IsPartialView {
			t.Error("expected IsPartialView=false after edit")
		}
	})

	t.Run("read_file_state_updated_after_create", func(t *testing.T) {
		// After a successful file creation, ReadFileState should be updated
		dir := t.TempDir()
		filePath := filepath.Join(dir, "new.txt")

		rfs := tools.NewReadFileState()
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "", "new_string": "brand new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		entry := rfs.Get(filePath)
		if entry == nil {
			t.Fatal("expected ReadFileState entry after create")
		}
		if entry.Content != "brand new" {
			t.Errorf("ReadFileState content = %q, want 'brand new'", entry.Content)
		}
	})

	t.Run("staleness_guard_bypass_without_read_file_state", func(t *testing.T) {
		// When no ReadFileState is set, staleness guard is skipped
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello world"), 0644)

		tc := &tools.ToolContext{CWD: dir} // no ReadFileState
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "old_string": "world", "new_string": "gopher"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("expected success without ReadFileState, got error: %s", out.Content)
		}
	})
}

func TestStripTrailingWhitespace(t *testing.T) {
	// Source: FileEditTool/utils.ts:44-64
	tests := []struct {
		name  string
		input string
		want  string
	}{
		{"no_trailing", "hello\nworld", "hello\nworld"},
		{"trailing_spaces", "hello   \nworld  ", "hello\nworld"},
		{"trailing_tabs", "hello\t\t\nworld\t", "hello\nworld"},
		{"mixed", "hello   \n  world  \n  ", "hello\n  world\n"},
		{"crlf", "hello  \r\nworld  \r\n", "hello\r\nworld\r\n"},
		{"empty", "", ""},
		{"only_whitespace_line", "  \n", "\n"},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := tools.StripTrailingWhitespace(tt.input)
			if got != tt.want {
				t.Errorf("StripTrailingWhitespace(%q) = %q, want %q", tt.input, got, tt.want)
			}
		})
	}
}

func TestNormalizeQuotes(t *testing.T) {
	// Source: FileEditTool/utils.ts:31-37
	input := "\u201Chello\u201D \u2018world\u2019"
	want := `"hello" 'world'`
	got := tools.NormalizeQuotes(input)
	if got != want {
		t.Errorf("NormalizeQuotes(%q) = %q, want %q", input, got, want)
	}
}

func TestPreserveQuoteStyle(t *testing.T) {
	// Source: FileEditTool/utils.ts:104-136
	t.Run("no_normalization_needed", func(t *testing.T) {
		got := tools.PreserveQuoteStyle("hello", "hello", "goodbye")
		if got != "goodbye" {
			t.Errorf("expected 'goodbye', got %q", got)
		}
	})

	t.Run("double_quotes", func(t *testing.T) {
		got := tools.PreserveQuoteStyle(
			`"hello"`,
			"\u201Chello\u201D",
			`"goodbye"`,
		)
		if !strings.Contains(got, "\u201C") || !strings.Contains(got, "\u201D") {
			t.Errorf("expected curly double quotes, got %q", got)
		}
	})

	t.Run("single_quotes", func(t *testing.T) {
		got := tools.PreserveQuoteStyle(
			"'hello'",
			"\u2018hello\u2019",
			"'goodbye'",
		)
		if !strings.Contains(got, "\u2018") || !strings.Contains(got, "\u2019") {
			t.Errorf("expected curly single quotes, got %q", got)
		}
	})
}

func TestApplyEditToFile(t *testing.T) {
	// Source: FileEditTool/utils.ts:206-228
	t.Run("simple_replace", func(t *testing.T) {
		got := tools.ApplyEditToFile("hello world", "world", "gopher", false)
		if got != "hello gopher" {
			t.Errorf("expected 'hello gopher', got %q", got)
		}
	})

	t.Run("replace_all", func(t *testing.T) {
		got := tools.ApplyEditToFile("aa bb aa cc", "aa", "zz", true)
		if got != "zz bb zz cc" {
			t.Errorf("expected 'zz bb zz cc', got %q", got)
		}
	})

	t.Run("deletion_strips_newline", func(t *testing.T) {
		got := tools.ApplyEditToFile("line1\nline2\nline3\n", "line2", "", false)
		if got != "line1\nline3\n" {
			t.Errorf("expected 'line1\\nline3\\n', got %q", got)
		}
	})

	t.Run("deletion_no_trailing_newline", func(t *testing.T) {
		got := tools.ApplyEditToFile("line1\nline2", "line2", "", false)
		if got != "line1\n" {
			t.Errorf("expected 'line1\\n', got %q", got)
		}
	})
}

func TestFindActualString(t *testing.T) {
	// Source: FileEditTool/utils.ts:73-93
	t.Run("exact_match", func(t *testing.T) {
		got := tools.FindActualString("hello world", "world")
		if got != "world" {
			t.Errorf("expected 'world', got %q", got)
		}
	})

	t.Run("quote_normalized_match", func(t *testing.T) {
		file := "\u201Chello\u201D"
		got := tools.FindActualString(file, `"hello"`)
		if got != "\u201Chello\u201D" {
			t.Errorf("expected curly-quoted string, got %q", got)
		}
	})

	t.Run("no_match", func(t *testing.T) {
		got := tools.FindActualString("hello world", "missing")
		if got != "" {
			t.Errorf("expected empty, got %q", got)
		}
	})
}
