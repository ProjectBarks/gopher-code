package display

import (
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
)

func TestBlink_StartsDisabledAndVisible(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	assert.False(t, b.IsEnabled())
	assert.True(t, b.Visible(), "disabled blink should always be visible")
}

func TestBlink_ToggleCyclesVisibility(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	b.Enable()

	assert.True(t, b.Visible(), "initial state after enable should be visible (count=0, even)")

	b.Toggle()
	assert.False(t, b.Visible(), "after 1 toggle should be hidden (count=1, odd)")

	b.Toggle()
	assert.True(t, b.Visible(), "after 2 toggles should be visible (count=2, even)")

	b.Toggle()
	assert.False(t, b.Visible(), "after 3 toggles should be hidden")
}

func TestBlink_ToggleNoOpWhenDisabled(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	b.Toggle()
	assert.True(t, b.Visible(), "toggle when disabled should keep visible")
}

func TestBlink_DisableResetsToVisible(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	b.Enable()
	b.Toggle() // hidden
	assert.False(t, b.Visible())

	b.Disable()
	assert.True(t, b.Visible(), "disable should reset to visible")
	assert.False(t, b.IsEnabled())
}

func TestBlink_EnableResetsCount(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	b.Enable()
	b.Toggle() // hidden
	b.Toggle() // visible
	b.Toggle() // hidden

	// Re-enable should reset.
	b.Enable()
	assert.True(t, b.Visible(), "re-enable should reset to visible")

	b.Toggle()
	assert.False(t, b.Visible(), "first toggle after re-enable should go hidden")
}

func TestBlink_TickReturnsNilWhenDisabled(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	assert.Nil(t, b.Tick())
}

func TestBlink_TickReturnsCmdWhenEnabled(t *testing.T) {
	b := NewBlink(1, DefaultBlinkInterval)
	b.Enable()
	assert.NotNil(t, b.Tick())
}

func TestBlink_DefaultInterval(t *testing.T) {
	assert.Equal(t, 600*time.Millisecond, DefaultBlinkInterval)
}

func TestBlink_CustomInterval(t *testing.T) {
	b := NewBlink(1, 200*time.Millisecond)
	b.Enable()
	// Just verify it doesn't panic and returns a cmd.
	assert.NotNil(t, b.Tick())
}

func TestBlink_NegativeIntervalDefaulted(t *testing.T) {
	b := NewBlink(1, -1)
	b.Enable()
	assert.NotNil(t, b.Tick())
}
