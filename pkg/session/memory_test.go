package session

import (
	"path/filepath"
	"strings"
	"testing"
)

// Source: memdir/memoryTypes.ts

func TestParseMemoryType(t *testing.T) {
	// Source: memdir/memoryTypes.ts:28-31
	tests := []struct {
		input    string
		expected MemoryType
	}{
		{"user", MemoryTypeUser},
		{"feedback", MemoryTypeFeedback},
		{"project", MemoryTypeProject},
		{"reference", MemoryTypeReference},
		{"invalid", ""},
		{"", ""},
		{"User", ""}, // Case-sensitive
	}
	for _, tt := range tests {
		got := ParseMemoryType(tt.input)
		if got != tt.expected {
			t.Errorf("ParseMemoryType(%q) = %q, want %q", tt.input, got, tt.expected)
		}
	}
}

func TestValidMemoryTypes(t *testing.T) {
	// Source: memdir/memoryTypes.ts:14-19
	expected := []string{"user", "feedback", "project", "reference"}
	if len(ValidMemoryTypes) != len(expected) {
		t.Fatalf("expected %d types, got %d", len(expected), len(ValidMemoryTypes))
	}
	for i, mt := range ValidMemoryTypes {
		if string(mt) != expected[i] {
			t.Errorf("type[%d] = %q, want %q", i, mt, expected[i])
		}
	}
}

func TestParseMemoryFile(t *testing.T) {
	// Source: memdir/memoryTypes.ts:261-271

	t.Run("with_frontmatter", func(t *testing.T) {
		content := `---
name: Ask before editing
description: Don't make code changes without explicit approval
type: feedback
---

Do not edit code without explicit user instruction.

**Why:** User reverted unsolicited code changes.
**How to apply:** Present diagnosis first, wait for approval.`

		entry, err := ParseMemoryFile(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Name != "Ask before editing" {
			t.Errorf("name = %q, want 'Ask before editing'", entry.Name)
		}
		if entry.Description != "Don't make code changes without explicit approval" {
			t.Errorf("description = %q", entry.Description)
		}
		if entry.Type != MemoryTypeFeedback {
			t.Errorf("type = %q, want feedback", entry.Type)
		}
		if !strings.Contains(entry.Body, "Do not edit code") {
			t.Error("body should contain memory content")
		}
		if !strings.Contains(entry.Body, "**Why:**") {
			t.Error("body should contain Why section")
		}
	})

	t.Run("without_frontmatter", func(t *testing.T) {
		content := "Just plain text without any frontmatter."
		entry, err := ParseMemoryFile(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Name != "" {
			t.Errorf("expected empty name, got %q", entry.Name)
		}
		if entry.Body != content {
			t.Errorf("body should be entire content, got %q", entry.Body)
		}
	})

	t.Run("invalid_type_parsed_as_empty", func(t *testing.T) {
		content := `---
name: test
type: invalid_type
---

Content here.`
		entry, err := ParseMemoryFile(content)
		if err != nil {
			t.Fatalf("unexpected error: %v", err)
		}
		if entry.Type != "" {
			t.Errorf("expected empty type for invalid, got %q", entry.Type)
		}
	})
}

func TestFormatMemoryFile(t *testing.T) {
	// Source: memdir/memoryTypes.ts:261-271
	entry := &MemoryEntry{
		Name:        "User role",
		Description: "User is a data scientist",
		Type:        MemoryTypeUser,
		Body:        "User is a data scientist focused on observability.",
	}
	formatted := FormatMemoryFile(entry)

	if !strings.HasPrefix(formatted, "---\n") {
		t.Error("should start with frontmatter delimiter")
	}
	if !strings.Contains(formatted, "name: User role") {
		t.Error("should contain name field")
	}
	if !strings.Contains(formatted, "type: user") {
		t.Error("should contain type field")
	}
	if !strings.Contains(formatted, "User is a data scientist focused") {
		t.Error("should contain body")
	}
}

func TestRoundTrip(t *testing.T) {
	original := &MemoryEntry{
		Name:        "Test memory",
		Description: "A test description",
		Type:        MemoryTypeProject,
		Body:        "Some project context.\n\n**Why:** Important.\n**How to apply:** Always.",
	}

	formatted := FormatMemoryFile(original)
	parsed, err := ParseMemoryFile(formatted)
	if err != nil {
		t.Fatalf("round-trip parse failed: %v", err)
	}

	if parsed.Name != original.Name {
		t.Errorf("name: %q != %q", parsed.Name, original.Name)
	}
	if parsed.Description != original.Description {
		t.Errorf("description: %q != %q", parsed.Description, original.Description)
	}
	if parsed.Type != original.Type {
		t.Errorf("type: %q != %q", parsed.Type, original.Type)
	}
	if strings.TrimSpace(parsed.Body) != strings.TrimSpace(original.Body) {
		t.Errorf("body mismatch:\ngot:  %q\nwant: %q", parsed.Body, original.Body)
	}
}

func TestTruncateEntrypointContent(t *testing.T) {
	// Source: memdir/memdir.ts:57-103

	t.Run("under_limits_unchanged", func(t *testing.T) {
		content := "- [Prefs](prefs.md) — user preferences\n- [Style](style.md) — coding style"
		result := TruncateEntrypointContent(content)
		if result.WasLineTruncated || result.WasByteTruncated {
			t.Error("should not be truncated when under limits")
		}
		if result.Content != strings.TrimSpace(content) {
			t.Error("content should be trimmed but unchanged")
		}
	})

	t.Run("line_truncation_at_200", func(t *testing.T) {
		// Source: memdir/memdir.ts:34 — MAX_ENTRYPOINT_LINES = 200
		var lines []string
		for i := 0; i < 250; i++ {
			lines = append(lines, "- [Entry](e.md) — short")
		}
		content := strings.Join(lines, "\n")
		result := TruncateEntrypointContent(content)

		if !result.WasLineTruncated {
			t.Error("should be line-truncated at 250 lines")
		}
		if result.LineCount != 250 {
			t.Errorf("lineCount = %d, want 250", result.LineCount)
		}
		// Content should have exactly 200 lines (plus warning)
		outputLines := strings.Split(result.Content, "\n")
		// Warning adds 2+ lines at the end
		if len(outputLines) < 200 {
			t.Errorf("truncated content should have at least 200 lines, got %d", len(outputLines))
		}
		if !strings.Contains(result.Content, "WARNING: MEMORY.md") {
			t.Error("should contain truncation warning")
		}
	})

	t.Run("byte_truncation", func(t *testing.T) {
		// Source: memdir/memdir.ts:38 — MAX_ENTRYPOINT_BYTES = 25_000
		// Create content with few lines but large byte count
		longLine := strings.Repeat("x", 30_000)
		result := TruncateEntrypointContent(longLine)

		if !result.WasByteTruncated {
			t.Error("should be byte-truncated")
		}
		if !strings.Contains(result.Content, "index entries are too long") {
			t.Error("should mention 'index entries are too long'")
		}
	})

	t.Run("constants_match_ts", func(t *testing.T) {
		// Source: memdir/memdir.ts:34-38
		if MaxEntrypointLines != 200 {
			t.Errorf("MaxEntrypointLines = %d, want 200", MaxEntrypointLines)
		}
		if MaxEntrypointBytes != 25_000 {
			t.Errorf("MaxEntrypointBytes = %d, want 25000", MaxEntrypointBytes)
		}
		if EntrypointName != "MEMORY.md" {
			t.Errorf("EntrypointName = %q, want 'MEMORY.md'", EntrypointName)
		}
	})
}

func TestAppendToMemoryIndex(t *testing.T) {
	dir := t.TempDir()
	memDir := filepath.Join(dir, "memory")

	// First append creates the file
	err := AppendToMemoryIndex(memDir, "- [Prefs](prefs.md) — user preferences")
	if err != nil {
		t.Fatalf("first append failed: %v", err)
	}

	// Second append adds to it
	err = AppendToMemoryIndex(memDir, "- [Style](style.md) — coding style")
	if err != nil {
		t.Fatalf("second append failed: %v", err)
	}

	// Read back
	result, err := ReadMemoryIndex(memDir)
	if err != nil {
		t.Fatalf("read failed: %v", err)
	}
	if result.LineCount != 2 {
		t.Errorf("expected 2 lines, got %d", result.LineCount)
	}
	if !strings.Contains(result.Content, "Prefs") || !strings.Contains(result.Content, "Style") {
		t.Error("should contain both entries")
	}
}

func TestReadMemoryIndex_Missing(t *testing.T) {
	dir := t.TempDir()
	result, err := ReadMemoryIndex(filepath.Join(dir, "nonexistent"))
	if err != nil {
		t.Fatalf("should not error on missing: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty content for missing index, got %q", result.Content)
	}
}
