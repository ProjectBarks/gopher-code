package lifecycle

import (
	"testing"
)

func TestDoublePress_FirstPressReturnsFalseAndPending(t *testing.T) {
	dp := NewDoublePress()

	fired, cmd := dp.Press()
	if fired {
		t.Error("First press should not fire")
	}
	if cmd == nil {
		t.Error("First press should return a timeout cmd")
	}
	if !dp.Pending() {
		t.Error("Should be pending after first press")
	}
}

func TestDoublePress_SecondPressReturnsTrueAndClearsPending(t *testing.T) {
	dp := NewDoublePress()

	dp.Press() // first
	fired, cmd := dp.Press()
	if !fired {
		t.Error("Second press should fire")
	}
	if cmd != nil {
		t.Error("Second press should return nil cmd")
	}
	if dp.Pending() {
		t.Error("Should not be pending after double press")
	}
}

func TestDoublePress_ResetClearsPending(t *testing.T) {
	dp := NewDoublePress()

	dp.Press() // first
	if !dp.Pending() {
		t.Fatal("Should be pending after first press")
	}
	dp.Reset()
	if dp.Pending() {
		t.Error("Should not be pending after Reset")
	}

	// After reset, next press is treated as first press
	fired, _ := dp.Press()
	if fired {
		t.Error("Press after Reset should be first press, not fire")
	}
}

func TestDoublePress_TimeoutResetMsg(t *testing.T) {
	dp := NewDoublePress()

	_, cmd := dp.Press()
	if cmd == nil {
		t.Fatal("Expected timeout cmd from first press")
	}
	// Execute the cmd to get the reset message
	msg := cmd()
	// Forward the message to Update
	dp.Update(msg)

	if dp.Pending() {
		t.Error("Should not be pending after timeout reset")
	}
}

func TestDoublePress_StaleTimeoutIgnored(t *testing.T) {
	dp := NewDoublePress()

	// First press generates a timeout
	_, cmd1 := dp.Press()
	// Reset (e.g. user pressed another key)
	dp.Reset()

	// Second sequence: new first press
	dp.Press()
	if !dp.Pending() {
		t.Fatal("Should be pending after new first press")
	}

	// The old timeout fires — should be ignored (stale generation)
	if cmd1 != nil {
		msg := cmd1()
		dp.Update(msg)
	}

	// Should still be pending from the new sequence
	if !dp.Pending() {
		t.Error("Stale timeout should not clear pending from new sequence")
	}
}

func TestDoublePress_TimeoutConstant(t *testing.T) {
	if DoublePressTimeoutMS != 800 {
		t.Errorf("DoublePressTimeoutMS should be 800, got %d", DoublePressTimeoutMS)
	}
}
