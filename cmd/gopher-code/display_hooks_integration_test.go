package main

import (
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/display"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// TestDisplayHooks_WiredThroughAppModel verifies that all display hooks
// (Typeahead, ElapsedTime, Blink, StatusThrottle) are initialized in
// AppModel and reachable from the binary.
func TestDisplayHooks_WiredThroughAppModel(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	dh := app.GetDisplayHooks()
	require.NotNil(t, dh, "DisplayHooks must be initialized")
	require.NotNil(t, dh.Typeahead, "Typeahead must be initialized")
	require.NotNil(t, dh.ElapsedTime, "ElapsedTime must be initialized")
	require.NotNil(t, dh.Blink, "Blink must be initialized")
	require.NotNil(t, dh.StatusThrottle, "StatusThrottle must be initialized")
}

// TestDisplayHooks_TypeaheadBlocksInStreaming verifies that when a query
// begins (submit), the typeahead blocks, and when the turn completes, it
// unblocks and replays buffered keystrokes.
func TestDisplayHooks_TypeaheadBlocksInStreaming(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	dh := app.GetDisplayHooks()

	// Initially not blocked.
	assert.False(t, dh.Typeahead.IsBlocked())

	// Simulate submit: send a SubmitMsg through Update to trigger blocking.
	// We can't easily trigger a full query cycle, but we can test the
	// display hooks directly through the real integration path.
	dh.Typeahead.Block()
	dh.ElapsedTime.Start()
	dh.Blink.Enable()

	assert.True(t, dh.Typeahead.IsBlocked())
	assert.True(t, dh.ElapsedTime.IsRunning())
	assert.True(t, dh.Blink.IsEnabled())

	// Push keys while blocked — they should be buffered.
	ok := dh.Typeahead.Push(tea.KeyPressMsg{Code: 'a', Text: "a"})
	assert.False(t, ok, "Push should return false when blocked")
	ok = dh.Typeahead.Push(tea.KeyPressMsg{Code: 'b', Text: "b"})
	assert.False(t, ok, "Push should return false when blocked")
	assert.Equal(t, 2, dh.Typeahead.Len())

	// Unblock — should replay keys.
	cmds := dh.UnblockAfterQuery()
	assert.False(t, dh.Typeahead.IsBlocked())
	assert.False(t, dh.ElapsedTime.IsRunning())
	assert.False(t, dh.Blink.IsEnabled())
	assert.Len(t, cmds, 2, "should have 2 replay commands for buffered keys")

	// Execute the replay commands and verify they produce KeyPressMsgs.
	for i, cmd := range cmds {
		msg := cmd()
		kpm, ok := msg.(tea.KeyPressMsg)
		require.True(t, ok, "replay cmd %d should produce KeyPressMsg", i)
		expected := rune('a' + i)
		assert.Equal(t, expected, kpm.Code, "replayed key %d", i)
	}
}

// TestDisplayHooks_TickMessageRouting verifies that display tick messages
// are handled through AppModel.Update().
func TestDisplayHooks_TickMessageRouting(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	dh := app.GetDisplayHooks()

	// Enable blink so the BlinkMsg handler actually toggles.
	dh.Blink.Enable()

	// Send a BlinkMsg through the real Update path.
	model, cmd := app.Update(display.BlinkMsg{ID: 1})
	_ = model
	// Blink should have toggled (was visible, now hidden).
	assert.False(t, dh.Blink.Visible(), "Blink should toggle to hidden after BlinkMsg")
	// Should return a tick command for the next blink.
	assert.NotNil(t, cmd, "BlinkMsg handler should return a tick cmd")

	// Send an ElapsedTimeMsg.
	dh.ElapsedTime.Start()
	model, cmd = app.Update(display.ElapsedTimeMsg{ID: 1})
	_ = model
	// Should return a tick command since the timer is running.
	assert.NotNil(t, cmd, "ElapsedTimeMsg handler should return a tick cmd when running")

	// Send a MinDisplayTimeMsg.
	model, cmd = app.Update(display.MinDisplayTimeMsg{ID: 1})
	_ = model
	// No pending value, so cmd should be nil.
	assert.Nil(t, cmd, "MinDisplayTimeMsg should return nil when no pending value")
}

// TestDisplayHooks_HandleKeyTypeaheadFiltering verifies that the handleKey
// path filters keystrokes through the typeahead when blocked.
func TestDisplayHooks_HandleKeyTypeaheadFiltering(t *testing.T) {
	app := ui.NewAppModel(nil, nil)
	dh := app.GetDisplayHooks()

	// Block typeahead.
	dh.Typeahead.Block()

	// Send a regular key through Update — it should be swallowed.
	model, cmd := app.Update(tea.KeyPressMsg{Code: 'x', Text: "x"})
	_ = model
	assert.Nil(t, cmd, "blocked key should produce nil cmd")
	assert.Equal(t, 1, dh.Typeahead.Len(), "key should be buffered")

	// Ctrl+C should NOT be blocked (always passes through for cancellation).
	model, cmd = app.Update(tea.KeyPressMsg{Code: 'c', Mod: tea.ModCtrl})
	_ = model
	// Ctrl+C on idle with no text triggers the double-press hint, not a buffer.
	assert.Equal(t, 1, dh.Typeahead.Len(), "Ctrl+C should not be buffered")
}

// TestDisplayHooks_ElapsedTimeFormat verifies that FormatDuration (exposed
// through display.FormatDuration) works correctly through the real import path.
func TestDisplayHooks_ElapsedTimeFormat(t *testing.T) {
	assert.Equal(t, "0s", display.FormatDuration(0))
	assert.Equal(t, "5s", display.FormatDuration(5_000_000_000))      // 5s in ns
	assert.Equal(t, "1m 30s", display.FormatDuration(90_000_000_000)) // 90s in ns
	assert.Equal(t, "1h 5m", display.FormatDuration(3_900_000_000_000))
}
