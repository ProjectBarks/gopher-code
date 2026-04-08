package ui

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/components"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/lifecycle"
)

// TestLifecycleDoublePress_IntegrationThroughAppModel verifies that the
// lifecycle.DoublePress detector is wired into AppModel and produces the
// correct quit/no-quit behavior through the real code path.
//
// This is an integration test for T414 — it tests DoublePress through the
// actual handleKey path in AppModel, not in isolation.
func TestLifecycleDoublePress_IntegrationThroughAppModel(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Verify the DoublePress detector is initialized.
	if app.ctrlCExit == nil {
		t.Fatal("ctrlCExit should be initialized by NewAppModel")
	}

	// Dismiss welcome screen by submitting, then completing the turn.
	app.Update(components.SubmitMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})

	// --- First Ctrl+C on empty input: should NOT quit, should arm pending ---
	_, cmd1 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !app.ctrlCExit.Pending() {
		t.Error("ctrlCExit should be pending after first Ctrl+C on empty input")
	}
	// The returned cmd should be non-nil (it's the 800ms timeout tick).
	if cmd1 == nil {
		t.Error("first Ctrl+C should return a timeout cmd from DoublePress")
	}

	// --- Second Ctrl+C within window: should quit ---
	_, cmd2 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd2 == nil {
		t.Fatal("double Ctrl+C should produce a quit cmd")
	}
	msg2 := cmd2()
	if _, ok := msg2.(tea.QuitMsg); !ok {
		t.Errorf("expected QuitMsg from double Ctrl+C, got %T", msg2)
	}
}

// TestLifecycleDoublePress_TimeoutResetThroughAppModel verifies that the
// 800ms timeout reset message is correctly forwarded through AppModel.Update
// and clears the pending state.
func TestLifecycleDoublePress_TimeoutResetThroughAppModel(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Dismiss welcome.
	app.Update(components.SubmitMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})

	// First Ctrl+C arms the pending state and returns a timeout cmd.
	_, cmd := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !app.ctrlCExit.Pending() {
		t.Fatal("should be pending after first Ctrl+C")
	}
	if cmd == nil {
		t.Fatal("expected timeout cmd from first Ctrl+C")
	}

	// Execute the timeout cmd to get the reset message, then forward it
	// through AppModel.Update (which calls ctrlCExit.Update internally).
	resetMsg := cmd()
	app.Update(resetMsg)

	// After timeout, pending should be cleared.
	if app.ctrlCExit.Pending() {
		t.Error("ctrlCExit should not be pending after timeout reset")
	}

	// After timeout, a single Ctrl+C should NOT quit (it's a fresh first press).
	_, cmd3 := app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if cmd3 != nil {
		msg3 := cmd3()
		if _, ok := msg3.(tea.QuitMsg); ok {
			t.Error("single Ctrl+C after timeout should not quit")
		}
	}
}

// TestLifecycleDoublePress_OtherKeyResetsThroughAppModel verifies that
// pressing a non-Ctrl+C key while pending resets the DoublePress state.
func TestLifecycleDoublePress_OtherKeyResetsThroughAppModel(t *testing.T) {
	config := session.DefaultConfig()
	sess := session.New(config, "/tmp")
	app := NewAppModel(sess, nil)
	app.Update(tea.WindowSizeMsg{Width: 80, Height: 24})

	// Dismiss welcome.
	app.Update(components.SubmitMsg{Text: "hello"})
	app.Update(TurnCompleteMsg{})

	// First Ctrl+C arms pending.
	app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	if !app.ctrlCExit.Pending() {
		t.Fatal("should be pending after first Ctrl+C")
	}

	// Any other key resets it.
	app.Update(tea.KeyPressMsg{Code: 'a', Text: "a"})
	if app.ctrlCExit.Pending() {
		t.Error("pressing another key should reset ctrlCExit pending state")
	}
}

// TestLifecycleDoublePress_TimeoutConstant verifies the DoublePress timeout
// matches the TS DOUBLE_PRESS_TIMEOUT_MS = 800.
func TestLifecycleDoublePress_TimeoutConstant(t *testing.T) {
	if lifecycle.DoublePressTimeoutMS != 800 {
		t.Errorf("DoublePressTimeoutMS = %d, want 800", lifecycle.DoublePressTimeoutMS)
	}
}
