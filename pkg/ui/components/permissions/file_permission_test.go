package permissions

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

// --- FileEditPermission ---

func TestFileEditPermission_Allow(t *testing.T) {
	m := NewFileEditPermission("t1", "/src/main.go", "old code", "new code", false)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Errorf("decision = %q", msg.Decision)
	}
	if msg.ToolUseID != "t1" {
		t.Errorf("toolUseID = %q", msg.ToolUseID)
	}
	if msg.Request.FilePath != "/src/main.go" {
		t.Errorf("filePath = %q", msg.Request.FilePath)
	}
}

func TestFileEditPermission_Deny(t *testing.T) {
	m := NewFileEditPermission("t1", "f.go", "a", "b", false)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'n'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Errorf("decision = %q", msg.Decision)
	}
}

func TestFileEditPermission_View(t *testing.T) {
	m := NewFileEditPermission("t1", "/src/main.go", "func old()", "func new()", false)
	v := m.View()
	if !strings.Contains(v, "edit a file") {
		t.Error("should mention file edit")
	}
	if !strings.Contains(v, "main.go") {
		t.Error("should show filename")
	}
	if !strings.Contains(v, "func old()") {
		t.Error("should show removed text")
	}
	if !strings.Contains(v, "func new()") {
		t.Error("should show added text")
	}
}

func TestFileEditPermission_ViewReplaceAll(t *testing.T) {
	m := NewFileEditPermission("t1", "f.go", "a", "b", true)
	v := m.View()
	if !strings.Contains(v, "replace all") {
		t.Error("should mention replace all")
	}
}

func TestFileEditPermission_Navigation(t *testing.T) {
	m := NewFileEditPermission("t1", "f.go", "a", "b", false)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.selected != 1 {
		t.Errorf("selected = %d", m.selected)
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.selected != 0 {
		t.Errorf("selected = %d", m.selected)
	}
}

func TestFileEditPermission_Enter(t *testing.T) {
	m := NewFileEditPermission("t1", "f.go", "a", "b", false)
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown}) // Deny
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Errorf("Enter on Deny should deny, got %q", msg.Decision)
	}
}

// --- FileWritePermission ---

func TestFileWritePermission_Allow(t *testing.T) {
	m := NewFileWritePermission("t1", "/tmp/new.txt", "hello world", true)
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'y'})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionAllow {
		t.Errorf("decision = %q", msg.Decision)
	}
}

func TestFileWritePermission_View_NewFile(t *testing.T) {
	m := NewFileWritePermission("t1", "/tmp/new.txt", "content here", true)
	v := m.View()
	if !strings.Contains(v, "create") {
		t.Error("new file should say 'create'")
	}
	if !strings.Contains(v, "new.txt") {
		t.Error("should show filename")
	}
	if !strings.Contains(v, "content here") {
		t.Error("should show content preview")
	}
}

func TestFileWritePermission_View_Overwrite(t *testing.T) {
	m := NewFileWritePermission("t1", "/tmp/existing.txt", "new content", false)
	v := m.View()
	if !strings.Contains(v, "write to") {
		t.Error("overwrite should say 'write to'")
	}
}

func TestFileWritePermission_View_Truncated(t *testing.T) {
	content := strings.Repeat("line\n", 20)
	m := NewFileWritePermission("t1", "f.txt", content, true)
	v := m.View()
	if !strings.Contains(v, "more lines") {
		t.Error("long content should be truncated")
	}
}

func TestFileWritePermission_Navigation(t *testing.T) {
	m := NewFileWritePermission("t1", "f.txt", "x", false)
	m, _ = m.Update(tea.KeyPressMsg{Code: 'j'})
	if m.selected != 1 {
		t.Error("j should move down")
	}
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k'})
	if m.selected != 0 {
		t.Error("k should move up")
	}
}

func TestFileWritePermission_Escape(t *testing.T) {
	m := NewFileWritePermission("t1", "f.txt", "x", false)
	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	msg := cmd().(PermissionDecisionMsg)
	if msg.Decision != DecisionDeny {
		t.Errorf("Esc should deny, got %q", msg.Decision)
	}
}
