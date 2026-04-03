package ui

import (
	"sync"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/query"
)

// EventBridge converts QueryEvent callbacks into Bubbletea messages
// and injects them into the program via program.Send().
// This allows non-Bubbletea goroutines (like the query executor)
// to safely communicate with the UI.
type EventBridge struct {
	program *tea.Program
	mu      sync.RWMutex
}

// NewEventBridge creates a new EventBridge.
func NewEventBridge() *EventBridge {
	return &EventBridge{}
}

// SetProgram sets the Bubbletea program for message injection.
// Must be called after the program is created.
func (eb *EventBridge) SetProgram(program *tea.Program) {
	eb.mu.Lock()
	defer eb.mu.Unlock()
	eb.program = program
}

// OnQueryEvent is the callback passed to query.Query() as onEvent.
// It converts QueryEvents into QueryEventMsg and sends them to the Bubbletea program.
func (eb *EventBridge) OnQueryEvent(evt query.QueryEvent) {
	eb.mu.RLock()
	program := eb.program
	eb.mu.RUnlock()

	if program == nil {
		// Program not yet set up (shouldn't happen in normal flow)
		return
	}

	msg := QueryEventMsg{Event: evt}
	program.Send(msg)
}

// BridgeCallback returns a callback function suitable for passing to query.Query().
func (eb *EventBridge) BridgeCallback() query.EventCallback {
	return eb.OnQueryEvent
}

// This allows clients to use the bridge like:
//
//   bridge := NewEventBridge()
//   // ... create and start program ...
//   bridge.SetProgram(program)
//   // ... pass bridge.BridgeCallback() to query.Query() ...
