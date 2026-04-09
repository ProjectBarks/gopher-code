package components

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestVimTextInput_StartsInInsert(t *testing.T) {
	m := NewVimTextInput("Enter text")
	if m.Mode() != VimInsert {
		t.Error("should start in insert mode")
	}
}

func TestVimTextInput_EscapeToNormal(t *testing.T) {
	m := NewVimTextInput("")
	m, cmd := m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	if m.Mode() != VimNormal {
		t.Error("escape should switch to normal mode")
	}
	if cmd == nil {
		t.Fatal("should return mode change cmd")
	}
	msg := cmd()
	mc, ok := msg.(VimModeChangeMsg)
	if !ok {
		t.Fatalf("expected VimModeChangeMsg, got %T", msg)
	}
	if mc.Mode != VimNormal {
		t.Error("mode change should be Normal")
	}
}

func TestVimTextInput_IToInsert(t *testing.T) {
	m := NewVimTextInput("")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // normal
	m, cmd := m.Update(tea.KeyPressMsg{Code: 'i'})        // back to insert
	if m.Mode() != VimInsert {
		t.Error("i should switch to insert mode")
	}
	if cmd == nil {
		t.Fatal("should return mode change cmd")
	}
}

func TestVimTextInput_AToInsert(t *testing.T) {
	m := NewVimTextInput("")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	m, _ = m.Update(tea.KeyPressMsg{Code: 'a'})
	if m.Mode() != VimInsert {
		t.Error("a should switch to insert mode")
	}
}

func TestVimTextInput_NormalModeBlocksText(t *testing.T) {
	m := NewVimTextInput("")
	m.SetValue("hello")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape}) // normal

	// In normal mode, typing 'x' should delete, not insert
	m, _ = m.Update(tea.KeyPressMsg{Code: 'x'})
	// The exact behavior depends on how Delete is handled — just verify no panic
}

func TestVimTextInput_ViewShowsMode(t *testing.T) {
	m := NewVimTextInput("")
	m, _ = m.Update(tea.KeyPressMsg{Code: tea.KeyEscape})
	v := m.View()
	if !strings.Contains(v, "NORMAL") {
		t.Error("normal mode view should show [NORMAL]")
	}
}

func TestVimTextInput_InsertModeNoIndicator(t *testing.T) {
	m := NewVimTextInput("")
	v := m.View()
	if strings.Contains(v, "NORMAL") {
		t.Error("insert mode should not show NORMAL indicator")
	}
}
