package ui

import (
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/display"
)

// ---------------------------------------------------------------------------
// T411: Display hooks integration — typeahead buffering, elapsed time
// tracking, blink toggling, and minimum display time enforcement.
//
// Source: useTypeahead.ts, useElapsedTime.ts, useBlink.ts, useMinDisplayTime.ts
// ---------------------------------------------------------------------------

// DisplayHooks aggregates the display-layer hooks that the AppModel uses
// during streaming and tool execution.
type DisplayHooks struct {
	// Typeahead buffers keystrokes while the UI is blocked (streaming/tool-running).
	Typeahead *display.Typeahead

	// ElapsedTime tracks how long the current model turn has been running.
	ElapsedTime *display.ElapsedTime

	// Blink provides a periodic visible/hidden toggle for cursor indicators.
	Blink *display.Blink

	// StatusThrottle prevents fast-cycling status text from flickering.
	StatusThrottle *display.MinDisplayTime[string]
}

// NewDisplayHooks creates the default set of display hooks.
func NewDisplayHooks() *DisplayHooks {
	return &DisplayHooks{
		Typeahead:      display.NewTypeahead(),
		ElapsedTime:    display.NewElapsedTime(1, time.Second),
		Blink:          display.NewBlink(1, display.DefaultBlinkInterval),
		StatusThrottle: display.NewMinDisplayTimeWith[string](1, 300*time.Millisecond, ""),
	}
}

// BlockForQuery activates typeahead buffering, starts the elapsed timer,
// and enables the blink cursor. Call when a model query begins.
func (d *DisplayHooks) BlockForQuery() tea.Cmd {
	d.Typeahead.Block()
	d.ElapsedTime.Start()
	d.Blink.Enable()
	return tea.Batch(d.ElapsedTime.Tick(), d.Blink.Tick())
}

// UnblockAfterQuery stops the elapsed timer, disables blink, and unblocks
// typeahead — returning any buffered keystrokes as replay commands.
func (d *DisplayHooks) UnblockAfterQuery() []tea.Cmd {
	d.ElapsedTime.Stop()
	d.Blink.Disable()
	buffered := d.Typeahead.Unblock()
	var cmds []tea.Cmd
	for _, key := range buffered {
		k := key
		cmds = append(cmds, func() tea.Msg { return k })
	}
	return cmds
}

// HandleDisplayMsg processes display hook tick messages (ElapsedTimeMsg,
// BlinkMsg, MinDisplayTimeMsg). Returns (handled, cmd). If handled is false,
// the caller should process the message normally.
func (d *DisplayHooks) HandleDisplayMsg(msg tea.Msg) (handled bool, cmd tea.Cmd) {
	switch msg.(type) {
	case display.ElapsedTimeMsg:
		return true, d.ElapsedTime.Tick()
	case display.BlinkMsg:
		d.Blink.Toggle()
		return true, d.Blink.Tick()
	case display.MinDisplayTimeMsg:
		d.StatusThrottle.Flush()
		return true, nil
	}
	return false, nil
}

// PushKey passes a keystroke through the typeahead. Returns true if the key
// should be processed normally (typeahead is not blocking).
func (d *DisplayHooks) PushKey(key tea.KeyPressMsg) bool {
	return d.Typeahead.Push(key)
}
