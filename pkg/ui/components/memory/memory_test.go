package memory

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"

	pkgmemory "github.com/projectbarks/gopher-code/pkg/memory"
)

func testFiles() []pkgmemory.FileInfo {
	return []pkgmemory.FileInfo{
		{Path: "/home/user/.claude/CLAUDE.md", Content: "user rules", Type: pkgmemory.TypeUser},
		{Path: "/project/CLAUDE.md", Content: "project rules", Type: pkgmemory.TypeProject},
		{Path: "/project/CLAUDE.local.md", Content: "", Type: pkgmemory.TypeLocal}, // doesn't exist yet
	}
}

func TestNew(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	if len(m.items) != 3 {
		t.Fatalf("items = %d, want 3", len(m.items))
	}
	if m.cursor != 0 {
		t.Error("cursor should start at 0")
	}
}

func TestModel_Navigation(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")

	// Move down
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 1 {
		t.Errorf("cursor = %d after down, want 1", m.cursor)
	}

	// Move down again
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d after down, want 2", m.cursor)
	}

	// Can't go past end
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.cursor != 2 {
		t.Errorf("cursor = %d, should stay at 2", m.cursor)
	}

	// Move up
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.cursor != 1 {
		t.Errorf("cursor = %d after up, want 1", m.cursor)
	}
}

func TestModel_VimKeys(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.cursor != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.cursor != 0 {
		t.Error("k should move up")
	}
}

func TestModel_SelectFile(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // move to project

	var cmd tea.Cmd
	m, cmd = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return a cmd")
	}
	msg := cmd()
	sel, ok := msg.(FileSelectedMsg)
	if !ok {
		t.Fatalf("expected FileSelectedMsg, got %T", msg)
	}
	if sel.Path != "/project/CLAUDE.md" {
		t.Errorf("selected path = %q", sel.Path)
	}
}

func TestModel_Cancel(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if cmd == nil {
		t.Fatal("Escape should return a cmd")
	}
	msg := cmd()
	if _, ok := msg.(FileCancelledMsg); !ok {
		t.Fatalf("expected FileCancelledMsg, got %T", msg)
	}
}

func TestModel_CancelQ(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'q'})
	if cmd == nil {
		t.Fatal("q should return cancel cmd")
	}
}

func TestModel_View(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	v := m.View()

	if !strings.Contains(v, "Memory Files") {
		t.Error("should contain title")
	}
	if !strings.Contains(v, "User memory") {
		t.Error("should show User memory label")
	}
	if !strings.Contains(v, "Project memory") {
		t.Error("should show Project memory label")
	}
	if !strings.Contains(v, "(new)") {
		t.Error("should show (new) for non-existent file")
	}
	if !strings.Contains(v, ">") {
		t.Error("should show cursor indicator")
	}
}

func TestModel_ViewEmpty(t *testing.T) {
	m := New(nil, "/project", "/home/user")
	v := m.View()
	if !strings.Contains(v, "No memory files found") {
		t.Error("empty list should show message")
	}
}

func TestModel_SelectedItem(t *testing.T) {
	m := New(testFiles(), "/project", "/home/user")
	item := m.SelectedItem()
	if item == nil {
		t.Fatal("should have selected item")
	}
	if item.Type != pkgmemory.TypeUser {
		t.Errorf("type = %q, want user", item.Type)
	}
}

func TestModel_SelectedItemEmpty(t *testing.T) {
	m := New(nil, "/project", "/home/user")
	if m.SelectedItem() != nil {
		t.Error("empty model should have nil selected")
	}
}

func TestFormatLabel(t *testing.T) {
	tests := []struct {
		item MemoryItem
		want string
	}{
		{MemoryItem{Path: "/home/user/.claude/CLAUDE.md", Type: pkgmemory.TypeUser}, "User memory"},
		{MemoryItem{Path: "/project/CLAUDE.md", Type: pkgmemory.TypeProject}, "Project memory"},
		{MemoryItem{Path: "/project/CLAUDE.local.md", Type: pkgmemory.TypeLocal}, "Local memory"},
		{MemoryItem{Path: "/etc/claude-code/CLAUDE.md", Type: pkgmemory.TypeManaged}, "Managed memory"},
		{MemoryItem{Path: "/home/user/.claude/rules/coding.md", Type: pkgmemory.TypeUser, DisplayPath: "rules/coding.md"}, "rules/coding.md"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := formatLabel(tt.item)
			if got != tt.want {
				t.Errorf("formatLabel = %q, want %q", got, tt.want)
			}
		})
	}
}

func TestDisplayPath(t *testing.T) {
	tests := []struct {
		path, cwd, home, want string
	}{
		{"/project/CLAUDE.md", "/project", "/home/user", "CLAUDE.md"},
		{"/home/user/.claude/CLAUDE.md", "/project", "/home/user", "~/.claude/CLAUDE.md"},
		{"/etc/claude-code/CLAUDE.md", "/project", "/home/user", "/etc/claude-code/CLAUDE.md"},
	}
	for _, tt := range tests {
		t.Run(tt.want, func(t *testing.T) {
			got := displayPath(tt.path, tt.cwd, tt.home)
			if got != tt.want {
				t.Errorf("displayPath(%q) = %q, want %q", tt.path, got, tt.want)
			}
		})
	}
}
