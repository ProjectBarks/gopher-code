package ink

// Source: ink/ink.tsx, ink/focus.ts, ink/hit-test.ts, ink/terminal-focus-state.ts
//
// In TS, Ink has a React reconciler, DOM tree, focus manager, and hit testing.
// In Go/bubbletea, these map to: focus tracking on the parent model, lifecycle
// via Init/Update/View, and mouse handling via tea.MouseMsg.
//
// This file provides the Go equivalents for the remaining Ink core concepts.

import (
	"sync"

	tea "charm.land/bubbletea/v2"
)

// ---------------------------------------------------------------------------
// Focus Manager — Source: ink/focus.ts
// ---------------------------------------------------------------------------

// FocusManager tracks which component currently has focus.
// In Ink, this walks a DOM tree. In Go, components register by ID.
type FocusManager struct {
	mu          sync.RWMutex
	active      string   // ID of the focused component
	stack       []string // focus history for restore-on-unmount
	maxStack    int
	tabOrder    []string // ordered list of focusable component IDs
}

// NewFocusManager creates a focus manager.
func NewFocusManager() *FocusManager {
	return &FocusManager{maxStack: 32}
}

// Focus sets the active component. Previous focus is pushed to the stack.
func (fm *FocusManager) Focus(id string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if id == fm.active {
		return
	}
	if fm.active != "" {
		// Remove duplicates before pushing
		fm.stack = removeFromSlice(fm.stack, fm.active)
		fm.stack = append(fm.stack, fm.active)
		if len(fm.stack) > fm.maxStack {
			fm.stack = fm.stack[1:]
		}
	}
	fm.active = id
}

// Blur removes focus from the active component.
func (fm *FocusManager) Blur() {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.active = ""
}

// ActiveID returns the currently focused component's ID.
func (fm *FocusManager) ActiveID() string {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.active
}

// IsFocused returns true if the given component has focus.
func (fm *FocusManager) IsFocused(id string) bool {
	fm.mu.RLock()
	defer fm.mu.RUnlock()
	return fm.active == id
}

// RestorePrevious pops the focus stack and focuses the previous component.
// Used when a focused component unmounts (e.g., dialog closes).
func (fm *FocusManager) RestorePrevious() string {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if len(fm.stack) == 0 {
		fm.active = ""
		return ""
	}
	prev := fm.stack[len(fm.stack)-1]
	fm.stack = fm.stack[:len(fm.stack)-1]
	fm.active = prev
	return prev
}

// SetTabOrder defines the order for Tab/Shift+Tab cycling.
func (fm *FocusManager) SetTabOrder(ids []string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.tabOrder = make([]string, len(ids))
	copy(fm.tabOrder, ids)
}

// FocusNext cycles to the next component in tab order.
func (fm *FocusManager) FocusNext() string {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if len(fm.tabOrder) == 0 {
		return fm.active
	}
	idx := indexOf(fm.tabOrder, fm.active)
	next := (idx + 1) % len(fm.tabOrder)
	fm.active = fm.tabOrder[next]
	return fm.active
}

// FocusPrev cycles to the previous component in tab order.
func (fm *FocusManager) FocusPrev() string {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	if len(fm.tabOrder) == 0 {
		return fm.active
	}
	idx := indexOf(fm.tabOrder, fm.active)
	prev := (idx - 1 + len(fm.tabOrder)) % len(fm.tabOrder)
	fm.active = fm.tabOrder[prev]
	return fm.active
}

// HandleRemoved cleans up when a component is removed.
func (fm *FocusManager) HandleRemoved(id string) {
	fm.mu.Lock()
	defer fm.mu.Unlock()
	fm.stack = removeFromSlice(fm.stack, id)
	if fm.active == id {
		// Restore from stack
		if len(fm.stack) > 0 {
			fm.active = fm.stack[len(fm.stack)-1]
			fm.stack = fm.stack[:len(fm.stack)-1]
		} else {
			fm.active = ""
		}
	}
}

// ---------------------------------------------------------------------------
// App Lifecycle — Source: ink/ink.tsx lifecycle methods
// ---------------------------------------------------------------------------

// ExitMsg requests graceful shutdown.
type ExitMsg struct {
	Code int // exit code
}

// Exit returns a tea.Cmd that triggers graceful shutdown.
func Exit(code int) tea.Cmd {
	return func() tea.Msg { return ExitMsg{Code: code} }
}

// ForceRedrawMsg requests a full screen redraw.
type ForceRedrawMsg struct{}

// ForceRedraw returns a tea.Cmd that triggers a full redraw.
func ForceRedraw() tea.Cmd {
	return func() tea.Msg { return ForceRedrawMsg{} }
}

// ---------------------------------------------------------------------------
// Helpers
// ---------------------------------------------------------------------------

func removeFromSlice(s []string, val string) []string {
	result := make([]string, 0, len(s))
	for _, v := range s {
		if v != val {
			result = append(result, v)
		}
	}
	return result
}

func indexOf(s []string, val string) int {
	for i, v := range s {
		if v == val {
			return i
		}
	}
	return -1
}
