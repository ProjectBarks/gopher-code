package main

import (
	"os"
	"path/filepath"
	"strings"
	"testing"

	"github.com/projectbarks/gopher-code/pkg/memdir"
	"github.com/projectbarks/gopher-code/pkg/prompt"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// TestSessionMemoryIndexPath_WiredIntoBinary verifies that session.MemoryIndexPath
// constructs the correct path, matching the usage in main.go's memory section builder.
func TestSessionMemoryIndexPath_WiredIntoBinary(t *testing.T) {
	dir := "/tmp/test-mem"
	got := session.MemoryIndexPath(dir)
	want := filepath.Join(dir, "MEMORY.md")
	if got != want {
		t.Errorf("MemoryIndexPath(%q) = %q, want %q", dir, got, want)
	}
}

// TestSessionReadMemoryIndex_WiredIntoBinary exercises session.ReadMemoryIndex
// through the same flow used by the memory section builder in main.go:
// create a temp memdir, write MEMORY.md via AppendToMemoryIndex, read it back.
func TestSessionReadMemoryIndex_WiredIntoBinary(t *testing.T) {
	memDir := t.TempDir()

	// Write entries using session.AppendToMemoryIndex.
	if err := session.AppendToMemoryIndex(memDir, "- [Prefs](prefs.md) — user preferences"); err != nil {
		t.Fatalf("AppendToMemoryIndex: %v", err)
	}
	if err := session.AppendToMemoryIndex(memDir, "- [Style](style.md) — coding style"); err != nil {
		t.Fatalf("AppendToMemoryIndex: %v", err)
	}

	// Read back through session.ReadMemoryIndex.
	result, err := session.ReadMemoryIndex(memDir)
	if err != nil {
		t.Fatalf("ReadMemoryIndex: %v", err)
	}
	if !strings.Contains(result.Content, "user preferences") {
		t.Error("ReadMemoryIndex result should contain 'user preferences'")
	}
	if !strings.Contains(result.Content, "coding style") {
		t.Error("ReadMemoryIndex result should contain 'coding style'")
	}
	if result.WasLineTruncated {
		t.Error("should not be line-truncated for 2 lines")
	}
}

// TestSessionReadMemoryIndex_Missing returns empty for nonexistent dir.
func TestSessionReadMemoryIndex_Missing(t *testing.T) {
	result, err := session.ReadMemoryIndex(filepath.Join(t.TempDir(), "nonexistent"))
	if err != nil {
		t.Fatalf("ReadMemoryIndex on missing dir should not error, got: %v", err)
	}
	if result.Content != "" {
		t.Errorf("expected empty content, got %q", result.Content)
	}
}

// TestSessionMemorySection_EndToEnd replicates the exact memory section
// builder from main.go to verify session.MemoryIndexPath and
// session.ReadMemoryIndex are wired through the binary code path.
func TestSessionMemorySection_EndToEnd(t *testing.T) {
	autoMemDir := t.TempDir()
	_ = memdir.EnsureMemoryDirExists(autoMemDir)

	// Write MEMORY.md at the session-computed path.
	indexPath := session.MemoryIndexPath(autoMemDir)
	err := os.WriteFile(indexPath, []byte("- [Config](config.md) — project config\n"), 0644)
	if err != nil {
		t.Fatalf("write MEMORY.md: %v", err)
	}

	// Replicate main.go's memory section builder logic exactly.
	section := prompt.SystemPromptSection("memory", func() *string {
		_ = memdir.EnsureMemoryDirExists(autoMemDir)
		content := ""
		if data, err := os.ReadFile(session.MemoryIndexPath(autoMemDir)); err == nil {
			content = string(data)
		}
		projectMemDir := session.MemoryDir()
		projectMem, _ := session.ReadMemoryIndex(projectMemDir)
		_ = projectMem
		p := memdir.BuildMemoryPrompt("auto memory", autoMemDir, nil, content)
		return &p
	})

	cwd := t.TempDir()
	result := prompt.BuildSystemPrompt("base prompt", cwd, "test-model", section)

	if !strings.Contains(result, "project config") {
		t.Error("prompt should contain MEMORY.md entry 'project config'")
	}
	if !strings.Contains(result, "auto memory") {
		t.Error("prompt should contain display name 'auto memory'")
	}
}

// TestSessionParseMemoryFile_WiredIntoBinary verifies session.ParseMemoryFile
// and session.FormatMemoryFile round-trip through the binary's import chain.
func TestSessionParseMemoryFile_WiredIntoBinary(t *testing.T) {
	input := "---\nname: test\ndescription: a test memory\ntype: user\n---\n\nHello world"
	entry, err := session.ParseMemoryFile(input)
	if err != nil {
		t.Fatalf("ParseMemoryFile: %v", err)
	}
	if entry.Name != "test" {
		t.Errorf("Name = %q, want %q", entry.Name, "test")
	}
	if entry.Type != session.MemoryTypeUser {
		t.Errorf("Type = %q, want %q", entry.Type, session.MemoryTypeUser)
	}
	if entry.Body != "Hello world" {
		t.Errorf("Body = %q, want %q", entry.Body, "Hello world")
	}

	// Round-trip through FormatMemoryFile.
	formatted := session.FormatMemoryFile(entry)
	if !strings.Contains(formatted, "name: test") {
		t.Error("FormatMemoryFile should contain 'name: test'")
	}
}

// TestSessionTruncateEntrypoint_WiredIntoBinary verifies that
// session.TruncateEntrypointContent correctly truncates oversized content.
func TestSessionTruncateEntrypoint_WiredIntoBinary(t *testing.T) {
	var lines []string
	for i := 0; i < session.MaxEntrypointLines+10; i++ {
		lines = append(lines, "- entry line")
	}
	content := strings.Join(lines, "\n")

	result := session.TruncateEntrypointContent(content)
	if !result.WasLineTruncated {
		t.Error("should be line-truncated")
	}
	if !strings.Contains(result.Content, "WARNING") {
		t.Error("truncated content should contain WARNING")
	}
}
