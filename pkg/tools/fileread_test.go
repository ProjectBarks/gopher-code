package tools_test

import (
	"context"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/provider"
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
		if !strings.Contains(out.Content, "device file would block or produce infinite output") {
			t.Errorf("expected device file warning, got %q", out.Content)
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

	t.Run("description_matches_ts", func(t *testing.T) {
		// Source: prompt.ts:12 — DESCRIPTION = 'Read a file from the local filesystem.'
		want := "Read a file from the local filesystem."
		if got := tool.Description(); got != want {
			t.Errorf("Description() = %q, want %q", got, want)
		}
	})

	t.Run("blocked_device_verbatim_message", func(t *testing.T) {
		// Source: FileReadTool.ts:489 — errorCode 9 verbatim message
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "/dev/zero"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for /dev/zero")
		}
		want := "Cannot read '/dev/zero': this device file would block or produce infinite output."
		if out.Content != want {
			t.Errorf("got %q, want %q", out.Content, want)
		}
	})

	t.Run("blocked_device_full", func(t *testing.T) {
		// Source: FileReadTool.ts:103 — /dev/full is blocked
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "/dev/full"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for /dev/full")
		}
	})

	t.Run("blocked_device_console", func(t *testing.T) {
		// Source: FileReadTool.ts:106 — /dev/console is blocked
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "/dev/console"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Error("expected error for /dev/console")
		}
	})

	t.Run("proc_fd_blocked", func(t *testing.T) {
		// Source: FileReadTool.ts:120-127 — /proc/<pid>/fd/0-2 blocked
		tc := &tools.ToolContext{CWD: "/tmp"}
		for _, p := range []string{"/proc/self/fd/0", "/proc/1234/fd/1", "/proc/self/fd/2"} {
			input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, p))
			out, err := tool.Execute(context.Background(), tc, input)
			if err != nil {
				t.Fatalf("unexpected error for %s: %v", p, err)
			}
			if !out.IsError {
				t.Errorf("expected error for %s", p)
			}
			if !strings.Contains(out.Content, "device file would block") {
				t.Errorf("expected device error for %s, got %q", p, out.Content)
			}
		}
	})

	t.Run("dev_null_not_blocked", func(t *testing.T) {
		// Source: FileReadTool.ts:97 — /dev/null intentionally omitted from blocked list
		tc := &tools.ToolContext{CWD: "/tmp"}
		input := json.RawMessage(`{"file_path": "/dev/null"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// /dev/null should NOT be blocked — it returns empty
		if out.IsError && strings.Contains(out.Content, "device file") {
			t.Error("/dev/null should not be blocked")
		}
	})

	t.Run("binary_file_rejected", func(t *testing.T) {
		// Binary files (non-image, non-PDF) should be rejected with errorCode 4 message
		dir := t.TempDir()
		binPath := filepath.Join(dir, "test.exe")
		os.WriteFile(binPath, []byte{0x4D, 0x5A, 0x90, 0x00}, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, binPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for binary .exe file")
		}
		if !strings.Contains(out.Content, "cannot read binary files") {
			t.Errorf("expected binary rejection message, got %q", out.Content)
		}
		if !strings.Contains(out.Content, ".exe") {
			t.Errorf("expected extension in message, got %q", out.Content)
		}
	})

	t.Run("binary_dll_rejected", func(t *testing.T) {
		dir := t.TempDir()
		dllPath := filepath.Join(dir, "lib.dll")
		os.WriteFile(dllPath, []byte{0x00, 0x01, 0x02}, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, dllPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for .dll file")
		}
	})

	t.Run("binary_zip_rejected", func(t *testing.T) {
		dir := t.TempDir()
		zipPath := filepath.Join(dir, "archive.zip")
		os.WriteFile(zipPath, []byte{0x50, 0x4B, 0x03, 0x04}, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, zipPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for .zip file")
		}
	})

	t.Run("image_png_returns_base64", func(t *testing.T) {
		// Source: FileReadTool.ts:866-891 — images return base64 encoded data
		dir := t.TempDir()
		imgPath := filepath.Join(dir, "test.png")
		// Minimal valid PNG header
		pngData := []byte{0x89, 0x50, 0x4E, 0x47, 0x0D, 0x0A, 0x1A, 0x0A, 0x00, 0x00, 0x00}
		os.WriteFile(imgPath, pngData, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, imgPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		// Should contain base64 data and media type
		var result map[string]any
		if jsonErr := json.Unmarshal([]byte(out.Content), &result); jsonErr != nil {
			t.Fatalf("expected JSON output for image, got %q", out.Content)
		}
		if result["type"] != "image" {
			t.Errorf("expected type=image, got %v", result["type"])
		}
		file, ok := result["file"].(map[string]any)
		if !ok {
			t.Fatalf("expected file map in result")
		}
		if file["type"] != "image/png" {
			t.Errorf("expected image/png, got %v", file["type"])
		}
		if file["base64"] == nil || file["base64"] == "" {
			t.Error("expected non-empty base64 data")
		}
	})

	t.Run("image_jpeg_returns_base64", func(t *testing.T) {
		dir := t.TempDir()
		imgPath := filepath.Join(dir, "photo.jpg")
		jpegData := []byte{0xFF, 0xD8, 0xFF, 0xE0, 0x00, 0x10}
		os.WriteFile(imgPath, jpegData, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, imgPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected tool error: %s", out.Content)
		}
		var result map[string]any
		if jsonErr := json.Unmarshal([]byte(out.Content), &result); jsonErr != nil {
			t.Fatalf("expected JSON output, got %q", out.Content)
		}
		file := result["file"].(map[string]any)
		if file["type"] != "image/jpeg" {
			t.Errorf("expected image/jpeg, got %v", file["type"])
		}
	})

	t.Run("image_empty_rejected", func(t *testing.T) {
		// Source: FileReadTool.ts:1109-1111 — empty image throws
		dir := t.TempDir()
		imgPath := filepath.Join(dir, "empty.png")
		os.WriteFile(imgPath, []byte{}, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, imgPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for empty image")
		}
		if !strings.Contains(out.Content, "Image file is empty") {
			t.Errorf("expected empty image error, got %q", out.Content)
		}
	})

	t.Run("image_not_found", func(t *testing.T) {
		dir := t.TempDir()
		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filepath.Join(dir, "missing.png")))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for missing image")
		}
		if !strings.Contains(out.Content, "does not exist") {
			t.Errorf("expected file-not-found, got %q", out.Content)
		}
	})

	t.Run("max_file_size_guard", func(t *testing.T) {
		// Source: limits.ts — maxSizeBytes gates on TOTAL file size
		dir := t.TempDir()
		bigPath := filepath.Join(dir, "big.txt")
		// Create a file larger than MaxOutputSize (256KB)
		data := make([]byte, 300*1024) // 300KB
		for i := range data {
			data[i] = 'A'
		}
		os.WriteFile(bigPath, data, 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, bigPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for oversized file")
		}
		if !strings.Contains(out.Content, "exceeds maximum allowed size") {
			t.Errorf("expected max size error, got %q", out.Content)
		}
	})

	t.Run("max_file_size_bypassed_with_limit", func(t *testing.T) {
		// When an explicit limit is set, maxSizeBytes guard is skipped
		dir := t.TempDir()
		bigPath := filepath.Join(dir, "big.txt")
		var sb strings.Builder
		for i := 0; i < 50000; i++ {
			sb.WriteString(fmt.Sprintf("line %d content here\n", i+1))
		}
		os.WriteFile(bigPath, []byte(sb.String()), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "limit": 5}`, bigPath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("with explicit limit, should not reject oversized file: %s", out.Content)
		}
		// Should have exactly 5 lines
		lineCount := strings.Count(out.Content, "\n")
		// Each line ends with \n so the trailing newline is the 6th
		if lineCount != 5 {
			t.Errorf("expected 5 lines, got %d (content: %q)", lineCount, out.Content[:min(200, len(out.Content))])
		}
	})

	t.Run("line_numbers_cat_n_format", func(t *testing.T) {
		// Source: prompt.ts:14 — LINE_FORMAT_INSTRUCTION: "cat -n format, with line numbers starting at 1"
		// Format: N\tline_content (number, tab, content)
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("alpha\nbeta\ngamma\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		lines := strings.Split(strings.TrimRight(out.Content, "\n"), "\n")
		if len(lines) != 3 {
			t.Fatalf("expected 3 lines, got %d: %q", len(lines), out.Content)
		}
		// Each line should be "N\tcontent"
		for i, line := range lines {
			parts := strings.SplitN(line, "\t", 2)
			if len(parts) != 2 {
				t.Errorf("line %d: expected N\\tcontent format, got %q", i+1, line)
				continue
			}
			wantNum := fmt.Sprintf("%d", i+1)
			if parts[0] != wantNum {
				t.Errorf("line %d: expected number %q, got %q", i+1, wantNum, parts[0])
			}
		}
	})

	t.Run("line_numbers_with_offset", func(t *testing.T) {
		// Line numbers should reflect original file position, not output position
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("a\nb\nc\nd\ne\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 3, "limit": 2}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		// Should show lines 3-4 with original line numbers
		if !strings.Contains(out.Content, "3\tc") {
			t.Errorf("expected '3\\tc', got %q", out.Content)
		}
		if !strings.Contains(out.Content, "4\td") {
			t.Errorf("expected '4\\td', got %q", out.Content)
		}
		if strings.Contains(out.Content, "1\t") || strings.Contains(out.Content, "5\t") {
			t.Errorf("should not contain lines outside range, got %q", out.Content)
		}
	})

	t.Run("read_file_state_recorded", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "tracked.txt")
		os.WriteFile(filePath, []byte("hello\nworld\n"), 0644)

		state := tools.NewReadFileState()
		tc := &tools.ToolContext{CWD: dir, ReadFileState: state}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		_, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entry := state.Get(filePath)
		if entry == nil {
			t.Fatal("expected ReadFileState to have an entry after read")
		}
		if entry.Content != "hello\nworld" {
			t.Errorf("expected content 'hello\\nworld', got %q", entry.Content)
		}
		if entry.IsPartialView {
			t.Error("full read should not be marked as partial")
		}
	})

	t.Run("read_file_state_partial", func(t *testing.T) {
		dir := t.TempDir()
		filePath := filepath.Join(dir, "partial.txt")
		os.WriteFile(filePath, []byte("a\nb\nc\nd\n"), 0644)

		state := tools.NewReadFileState()
		tc := &tools.ToolContext{CWD: dir, ReadFileState: state}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q, "offset": 2, "limit": 1}`, filePath))
		_, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		entry := state.Get(filePath)
		if entry == nil {
			t.Fatal("expected ReadFileState entry")
		}
		if !entry.IsPartialView {
			t.Error("offset+limit read should be marked as partial")
		}
	})

	t.Run("constants_match_ts", func(t *testing.T) {
		// Verify critical constants match TS source
		if tools.MaxLinesToRead != 2000 {
			t.Errorf("MaxLinesToRead = %d, want 2000", tools.MaxLinesToRead)
		}
		if tools.DefaultMaxOutputTokens != 25000 {
			t.Errorf("DefaultMaxOutputTokens = %d, want 25000", tools.DefaultMaxOutputTokens)
		}
		if tools.MaxOutputSize != 256*1024 {
			t.Errorf("MaxOutputSize = %d, want %d", tools.MaxOutputSize, 256*1024)
		}
		if tools.PDFMaxPagesPerRead != 20 {
			t.Errorf("PDFMaxPagesPerRead = %d, want 20", tools.PDFMaxPagesPerRead)
		}
	})

	t.Run("file_unchanged_stub_verbatim", func(t *testing.T) {
		want := "File unchanged since last read. The content from the earlier Read tool_result in this conversation is still current — refer to that instead of re-reading."
		if tools.FileUnchangedStub != want {
			t.Errorf("FileUnchangedStub = %q, want %q", tools.FileUnchangedStub, want)
		}
	})

	t.Run("cyber_risk_reminder_verbatim", func(t *testing.T) {
		if !strings.Contains(tools.CyberRiskMitigationReminder, "consider whether it would be considered malware") {
			t.Error("CyberRiskMitigationReminder missing expected text")
		}
		if !strings.HasPrefix(tools.CyberRiskMitigationReminder, "\n\n<system-reminder>") {
			t.Error("CyberRiskMitigationReminder should start with \\n\\n<system-reminder>")
		}
	})

	t.Run("pdf_page_range_valid", func(t *testing.T) {
		tests := []struct {
			input string
			first int
			last  int
		}{
			{"5", 5, 5},
			{"1-10", 1, 10},
			{"3-", 3, -1},
		}
		for _, tt := range tests {
			r := tools.ParsePDFPageRange(tt.input)
			if r == nil {
				t.Errorf("ParsePDFPageRange(%q) = nil, want result", tt.input)
				continue
			}
			if r.FirstPage != tt.first || r.LastPage != tt.last {
				t.Errorf("ParsePDFPageRange(%q) = {%d,%d}, want {%d,%d}", tt.input, r.FirstPage, r.LastPage, tt.first, tt.last)
			}
		}
	})

	t.Run("pdf_page_range_invalid", func(t *testing.T) {
		for _, input := range []string{"", "abc", "0", "-1", "10-5"} {
			r := tools.ParsePDFPageRange(input)
			if r != nil {
				t.Errorf("ParsePDFPageRange(%q) = %+v, want nil", input, r)
			}
		}
	})

	t.Run("pdf_pages_too_many_rejected", func(t *testing.T) {
		tc := &tools.ToolContext{CWD: t.TempDir()}
		input := json.RawMessage(`{"file_path": "/tmp/test.pdf", "pages": "1-25"}`)
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !out.IsError {
			t.Fatal("expected error for page range exceeding max")
		}
		if !strings.Contains(out.Content, "exceeds maximum of 20 pages") {
			t.Errorf("expected page limit error, got %q", out.Content)
		}
	})

	t.Run("trailing_newline_in_output", func(t *testing.T) {
		// Output should end with exactly one trailing newline
		dir := t.TempDir()
		filePath := filepath.Join(dir, "test.txt")
		os.WriteFile(filePath, []byte("hello\n"), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if !strings.HasSuffix(out.Content, "\n") {
			t.Error("output should end with trailing newline")
		}
	})

	t.Run("default_limit_2000", func(t *testing.T) {
		// Source: prompt.ts:10 — MAX_LINES_TO_READ = 2000
		dir := t.TempDir()
		filePath := filepath.Join(dir, "large.txt")
		var sb strings.Builder
		for i := 1; i <= 2500; i++ {
			sb.WriteString(fmt.Sprintf("line %d\n", i))
		}
		os.WriteFile(filePath, []byte(sb.String()), 0644)

		tc := &tools.ToolContext{CWD: dir}
		input := json.RawMessage(fmt.Sprintf(`{"file_path": %q}`, filePath))
		out, err := tool.Execute(context.Background(), tc, input)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if out.IsError {
			t.Fatalf("unexpected error: %s", out.Content)
		}
		// Count output lines (trailing newline means count = lines)
		outputLines := strings.Split(strings.TrimRight(out.Content, "\n"), "\n")
		if len(outputLines) != 2000 {
			t.Errorf("expected 2000 lines (default limit), got %d", len(outputLines))
		}
		// Last line should be line 2000
		if !strings.Contains(outputLines[len(outputLines)-1], "2000\t") {
			t.Errorf("last line should be line 2000, got %q", outputLines[len(outputLines)-1])
		}
	})
}

// TestFileReadTool_APILimitsIntegration verifies that the FileReadTool's page
// range validation uses the canonical constants from pkg/provider/api_limits.go.
// This ensures the provider limits are wired into the binary through tools.
func TestFileReadTool_APILimitsIntegration(t *testing.T) {
	// Verify the re-exported constant matches the provider canonical value.
	if tools.PDFMaxPagesPerRead != provider.PDFMaxPagesPerRead {
		t.Fatalf("tools.PDFMaxPagesPerRead (%d) != provider.PDFMaxPagesPerRead (%d)",
			tools.PDFMaxPagesPerRead, provider.PDFMaxPagesPerRead)
	}

	tool := tools.FileReadTool{}
	tc := &tools.ToolContext{CWD: t.TempDir()}

	// Request exactly at the limit should NOT be rejected on page count alone.
	atLimit := fmt.Sprintf(`{"file_path": "/tmp/test.pdf", "pages": "1-%d"}`, provider.PDFMaxPagesPerRead)
	out, err := tool.Execute(context.Background(), tc, json.RawMessage(atLimit))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if out.IsError && strings.Contains(out.Content, "exceeds maximum") {
		t.Errorf("page range at limit (%d) should not be rejected for exceeding max pages", provider.PDFMaxPagesPerRead)
	}

	// Request one page over the limit MUST be rejected.
	overLimit := fmt.Sprintf(`{"file_path": "/tmp/test.pdf", "pages": "1-%d"}`, provider.PDFMaxPagesPerRead+1)
	out, err = tool.Execute(context.Background(), tc, json.RawMessage(overLimit))
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if !out.IsError {
		t.Fatal("expected error for page range exceeding provider.PDFMaxPagesPerRead")
	}
	expected := fmt.Sprintf("exceeds maximum of %d pages", provider.PDFMaxPagesPerRead)
	if !strings.Contains(out.Content, expected) {
		t.Errorf("error should reference provider limit %d, got %q", provider.PDFMaxPagesPerRead, out.Content)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
