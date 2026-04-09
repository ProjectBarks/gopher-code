package prompt_input

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestNew(t *testing.T) {
	m := New("Type a message…")
	if m.Value() != "" {
		t.Error("should start empty")
	}
	if !m.IsEmpty() {
		t.Error("should be empty")
	}
	if m.placeholder != "Type a message…" {
		t.Error("wrong placeholder")
	}
}

func TestModel_Typing(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'h'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	if m.Value() != "hi" {
		t.Errorf("value = %q", m.Value())
	}
	if m.cursorPos != 2 {
		t.Errorf("cursor = %d", m.cursorPos)
	}
}

func TestModel_Submit(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'g'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'o'})

	_, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should return cmd")
	}
	msg := cmd()
	sub, ok := msg.(SubmitMsg)
	if !ok {
		t.Fatalf("expected SubmitMsg, got %T", msg)
	}
	if sub.Text != "go" {
		t.Errorf("text = %q", sub.Text)
	}
}

func TestModel_SubmitClearsInput(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a'})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if m.Value() != "" {
		t.Error("should be cleared after submit")
	}
}

func TestModel_Backspace(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'b'})
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if m.Value() != "a" {
		t.Errorf("after backspace: %q", m.Value())
	}
}

func TestModel_CursorMovement(t *testing.T) {
	m := New("")
	for _, ch := range "hello" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if m.cursorPos != 4 {
		t.Errorf("cursor after left = %d", m.cursorPos)
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if m.cursorPos != 0 {
		t.Error("Home should go to start")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if m.cursorPos != 5 {
		t.Error("End should go to end")
	}
}

func TestModel_CtrlA_CtrlE(t *testing.T) {
	m := New("")
	for _, ch := range "test" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	if m.cursorPos != 0 {
		t.Error("Ctrl+A should go to start")
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	if m.cursorPos != 4 {
		t.Error("Ctrl+E should go to end")
	}
}

func TestModel_CtrlK(t *testing.T) {
	m := New("")
	for _, ch := range "hello world" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}
	// Move cursor to position 5 (after "hello")
	m.cursorPos = 5
	m, _ = m.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if m.Value() != "hello" {
		t.Errorf("after Ctrl+K: %q", m.Value())
	}
}

func TestModel_CtrlU(t *testing.T) {
	m := New("")
	for _, ch := range "hello world" {
		m, _ = m.Update(tea.KeyPressMsg{Code: ch})
	}
	m.cursorPos = 6
	m, _ = m.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if m.Value() != "world" {
		t.Errorf("after Ctrl+U: %q", m.Value())
	}
	if m.cursorPos != 0 {
		t.Error("cursor should be at start")
	}
}

func TestModel_CtrlC_Empty_Cancel(t *testing.T) {
	m := New("")
	_, cmd := m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd == nil {
		t.Fatal("Ctrl+C on empty should return cmd")
	}
	if _, ok := cmd().(CancelMsg); !ok {
		t.Error("should be CancelMsg")
	}
}

func TestModel_CtrlC_NonEmpty_Clears(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if m.Value() != "" {
		t.Error("Ctrl+C on non-empty should clear")
	}
}

func TestModel_History(t *testing.T) {
	m := New("")
	m.SetHistory([]string{"first", "second", "third"})

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Value() != "third" {
		t.Errorf("up 1: %q", m.Value())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if m.Value() != "second" {
		t.Errorf("up 2: %q", m.Value())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Value() != "third" {
		t.Errorf("down: %q", m.Value())
	}

	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if m.Value() != "" {
		t.Error("down past end should clear")
	}
}

func TestModel_Reset(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	m.Reset()
	if m.Value() != "" {
		t.Error("should be empty after reset")
	}
	if m.cursorPos != 0 {
		t.Error("cursor should be 0 after reset")
	}
}

func TestModel_View(t *testing.T) {
	m := New("Type here…")
	v := m.View()
	if !strings.Contains(v, "❯") {
		t.Error("should show prompt")
	}
	if !strings.Contains(v, "Type here") {
		t.Error("empty input should show placeholder")
	}
}

func TestModel_View_WithText(t *testing.T) {
	m := New("")
	m, _ = m.Update(tea.KeyPressMsg{Code: 'h'})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'i'})
	v := m.View()
	if !strings.Contains(v, "hi") {
		t.Error("should show typed text")
	}
}

func TestModel_View_PermissionMode(t *testing.T) {
	m := New("")
	m.SetPermissionMode("plan")
	v := m.View()
	if !strings.Contains(v, "plan") {
		t.Error("should show permission mode")
	}
}

func TestModel_NotFocused(t *testing.T) {
	m := New("")
	m.SetFocused(false)
	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	if m.Value() != "" {
		t.Error("unfocused should not accept input")
	}
}

func TestModel_Suggestions(t *testing.T) {
	m := New("")
	m.SetSuggestions([]string{"/help", "/model", "/clear"})
	if !m.showSuggestions {
		t.Error("should show suggestions")
	}
}

func TestInputMode_Constants(t *testing.T) {
	if ModeNormal != "normal" {
		t.Error("wrong")
	}
	if ModeInsert != "insert" {
		t.Error("wrong")
	}
}
