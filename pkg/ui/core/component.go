package core

import tea "charm.land/bubbletea/v2"

// Component is the base interface for all UI components.
// It extends tea.Model with size management for layout engines.
type Component interface {
	tea.Model
	SetSize(width, height int)
}

// Focusable is an interface for components that can receive and lose focus.
// It extends tea.Model so the FocusManager can route messages to focused elements.
type Focusable interface {
	tea.Model
	Focus()
	Blur()
	Focused() bool
}
