package events

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestClickMsg_Propagation(t *testing.T) {
	e := &ClickMsg{Col: 10, Row: 5}
	if e.Stopped() {
		t.Error("should not be stopped initially")
	}
	e.StopPropagation()
	if !e.Stopped() {
		t.Error("should be stopped after StopPropagation")
	}
}

func TestClickMsg_Fields(t *testing.T) {
	e := ClickMsg{Col: 10, Row: 5, LocalCol: 3, LocalRow: 2, CellIsBlank: true}
	if e.Col != 10 || e.Row != 5 {
		t.Error("wrong coordinates")
	}
	if e.LocalCol != 3 || e.LocalRow != 2 {
		t.Error("wrong local coordinates")
	}
	if !e.CellIsBlank {
		t.Error("should be blank")
	}
}

func TestInputMsg_Propagation(t *testing.T) {
	e := &InputMsg{Key: tea.KeyPressMsg{Code: 'a'}}
	if e.Stopped() {
		t.Error("should not be stopped initially")
	}
	e.StopPropagation()
	if !e.Stopped() {
		t.Error("should be stopped")
	}
}

func TestNewInputMsg_Printable(t *testing.T) {
	msg := NewInputMsg(tea.KeyPressMsg{Code: 'x'})
	if msg.Input != "x" {
		t.Errorf("input = %q, want 'x'", msg.Input)
	}
}

func TestNewInputMsg_Special(t *testing.T) {
	msg := NewInputMsg(tea.KeyPressMsg{Code: tea.KeyEnter})
	if msg.Input != "" {
		t.Errorf("special key should have empty input, got %q", msg.Input)
	}
}

func TestNewInputMsg_WithModifier(t *testing.T) {
	msg := NewInputMsg(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	if msg.Input != "" {
		t.Errorf("ctrl+a should have empty input, got %q", msg.Input)
	}
}

func TestFocusChangeMsg(t *testing.T) {
	msg := FocusChangeMsg{Focused: true, ComponentID: "prompt"}
	if !msg.Focused {
		t.Error("should be focused")
	}
	if msg.ComponentID != "prompt" {
		t.Error("wrong component ID")
	}
}

func TestScrollMsg(t *testing.T) {
	msg := ScrollMsg{Delta: -3, Col: 40, Row: 10}
	if msg.Delta != -3 {
		t.Error("wrong delta")
	}
}

func TestPasteMsg(t *testing.T) {
	msg := PasteMsg{Text: "hello world"}
	if msg.Text != "hello world" {
		t.Error("wrong text")
	}
}

func TestNewResizeMsg(t *testing.T) {
	msg := NewResizeMsg(tea.WindowSizeMsg{Width: 120, Height: 40})
	if msg.Width != 120 || msg.Height != 40 {
		t.Errorf("got %dx%d", msg.Width, msg.Height)
	}
}

func TestTerminalFocusMsg(t *testing.T) {
	msg := TerminalFocusMsg{Focused: false}
	if msg.Focused {
		t.Error("should not be focused")
	}
}

func TestHandled(t *testing.T) {
	original := tea.KeyPressMsg{Code: 'a'}
	handled := Handled{Msg: original}
	if _, ok := handled.Msg.(tea.KeyPressMsg); !ok {
		t.Error("wrapped msg should be KeyPressMsg")
	}
}

func TestIsKeyPress(t *testing.T) {
	if !IsKeyPress(tea.KeyPressMsg{Code: 'a'}) {
		t.Error("should detect key press")
	}
	if IsKeyPress(tea.WindowSizeMsg{}) {
		t.Error("should not detect window size as key press")
	}
}

func TestIsClick(t *testing.T) {
	if !IsClick(ClickMsg{Col: 1, Row: 1}) {
		t.Error("should detect click (value)")
	}
	if !IsClick(&ClickMsg{Col: 1, Row: 1}) {
		t.Error("should detect click (pointer)")
	}
	if IsClick(tea.KeyPressMsg{}) {
		t.Error("should not detect key as click")
	}
}

func TestIsScroll(t *testing.T) {
	if !IsScroll(ScrollMsg{Delta: 1}) {
		t.Error("should detect scroll")
	}
	if IsScroll(tea.KeyPressMsg{}) {
		t.Error("should not detect key as scroll")
	}
}

func TestKeyIs(t *testing.T) {
	if !KeyIs(tea.KeyPressMsg{Code: 'q'}, 'q') {
		t.Error("should match 'q'")
	}
	if KeyIs(tea.KeyPressMsg{Code: 'a'}, 'q') {
		t.Error("should not match 'a' as 'q'")
	}
	if KeyIs(tea.WindowSizeMsg{}, 'q') {
		t.Error("non-key msg should not match")
	}
}

func TestKeyHasMod(t *testing.T) {
	if !KeyHasMod(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl}, tea.ModCtrl) {
		t.Error("should detect ctrl")
	}
	if KeyHasMod(tea.KeyPressMsg{Code: 'c'}, tea.ModCtrl) {
		t.Error("should not detect ctrl when absent")
	}
}
