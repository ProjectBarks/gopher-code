package hooks

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

func TestFocusTracker_Default(t *testing.T) {
	f := NewFocusTracker()
	if !f.IsFocused() {
		t.Error("should default to focused")
	}
	if f.State() != FocusUnknown {
		t.Errorf("state = %q, want unknown", f.State())
	}
}

func TestFocusTracker_BlurAndFocus(t *testing.T) {
	f := NewFocusTracker()

	changed := f.HandleMsg(tea.BlurMsg{})
	if !changed {
		t.Error("blur should change state")
	}
	if f.IsFocused() {
		t.Error("should not be focused after blur")
	}
	if f.State() != FocusBlurred {
		t.Errorf("state = %q, want blurred", f.State())
	}

	changed = f.HandleMsg(tea.FocusMsg{})
	if !changed {
		t.Error("focus should change state")
	}
	if !f.IsFocused() {
		t.Error("should be focused after focus")
	}
	if f.State() != FocusFocused {
		t.Errorf("state = %q, want focused", f.State())
	}
}

func TestFocusTracker_DuplicateNoChange(t *testing.T) {
	f := NewFocusTracker()
	f.HandleMsg(tea.FocusMsg{})

	changed := f.HandleMsg(tea.FocusMsg{})
	if changed {
		t.Error("duplicate focus should not change")
	}
}

func TestFocusTracker_UnrelatedMsg(t *testing.T) {
	f := NewFocusTracker()
	changed := f.HandleMsg(tea.WindowSizeMsg{Width: 80, Height: 24})
	if changed {
		t.Error("unrelated msg should not change focus")
	}
}

func TestClock_Basic(t *testing.T) {
	c := NewClock()
	if c.IsActive() {
		t.Error("should not be active initially")
	}
	if c.Now() < 0 {
		t.Error("Now() should be non-negative")
	}
}

func TestClock_StartStop(t *testing.T) {
	c := NewClock()
	cmd := c.Start()
	if cmd == nil {
		t.Error("Start should return a cmd")
	}
	if !c.IsActive() {
		t.Error("should be active after Start")
	}

	c.Stop()
	if c.IsActive() {
		t.Error("should not be active after Stop")
	}

	// Tick returns nil when stopped
	if c.Tick() != nil {
		t.Error("Tick should return nil when stopped")
	}
}

func TestClock_TickWhenActive(t *testing.T) {
	c := NewClock()
	c.Start()
	cmd := c.Tick()
	if cmd == nil {
		t.Error("Tick should return cmd when active")
	}
}

func TestClock_SetFocused(t *testing.T) {
	c := NewClock()
	c.SetFocused(false)
	// Interval should be BlurredFrameIntervalMS
	c.mu.Lock()
	interval := c.interval
	c.mu.Unlock()
	if interval.Milliseconds() != BlurredFrameIntervalMS {
		t.Errorf("blurred interval = %dms, want %dms", interval.Milliseconds(), BlurredFrameIntervalMS)
	}

	c.SetFocused(true)
	c.mu.Lock()
	interval = c.interval
	c.mu.Unlock()
	if interval.Milliseconds() != FrameIntervalMS {
		t.Errorf("focused interval = %dms, want %dms", interval.Milliseconds(), FrameIntervalMS)
	}
}

func TestCursorState(t *testing.T) {
	c := NewCursorState()
	if !c.IsVisible() {
		t.Error("should default to visible")
	}

	c.Hide()
	if c.IsVisible() {
		t.Error("should be hidden after Hide")
	}

	c.Show()
	if !c.IsVisible() {
		t.Error("should be visible after Show")
	}
}

func TestFocusState_Constants(t *testing.T) {
	if FocusUnknown != "unknown" {
		t.Error("wrong")
	}
	if FocusFocused != "focused" {
		t.Error("wrong")
	}
	if FocusBlurred != "blurred" {
		t.Error("wrong")
	}
}

func TestFrameIntervals(t *testing.T) {
	if FrameIntervalMS != 16 {
		t.Errorf("FrameIntervalMS = %d", FrameIntervalMS)
	}
	if BlurredFrameIntervalMS != 32 {
		t.Errorf("BlurredFrameIntervalMS = %d", BlurredFrameIntervalMS)
	}
}
