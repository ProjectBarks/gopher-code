package components

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestInputPaneCreation(t *testing.T) {
	ip := NewInputPane()
	if ip == nil {
		t.Fatal("InputPane should not be nil")
	}
	if ip.Value() != "" {
		t.Error("Initial value should be empty")
	}
	if ip.Focused() {
		t.Error("Should not be focused initially")
	}
}

func TestInputPaneSetValue(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello")
	if ip.Value() != "hello" {
		t.Errorf("Expected 'hello', got %q", ip.Value())
	}
}

func TestInputPaneClear(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("test")
	ip.Clear()
	if ip.Value() != "" {
		t.Error("Value should be empty after Clear")
	}
}

func TestInputPaneFocus(t *testing.T) {
	ip := NewInputPane()
	ip.Focus()
	if !ip.Focused() {
		t.Error("Should be focused after Focus()")
	}
	ip.Blur()
	if ip.Focused() {
		t.Error("Should not be focused after Blur()")
	}
}

func TestInputPaneCharacterInput(t *testing.T) {
	ip := NewInputPane()
	ip.Focus()
	ip.Update(tea.KeyPressMsg{Text: "h"})
	ip.Update(tea.KeyPressMsg{Text: "i"})
	if ip.Value() != "hi" {
		t.Errorf("Expected 'hi', got %q", ip.Value())
	}
}

func TestInputPaneUnicodeInput(t *testing.T) {
	ip := NewInputPane()
	ip.Focus()
	ip.Update(tea.KeyPressMsg{Text: "日"})
	ip.Update(tea.KeyPressMsg{Text: "本"})
	if ip.Value() != "日本" {
		t.Errorf("Expected '日本', got %q", ip.Value())
	}
	if ip.cursor != 2 {
		t.Errorf("Expected cursor at rune position 2, got %d", ip.cursor)
	}
}

func TestInputPaneBackspace(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("abc")
	ip.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if ip.Value() != "ab" {
		t.Errorf("Expected 'ab', got %q", ip.Value())
	}
}

func TestInputPaneBackspaceEmpty(t *testing.T) {
	ip := NewInputPane()
	ip.Update(tea.KeyPressMsg{Code: tea.KeyBackspace})
	if ip.Value() != "" {
		t.Error("Backspace on empty should stay empty")
	}
}

func TestInputPaneDelete(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("abc")
	ip.cursor = 1 // Position after 'a'
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
	if ip.Value() != "ac" {
		t.Errorf("Expected 'ac', got %q", ip.Value())
	}
}

func TestInputPaneCursorMovement(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello")
	// Cursor at end (5)

	ip.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if ip.cursor != 4 {
		t.Errorf("Expected cursor 4, got %d", ip.cursor)
	}

	ip.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if ip.cursor != 5 {
		t.Errorf("Expected cursor 5, got %d", ip.cursor)
	}

	ip.Update(tea.KeyPressMsg{Code: tea.KeyHome})
	if ip.cursor != 0 {
		t.Errorf("Expected cursor 0, got %d", ip.cursor)
	}

	ip.Update(tea.KeyPressMsg{Code: tea.KeyEnd})
	if ip.cursor != 5 {
		t.Errorf("Expected cursor 5, got %d", ip.cursor)
	}
}

func TestInputPaneCtrlA(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello")
	ip.Update(tea.KeyPressMsg{Code: 'a', Mod: tea.ModCtrl})
	if ip.cursor != 0 {
		t.Error("Ctrl+A should move to beginning")
	}
}

func TestInputPaneCtrlE(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello")
	ip.cursor = 0
	ip.Update(tea.KeyPressMsg{Code: 'e', Mod: tea.ModCtrl})
	if ip.cursor != 5 {
		t.Error("Ctrl+E should move to end")
	}
}

func TestInputPaneCtrlK(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello world")
	ip.cursor = 5
	ip.Update(tea.KeyPressMsg{Code: 'k', Mod: tea.ModCtrl})
	if ip.Value() != "hello" {
		t.Errorf("Ctrl+K should kill to end, got %q", ip.Value())
	}
}

func TestInputPaneCtrlU(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello world")
	ip.cursor = 5
	ip.Update(tea.KeyPressMsg{Code: 'u', Mod: tea.ModCtrl})
	if ip.Value() != " world" {
		t.Errorf("Ctrl+U should kill to beginning, got %q", ip.Value())
	}
	if ip.cursor != 0 {
		t.Error("Cursor should be at 0 after Ctrl+U")
	}
}

func TestInputPaneCtrlW(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello world")
	// Cursor at end
	ip.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if ip.Value() != "hello " {
		t.Errorf("Ctrl+W should delete word backward, got %q", ip.Value())
	}
}

func TestInputPaneCtrlWMultipleSpaces(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hello   world")
	ip.cursor = 8 // After "hello   "
	ip.Update(tea.KeyPressMsg{Code: 'w', Mod: tea.ModCtrl})
	if ip.Value() != "world" {
		t.Errorf("Ctrl+W should skip spaces then delete word, got %q", ip.Value())
	}
}

func TestInputPaneSubmit(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("test command")
	_, cmd := ip.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd == nil {
		t.Fatal("Enter should produce a command")
	}
	msg := cmd()
	submit, ok := msg.(SubmitMsg)
	if !ok {
		t.Fatalf("Expected SubmitMsg, got %T", msg)
	}
	if submit.Text != "test command" {
		t.Errorf("Expected 'test command', got %q", submit.Text)
	}
	if ip.Value() != "" {
		t.Error("Input should be cleared after submit")
	}
}

func TestInputPaneSubmitEmpty(t *testing.T) {
	ip := NewInputPane()
	_, cmd := ip.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	if cmd != nil {
		t.Error("Enter on empty input should not produce a command")
	}
}

func TestInputPaneHistory(t *testing.T) {
	ip := NewInputPane()
	ip.AddToHistory("cmd1")
	ip.AddToHistory("cmd2")
	ip.AddToHistory("cmd3")

	// Navigate up through history (input is empty)
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "cmd3" {
		t.Errorf("Expected 'cmd3', got %q", ip.Value())
	}

	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "cmd2" {
		t.Errorf("Expected 'cmd2', got %q", ip.Value())
	}

	// Navigate back down
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "cmd3" {
		t.Errorf("Expected 'cmd3', got %q", ip.Value())
	}

	// Go past end to restore original
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "" {
		t.Errorf("Expected empty (original), got %q", ip.Value())
	}
}

func TestInputPaneHistorySavesInput(t *testing.T) {
	ip := NewInputPane()
	ip.AddToHistory("old command")
	// Start with empty input so Up triggers history
	ip.SetValue("")

	// Up enters history
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "old command" {
		t.Errorf("Expected 'old command', got %q", ip.Value())
	}

	// Down past end restores saved empty input
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "" {
		t.Errorf("Expected empty (restored), got %q", ip.Value())
	}
}

func TestInputPaneHistoryWithDraftPreservation(t *testing.T) {
	ip := NewInputPane()
	ip.AddToHistory("old")
	ip.SetValue("new text")

	// Up with text should save draft and navigate to history
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "old" {
		t.Errorf("Up should navigate to history entry, got %q", ip.Value())
	}

	// Down should restore draft
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "new text" {
		t.Errorf("Down should restore draft 'new text', got %q", ip.Value())
	}
}

func TestInputPaneView(t *testing.T) {
	ip := NewInputPane()
	ip.SetSize(80, 3)
	ip.SetValue("test")
	view := ip.View()
	if view.Content == "" {
		t.Error("View should produce output")
	}
}

func TestInputPaneViewWithCursor(t *testing.T) {
	ip := NewInputPane()
	ip.SetSize(80, 3)
	ip.Focus()
	ip.SetValue("test")
	view := ip.View()
	if view.Content == "" {
		t.Error("Focused view should produce output")
	}
}

func TestInputPaneInsertAtCursor(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("hllo")
	ip.cursor = 1
	ip.Update(tea.KeyPressMsg{Text: "e"})
	if ip.Value() != "hello" {
		t.Errorf("Expected 'hello', got %q", ip.Value())
	}
	if ip.cursor != 2 {
		t.Errorf("Expected cursor 2, got %d", ip.cursor)
	}
}

func TestInputPaneCursorBounds(t *testing.T) {
	ip := NewInputPane()
	ip.SetValue("abc")

	// Left at beginning stays at 0
	ip.cursor = 0
	ip.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
	if ip.cursor != 0 {
		t.Error("Left at 0 should stay at 0")
	}

	// Right at end stays at end
	ip.cursor = 3
	ip.Update(tea.KeyPressMsg{Code: tea.KeyRight})
	if ip.cursor != 3 {
		t.Error("Right at end should stay at end")
	}
}

func TestInputPaneSetSize(t *testing.T) {
	ip := NewInputPane()
	ip.SetSize(100, 5)
	if ip.width != 100 || ip.height != 5 {
		t.Error("SetSize should update dimensions")
	}
}

func TestInputPaneInit(t *testing.T) {
	ip := NewInputPane()
	cmd := ip.Init()
	if cmd == nil {
		t.Error("Init should return cursor blink command")
	}
}

// TestInputPaneDraftPreservationEndToEnd is an integration test verifying the
// full draft-preservation cycle: type partial text, press Up to enter history,
// then press Down to return -- the partial text must be restored exactly.
func TestInputPaneDraftPreservationEndToEnd(t *testing.T) {
	ip := NewInputPane()
	ip.AddToHistory("previous command")
	ip.AddToHistory("another command")

	// Simulate typing "partial" character by character.
	for _, ch := range "partial" {
		ip.Update(tea.KeyPressMsg{Text: string(ch)})
	}
	if ip.Value() != "partial" {
		t.Fatalf("Setup: expected 'partial', got %q", ip.Value())
	}

	// Press Up -- should save "partial" as draft and show newest history entry.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "another command" {
		t.Errorf("After Up: expected 'another command', got %q", ip.Value())
	}

	// Press Up again -- should show older entry.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "previous command" {
		t.Errorf("After second Up: expected 'previous command', got %q", ip.Value())
	}

	// Press Down -- back to newest entry.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "another command" {
		t.Errorf("After Down: expected 'another command', got %q", ip.Value())
	}

	// Press Down again -- should restore draft "partial".
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "partial" {
		t.Errorf("After second Down: expected draft 'partial' restored, got %q", ip.Value())
	}

	// Cursor should be at end of restored text.
	if ip.cursor != len([]rune("partial")) {
		t.Errorf("Cursor should be at end of restored draft, got %d", ip.cursor)
	}
}

// TestInputPaneModeFilterHistory verifies that history navigation respects
// the ModeFilter on the underlying InputHistory (e.g. bash-only entries).
func TestInputPaneModeFilterHistory(t *testing.T) {
	ip := NewInputPane()
	ip.History.ModeFilter = "!"

	ip.AddToHistory("!ls -la")
	ip.AddToHistory("regular prompt")
	ip.AddToHistory("!git status")

	// Up should skip "regular prompt" and show "!git status" (newest matching).
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "!git status" {
		t.Errorf("Expected '!git status', got %q", ip.Value())
	}

	// Up again should show "!ls -la" (next matching, skipping "regular prompt").
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "!ls -la" {
		t.Errorf("Expected '!ls -la', got %q", ip.Value())
	}

	// Down back to "!git status".
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "!git status" {
		t.Errorf("Expected '!git status', got %q", ip.Value())
	}

	// Down restores draft (empty).
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "" {
		t.Errorf("Expected empty draft, got %q", ip.Value())
	}
}

// TestT417_ArrowKeyHistoryIntegration is the integration test for T417:
// exercises the full arrow-key history fix through InputPane.Update(),
// validating draft preservation + mode filtering + submit-adds-to-history
// in a single end-to-end flow reachable from main() via app.go -> InputPane.
func TestT417_ArrowKeyHistoryIntegration(t *testing.T) {
	ip := NewInputPane()
	ip.SetSize(80, 3)
	ip.Focus()

	// === Phase 1: Submit commands to build history (mirrors app.go flow) ===
	// Type "first command" and submit.
	for _, ch := range "first command" {
		ip.Update(tea.KeyPressMsg{Text: string(ch)})
	}
	ip.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	// After submit, InputPane clears and resets history cursor.
	// The app layer calls AddToHistory; simulate that here.
	ip.AddToHistory("first command")

	for _, ch := range "second command" {
		ip.Update(tea.KeyPressMsg{Text: string(ch)})
	}
	ip.Update(tea.KeyPressMsg{Code: tea.KeyEnter})
	ip.AddToHistory("second command")

	// === Phase 2: Draft preservation ===
	// Type a partial draft, then navigate history.
	for _, ch := range "my draft" {
		ip.Update(tea.KeyPressMsg{Text: string(ch)})
	}

	// Up: save draft, show newest history entry.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "second command" {
		t.Fatalf("Up should show 'second command', got %q", ip.Value())
	}

	// Up: show older entry.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "first command" {
		t.Fatalf("Up should show 'first command', got %q", ip.Value())
	}

	// Up at oldest: no change (no wrap-around).
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "first command" {
		t.Fatalf("Up at oldest should stay on 'first command', got %q", ip.Value())
	}

	// Down: back to newest.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "second command" {
		t.Fatalf("Down should show 'second command', got %q", ip.Value())
	}

	// Down past newest: restore draft.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "my draft" {
		t.Fatalf("Down past newest should restore draft 'my draft', got %q", ip.Value())
	}

	// === Phase 3: Mode filtering ===
	ip.Clear()
	ip.History.Reset()

	// Add mixed-mode entries.
	ip.AddToHistory("!bash ls")
	ip.AddToHistory("normal prompt")
	ip.AddToHistory("!bash git status")

	// Enable bash-mode filter.
	ip.History.ModeFilter = "!"

	// Up: should skip "normal prompt" and show "!bash git status".
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "!bash git status" {
		t.Fatalf("Filtered Up should show '!bash git status', got %q", ip.Value())
	}

	// Up: should show "!bash ls" (skipping "normal prompt").
	ip.Update(tea.KeyPressMsg{Code: tea.KeyUp})
	if ip.Value() != "!bash ls" {
		t.Fatalf("Filtered Up should show '!bash ls', got %q", ip.Value())
	}

	// Down: back to newest filtered.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "!bash git status" {
		t.Fatalf("Filtered Down should show '!bash git status', got %q", ip.Value())
	}

	// Down: restore empty draft.
	ip.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip.Value() != "" {
		t.Fatalf("Down past newest should restore empty draft, got %q", ip.Value())
	}

	// Clear mode filter for next use.
	ip.History.ModeFilter = ""

	// === Phase 4: Down on empty history is no-op ===
	ip2 := NewInputPane()
	ip2.SetSize(80, 3)
	ip2.Focus()
	ip2.Update(tea.KeyPressMsg{Code: tea.KeyDown})
	if ip2.Value() != "" {
		t.Fatalf("Down on empty history should be no-op, got %q", ip2.Value())
	}
}
