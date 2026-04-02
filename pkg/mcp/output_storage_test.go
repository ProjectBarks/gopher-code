package mcp

import (
	"os"
	"path/filepath"
	"strings"
	"testing"
)

// Source: utils/mcpOutputStorage.ts, utils/toolResultStorage.ts

func TestExtensionForMimeType(t *testing.T) {
	// Source: utils/mcpOutputStorage.ts:66-118
	tests := []struct {
		mimeType string
		expected string
	}{
		{"application/pdf", "pdf"},
		{"application/json", "json"},
		{"text/csv", "csv"},
		{"text/plain", "txt"},
		{"text/html", "html"},
		{"text/markdown", "md"},
		{"application/zip", "zip"},
		{"application/vnd.openxmlformats-officedocument.wordprocessingml.document", "docx"},
		{"application/vnd.openxmlformats-officedocument.spreadsheetml.sheet", "xlsx"},
		{"application/vnd.openxmlformats-officedocument.presentationml.presentation", "pptx"},
		{"application/msword", "doc"},
		{"application/vnd.ms-excel", "xls"},
		{"audio/mpeg", "mp3"},
		{"audio/wav", "wav"},
		{"audio/ogg", "ogg"},
		{"video/mp4", "mp4"},
		{"video/webm", "webm"},
		{"image/png", "png"},
		{"image/jpeg", "jpg"},
		{"image/gif", "gif"},
		{"image/webp", "webp"},
		{"image/svg+xml", "svg"},
		{"", "bin"},
		{"application/octet-stream", "bin"},
		{"application/pdf; charset=utf-8", "pdf"}, // strips parameters
	}

	for _, tc := range tests {
		t.Run(tc.mimeType, func(t *testing.T) {
			got := ExtensionForMimeType(tc.mimeType)
			if got != tc.expected {
				t.Errorf("ExtensionForMimeType(%q) = %q, want %q", tc.mimeType, got, tc.expected)
			}
		})
	}
}

func TestIsBinaryContentType(t *testing.T) {
	// Source: utils/mcpOutputStorage.ts:125-136
	t.Run("text_types_not_binary", func(t *testing.T) {
		for _, ct := range []string{"text/plain", "text/html", "text/csv"} {
			if IsBinaryContentType(ct) {
				t.Errorf("%q should not be binary", ct)
			}
		}
	})

	t.Run("json_not_binary", func(t *testing.T) {
		for _, ct := range []string{"application/json", "application/vnd.api+json"} {
			if IsBinaryContentType(ct) {
				t.Errorf("%q should not be binary", ct)
			}
		}
	})

	t.Run("xml_not_binary", func(t *testing.T) {
		for _, ct := range []string{"application/xml", "application/atom+xml"} {
			if IsBinaryContentType(ct) {
				t.Errorf("%q should not be binary", ct)
			}
		}
	})

	t.Run("javascript_not_binary", func(t *testing.T) {
		if IsBinaryContentType("application/javascript") {
			t.Error("javascript should not be binary")
		}
	})

	t.Run("form_data_not_binary", func(t *testing.T) {
		if IsBinaryContentType("application/x-www-form-urlencoded") {
			t.Error("form data should not be binary")
		}
	})

	t.Run("binary_types", func(t *testing.T) {
		for _, ct := range []string{
			"application/pdf", "application/zip", "application/octet-stream",
			"image/png", "image/jpeg", "audio/mpeg", "video/mp4",
		} {
			if !IsBinaryContentType(ct) {
				t.Errorf("%q should be binary", ct)
			}
		}
	})

	t.Run("empty_not_binary", func(t *testing.T) {
		if IsBinaryContentType("") {
			t.Error("empty should not be binary")
		}
	})

	t.Run("with_parameters", func(t *testing.T) {
		// Source: mcpOutputStorage.ts:128 — strips charset parameter
		if IsBinaryContentType("text/plain; charset=utf-8") {
			t.Error("text with charset should not be binary")
		}
		if !IsBinaryContentType("image/png; quality=high") {
			t.Error("image with params should be binary")
		}
	})
}

func TestGetLargeOutputInstructions(t *testing.T) {
	// Source: utils/mcpOutputStorage.ts:39-58
	instructions := GetLargeOutputInstructions("/tmp/output.txt", 150000, "Plain text")

	if !strings.Contains(instructions, "150000 characters") {
		t.Error("should mention character count")
	}
	if !strings.Contains(instructions, "/tmp/output.txt") {
		t.Error("should mention file path")
	}
	if !strings.Contains(instructions, "Plain text") {
		t.Error("should mention format")
	}
	if !strings.Contains(instructions, "sequential chunks") {
		t.Error("should instruct sequential reading")
	}
	if !strings.Contains(instructions, "100%") {
		t.Error("should require 100% read")
	}
}

func TestPersistBinaryContent(t *testing.T) {
	// Source: utils/mcpOutputStorage.ts:148-174
	dir := t.TempDir()
	data := []byte("fake PDF content")

	result := PersistBinaryContent(data, "application/pdf", "test-persist-1", dir)
	if result.Error != "" {
		t.Fatalf("unexpected error: %s", result.Error)
	}
	if result.Ext != "pdf" {
		t.Errorf("ext = %q, want pdf", result.Ext)
	}
	if result.Size != len(data) {
		t.Errorf("size = %d, want %d", result.Size, len(data))
	}
	if !strings.HasSuffix(result.Filepath, "test-persist-1.pdf") {
		t.Errorf("filepath = %q, expected .pdf suffix", result.Filepath)
	}

	// Verify file was written
	read, err := os.ReadFile(result.Filepath)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if string(read) != string(data) {
		t.Error("file content mismatch")
	}
}

func TestPersistBinaryContentBadDir(t *testing.T) {
	result := PersistBinaryContent([]byte("data"), "image/png", "test", "/nonexistent/dir/that/does/not/exist")
	if result.Error == "" {
		t.Error("expected error for bad directory")
	}
}

func TestPersistLargeOutput(t *testing.T) {
	// Source: utils/toolResultStorage.ts + mcpOutputStorage.ts
	dir := t.TempDir()
	content := strings.Repeat("x", 5000)

	instructions, err := PersistLargeOutput(content, "test-large-1", dir, "Plain text")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	// Should contain instructions
	if !strings.Contains(instructions, "5000 characters") {
		t.Error("should mention character count")
	}

	// Should contain preview
	if !strings.Contains(instructions, "Preview") {
		t.Error("should contain preview for large content")
	}

	// Verify file was written
	fp := filepath.Join(dir, "test-large-1.txt")
	read, err := os.ReadFile(fp)
	if err != nil {
		t.Fatalf("read file: %v", err)
	}
	if len(read) != 5000 {
		t.Errorf("file size = %d, want 5000", len(read))
	}
}

func TestPreviewSizeBytes(t *testing.T) {
	// Source: derived from mcpOutputStorage.ts PREVIEW_SIZE_BYTES
	if PreviewSizeBytes != 2000 {
		t.Errorf("PreviewSizeBytes = %d, want 2000", PreviewSizeBytes)
	}
}
