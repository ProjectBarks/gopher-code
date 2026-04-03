package core

import (
	"testing"

	tea "charm.land/bubbletea/v2"
)

// MockComponent is a test implementation of Component.
type MockComponent struct {
	width   int
	height  int
	focused bool
	init    bool
	updates int
}

func (m *MockComponent) Init() tea.Cmd {
	m.init = true
	return nil
}

func (m *MockComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updates++
	return m, nil
}

func (m *MockComponent) View() tea.View {
	return tea.NewView("mock")
}

func (m *MockComponent) SetSize(width, height int) {
	m.width = width
	m.height = height
}

func (m *MockComponent) Focus() {
	m.focused = true
}

func (m *MockComponent) Blur() {
	m.focused = false
}

func (m *MockComponent) Focused() bool {
	return m.focused
}

// TestComponentInterface verifies the Component interface can be implemented.
func TestComponentInterface(t *testing.T) {
	m := &MockComponent{}

	// Component interface methods
	m.Init()
	if !m.init {
		t.Error("Init() not called")
	}

	m.SetSize(80, 24)
	if m.width != 80 || m.height != 24 {
		t.Errorf("SetSize failed: got %dx%d, want 80x24", m.width, m.height)
	}

	view := m.View()
	if view.Content != "mock" {
		t.Errorf("View failed: got %s, want mock", view.Content)
	}

	// Focusable interface methods
	m.Focus()
	if !m.Focused() {
		t.Error("Focus failed")
	}

	m.Blur()
	if m.Focused() {
		t.Error("Blur failed")
	}
}

// TestComponentLifecycle verifies component lifecycle.
func TestComponentLifecycle(t *testing.T) {
	m := &MockComponent{}

	// Initial state
	if m.init {
		t.Error("Init should not be called initially")
	}

	// After Init
	m.Init()
	if !m.init {
		t.Error("Init() should set init flag")
	}

	// Multiple updates
	for i := 0; i < 5; i++ {
		m.Update(nil)
	}
	if m.updates != 5 {
		t.Errorf("Update count: got %d, want 5", m.updates)
	}
}
