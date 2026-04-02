package session

import (
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
