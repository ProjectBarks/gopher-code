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

// mockFileHistory records TrackEdit calls for testing.
type mockFileHistory struct {
	calls []fileHistoryCall
}

type fileHistoryCall struct {
	Path    string
	Content string
}

func (m *mockFileHistory) TrackEdit(path string, content string) error {
	m.calls = append(m.calls, fileHistoryCall{Path: path, Content: content})
	return nil
}

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

	// Source: FileWriteTool.ts:100 — verbatim description
	t.Run("description_verbatim", func(t *testing.T) {
		want := "Write a file to the local filesystem."
		if got := tool.Description(); got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	// Source: FileWriteTool.ts:96 — searchHint
	t.Run("search_hint", func(t *testing.T) {
		want := "create or overwrite files"
		if got := tool.SearchHint(); got != want {
			t.Errorf("SearchHint() = %q, want %q", got, want)
		}
	})

	// Source: FileWriteTool.ts:97 — maxResultSizeChars: 100_000
	t.Run("max_result_size_chars", func(t *testing.T) {
		want := 100_000
		if got := tool.MaxResultSizeChars(); got != want {
			t.Errorf("MaxResultSizeChars() = %d, want %d", got, want)
		}
	})

	// Source: prompt.ts:10-17 — verbatim prompt template
	t.Run("prompt_verbatim", func(t *testing.T) {
		prompt := tool.Prompt()
		// Check key bullets are present verbatim.
		mustContain := []string{
			"Writes a file to the local filesystem.",
			"This tool will overwrite the existing file if there is one at the provided path.",
			"you MUST use the Read tool first to read the file's contents",
			"Prefer the Edit tool for modifying existing files",
			"NEVER create documentation files (*.md) or README files unless explicitly requested by the User.",
			"Only use emojis if the user explicitly requests it. Avoid writing emojis to files unless asked.",
		}
		for _, s := range mustContain {
			if !strings.Contains(prompt, s) {
				t.Errorf("Prompt() missing substring: %q", s)
			}
		}
	})

	t.Run("happy_path_creates_new_file", func(t *testing.T) {
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

	// Source: FileWriteTool.ts:421-424 — create result message
	t.Run("create_result_message_verbatim", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "new.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "data"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		want := fmt.Sprintf("File created successfully at: %s", filePath)
		if out.Content != want {
			t.Errorf("create message = %q, want %q", out.Content, want)
		}
	})

	// Source: FileWriteTool.ts:428-431 — update result message
	t.Run("update_result_message_verbatim", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old"), 0644)

		// Set up ReadFileState so overwrite is allowed.
		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "old", false)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		want := fmt.Sprintf("The file %s has been updated successfully.", filePath)
		if out.Content != want {
			t.Errorf("update message = %q, want %q", out.Content, want)
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

	// Source: FileWriteTool.ts:254 — mkdir before write
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

	// Source: FileWriteTool.ts:198-205 — file not read yet
	t.Run("overwrite_requires_prior_read", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old"), 0644)

		rfs := tools.NewReadFileState()
		// Don't record the file — simulates it was never read.
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for unread file overwrite")
		}
		want := "File has not been read yet. Read it first before writing to it."
		if out.Content != want {
			t.Errorf("error = %q, want %q", out.Content, want)
		}
	})

	// Source: FileWriteTool.ts:198-205 — partial read blocks overwrite
	t.Run("overwrite_rejects_partial_read", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old content"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "old", true) // partial read
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for partial-read file overwrite")
		}
		want := "File has not been read yet. Read it first before writing to it."
		if out.Content != want {
			t.Errorf("error = %q, want %q", out.Content, want)
		}
	})

	// Source: FileWriteTool.ts:211-218 — stale mtime
	t.Run("overwrite_rejects_stale_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "stale.txt")
		os.WriteFile(filePath, []byte("original"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "original", false)

		// Simulate file modification after read: touch with future mtime.
		time.Sleep(10 * time.Millisecond)
		os.WriteFile(filePath, []byte("modified by linter"), 0644)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for stale file overwrite")
		}
		want := "File has been modified since read, either by the user or by a linter. Read it again before attempting to write it."
		if out.Content != want {
			t.Errorf("error = %q, want %q", out.Content, want)
		}
	})

	t.Run("overwrites_existing_file_with_readfilestate", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "old", false)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
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

	// Source: FileWriteTool.ts:255-264 — fileHistoryTrackEdit backup before overwrite
	t.Run("file_history_checkpoint_before_overwrite", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "tracked.txt")
		os.WriteFile(filePath, []byte("original content"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "original content", false)

		history := &mockFileHistory{}
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs, FileHistory: history}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "new content"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// Verify checkpoint was called with old content.
		if len(history.calls) != 1 {
			t.Fatalf("expected 1 TrackEdit call, got %d", len(history.calls))
		}
		if history.calls[0].Path != filePath {
			t.Errorf("TrackEdit path = %q, want %q", history.calls[0].Path, filePath)
		}
		if history.calls[0].Content != "original content" {
			t.Errorf("TrackEdit content = %q, want %q", history.calls[0].Content, "original content")
		}
	})

	// File history should NOT be called for new file creation.
	t.Run("file_history_not_called_for_new_file", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "brand_new.txt")

		history := &mockFileHistory{}
		tc := &tools.ToolContext{CWD: dir, FileHistory: history}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "fresh"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if len(history.calls) != 0 {
			t.Errorf("expected 0 TrackEdit calls for new file, got %d", len(history.calls))
		}
	})

	// Source: FileWriteTool.ts:305 — always LF line endings
	t.Run("normalizes_crlf_to_lf", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "crlf.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "line1\r\nline2\r\nline3"}`, filePath))
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
		if strings.Contains(string(data), "\r\n") {
			t.Error("file should not contain CRLF after normalization")
		}
		if string(data) != "line1\nline2\nline3" {
			t.Errorf("expected LF content, got %q", string(data))
		}
	})

	// New file in deeply nested non-existent directory
	t.Run("write_new_file_in_new_directory_tree", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "src", "pkg", "utils", "helper.go")
		tc := &tools.ToolContext{CWD: dir}
		content := "package utils\n\nfunc Helper() {}\n"
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": %q}`, filePath, content))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}

		// Verify directories were created.
		info, err := os.Stat(filepath.Join(dir, "src", "pkg", "utils"))
		if err != nil {
			t.Fatalf("parent directory not created: %v", err)
		}
		if !info.IsDir() {
			t.Error("expected directory, got file")
		}

		// Verify content.
		data, err := os.ReadFile(filePath)
		if err != nil {
			t.Fatalf("failed to read: %v", err)
		}
		if string(data) != content {
			t.Errorf("content mismatch: got %q", string(data))
		}

		// Verify create message.
		want := fmt.Sprintf("File created successfully at: %s", filePath)
		if out.Content != want {
			t.Errorf("message = %q, want %q", out.Content, want)
		}
	})

	// Overwrite when no ReadFileState is set (nil) — backwards compat,
	// allows overwrite without staleness check.
	t.Run("overwrite_without_readfilestate_nil", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "existing.txt")
		os.WriteFile(filePath, []byte("old"), 0644)

		tc := &tools.ToolContext{CWD: dir} // ReadFileState is nil
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

	// ReadFileState is updated after successful write.
	t.Run("updates_readfilestate_after_write", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "track.txt")

		rfs := tools.NewReadFileState()
		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "hello"}`, filePath))
		_, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}

		entry := rfs.Get(filePath)
		if entry == nil {
			t.Fatal("expected ReadFileState entry after write")
		}
		if entry.Content != "hello" {
			t.Errorf("ReadFileState content = %q, want %q", entry.Content, "hello")
		}
		if entry.IsPartialView {
			t.Error("ReadFileState should not be partial after write")
		}
	})

	// Write to non-writable path
	t.Run("write_to_readonly_dir_returns_error", func(t *testing.T) {
		dir := t.TempDir()
		readonlyDir := filepath.Join(dir, "readonly")
		os.MkdirAll(readonlyDir, 0555)
		t.Cleanup(func() { os.Chmod(readonlyDir, 0755) })

		filePath := filepath.Join(readonlyDir, "nope.txt")
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "fail"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			// On some systems (e.g. root), writing to 0555 may succeed.
			// Only assert error if the OS actually denied the write.
			if _, readErr := os.ReadFile(filePath); readErr != nil {
				t.Error("expected error for read-only directory write")
			}
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

	// Verify tool satisfies optional interfaces.
	t.Run("implements_tool_prompter", func(t *testing.T) {
		var _ tools.ToolPrompter = tool
	})

	t.Run("implements_search_hinter", func(t *testing.T) {
		var _ tools.SearchHinter = tool
	})

	t.Run("implements_max_result_size_provider", func(t *testing.T) {
		var _ tools.MaxResultSizeCharsProvider = tool
	})

	// DiffDisplay should be attached for updates.
	t.Run("diff_display_on_update", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "diff.txt")
		os.WriteFile(filePath, []byte("line1\nline2\nline3\n"), 0644)

		rfs := tools.NewReadFileState()
		rfs.Record(filePath, "line1\nline2\nline3\n", false)

		tc := &tools.ToolContext{CWD: dir, ReadFileState: rfs}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "content": "line1\nchanged\nline3\n"}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		if out.Display == nil {
			t.Error("expected DiffDisplay for update, got nil")
		}
		dd, ok := out.Display.(tools.DiffDisplay)
		if !ok {
			t.Fatalf("Display is %T, want DiffDisplay", out.Display)
		}
		if dd.FilePath != filePath {
			t.Errorf("DiffDisplay.FilePath = %q, want %q", dd.FilePath, filePath)
		}
		if len(dd.Hunks) == 0 {
			t.Error("expected non-empty hunks in DiffDisplay")
		}
	})
}
