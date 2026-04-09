package ink

import "testing"

func TestFocusManager_Basic(t *testing.T) {
	fm := NewFocusManager()
	if fm.ActiveID() != "" {
		t.Error("should start with no focus")
	}

	fm.Focus("input")
	if fm.ActiveID() != "input" {
		t.Errorf("active = %q", fm.ActiveID())
	}
	if !fm.IsFocused("input") {
		t.Error("input should be focused")
	}
	if fm.IsFocused("other") {
		t.Error("other should not be focused")
	}
}

func TestFocusManager_FocusSwitch(t *testing.T) {
	fm := NewFocusManager()
	fm.Focus("a")
	fm.Focus("b")
	if fm.ActiveID() != "b" {
		t.Error("should focus b")
	}
	// "a" should be on the stack
	prev := fm.RestorePrevious()
	if prev != "a" {
		t.Errorf("restored = %q, want a", prev)
	}
	if fm.ActiveID() != "a" {
		t.Error("should restore to a")
	}
}

func TestFocusManager_Blur(t *testing.T) {
	fm := NewFocusManager()
	fm.Focus("input")
	fm.Blur()
	if fm.ActiveID() != "" {
		t.Error("should have no focus after blur")
	}
}

func TestFocusManager_RestoreEmpty(t *testing.T) {
	fm := NewFocusManager()
	prev := fm.RestorePrevious()
	if prev != "" {
		t.Error("empty stack should return empty")
	}
}

func TestFocusManager_TabCycling(t *testing.T) {
	fm := NewFocusManager()
	fm.SetTabOrder([]string{"input", "messages", "sidebar"})
	fm.Focus("input")

	next := fm.FocusNext()
	if next != "messages" {
		t.Errorf("next = %q", next)
	}

	next = fm.FocusNext()
	if next != "sidebar" {
		t.Errorf("next = %q", next)
	}

	// Wrap around
	next = fm.FocusNext()
	if next != "input" {
		t.Errorf("wrap: next = %q", next)
	}
}

func TestFocusManager_TabPrev(t *testing.T) {
	fm := NewFocusManager()
	fm.SetTabOrder([]string{"a", "b", "c"})
	fm.Focus("a")

	// Prev from first wraps to last
	prev := fm.FocusPrev()
	if prev != "c" {
		t.Errorf("prev = %q, want c", prev)
	}
}

func TestFocusManager_HandleRemoved(t *testing.T) {
	fm := NewFocusManager()
	fm.Focus("a")
	fm.Focus("b")
	fm.Focus("c")

	// Remove focused component — should restore from stack
	fm.HandleRemoved("c")
	if fm.ActiveID() != "b" {
		t.Errorf("after removing c: active = %q, want b", fm.ActiveID())
	}

	// Remove non-focused from stack
	fm.HandleRemoved("a")
	if fm.ActiveID() != "b" {
		t.Error("removing non-focused should not change active")
	}
}

func TestFocusManager_DuplicateFocus(t *testing.T) {
	fm := NewFocusManager()
	fm.Focus("a")
	fm.Focus("a") // same — should be no-op
	if fm.ActiveID() != "a" {
		t.Error("duplicate focus should keep same active")
	}
}

func TestFocusManager_EmptyTabOrder(t *testing.T) {
	fm := NewFocusManager()
	fm.Focus("x")
	next := fm.FocusNext()
	if next != "x" {
		t.Error("empty tab order should keep current")
	}
}

func TestExitMsg(t *testing.T) {
	cmd := Exit(0)
	msg := cmd()
	exitMsg, ok := msg.(ExitMsg)
	if !ok {
		t.Fatal("should be ExitMsg")
	}
	if exitMsg.Code != 0 {
		t.Errorf("code = %d", exitMsg.Code)
	}
}

func TestForceRedraw(t *testing.T) {
	cmd := ForceRedraw()
	msg := cmd()
	if _, ok := msg.(ForceRedrawMsg); !ok {
		t.Fatal("should be ForceRedrawMsg")
	}
}
