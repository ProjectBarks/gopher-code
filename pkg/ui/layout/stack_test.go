package layout

import (
	"strings"
	"testing"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// MockComponent implements core.Component for testing.
type MockComponent struct {
	name    string
	width   int
	height  int
	focused bool
	updated bool
}

func (m *MockComponent) Init() tea.Cmd {
	return nil
}

func (m *MockComponent) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	m.updated = true
	return m, nil
}

func (m *MockComponent) View() tea.View {
	return tea.NewView(m.name + " view")
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

var _ core.Component = (*MockComponent)(nil)

func TestModalStackCreation(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	if stack == nil {
		t.Error("Expected non-nil modal stack")
	}
	if stack.main != main {
		t.Error("Expected main component to be set")
	}
	if stack.HasModal() {
		t.Error("Expected no modals initially")
	}
}

func TestModalStackPushPop(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	modal1 := &MockComponent{name: "modal1"}
	stack.PushModal(modal1)

	if !stack.HasModal() {
		t.Error("Expected modal after push")
	}
	if stack.TopModal() != modal1 {
		t.Error("Expected modal1 to be top")
	}

	modal2 := &MockComponent{name: "modal2"}
	stack.PushModal(modal2)

	if stack.TopModal() != modal2 {
		t.Error("Expected modal2 to be top after second push")
	}

	popped := stack.PopModal()
	if popped != modal2 {
		t.Error("Expected modal2 to be popped")
	}
	if stack.TopModal() != modal1 {
		t.Error("Expected modal1 to be top after pop")
	}

	stack.PopModal()
	if stack.HasModal() {
		t.Error("Expected no modals after popping all")
	}
}

func TestModalStackPopEmpty(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	popped := stack.PopModal()
	if popped != nil {
		t.Error("Expected nil when popping from empty stack")
	}
}

func TestModalStackMainUpdate(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	// With no modal, update should go to main
	msg := struct{}{}
	stack.Update(msg)

	if !main.updated {
		t.Error("Expected main component to be updated")
	}
}

func TestModalStackModalUpdate(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal)

	// With modal, update should go to modal, not main
	main.updated = false
	modal.updated = false
	msg := struct{}{}
	stack.Update(msg)

	if main.updated {
		t.Error("Expected main component NOT to be updated when modal exists")
	}
	if !modal.updated {
		t.Error("Expected modal component to be updated")
	}
}

func TestModalStackEscapeClosesModal(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal)
	if !stack.HasModal() {
		t.Error("Expected modal to be present")
	}

	// Simulate Escape key
	escMsg := tea.KeyPressMsg{Code: tea.KeyEsc}
	stack.Update(escMsg)

	if stack.HasModal() {
		t.Error("Expected modal to be closed by Escape")
	}
}

func TestModalStackWindowSizeMsg(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.SetSize(80, 24)
	if main.width != 80 || main.height != 24 {
		t.Error("Expected main component size to be updated")
	}

	modal := &MockComponent{name: "modal"}
	stack.PushModal(modal)

	windowMsg := tea.WindowSizeMsg{Width: 120, Height: 40}
	stack.Update(windowMsg)

	if stack.width != 120 || stack.height != 40 {
		t.Error("Expected stack size to be updated")
	}
	if main.width != 120 || main.height != 40 {
		t.Error("Expected main component size to be updated")
	}
	if modal.width != 120 || modal.height != 40 {
		t.Error("Expected modal component size to be updated")
	}
}

func TestModalStackView(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	view := stack.View()
	if !strings.Contains(view.Content, "main") {
		t.Errorf("Expected main view in content, got %q", view.Content)
	}
}

func TestModalStackViewWithModal(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal)
	view := stack.View()

	// Should contain both main and modal content
	if !strings.Contains(view.Content, "main") {
		t.Error("Expected main view in combined content")
	}
	if !strings.Contains(view.Content, "modal") {
		t.Error("Expected modal view in combined content")
	}
}

func TestModalStackMultipleModals(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal1 := &MockComponent{name: "modal1"}
	modal2 := &MockComponent{name: "modal2"}
	modal3 := &MockComponent{name: "modal3"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal1)
	stack.PushModal(modal2)
	stack.PushModal(modal3)

	if len(stack.modals) != 3 {
		t.Errorf("Expected 3 modals, got %d", len(stack.modals))
	}

	// Only top modal should be updated
	modal1.updated = false
	modal2.updated = false
	modal3.updated = false
	stack.Update(struct{}{})

	if modal3.updated && (modal1.updated || modal2.updated) {
		t.Error("Expected only top modal to be updated")
	}
}

func TestModalStackInit(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	cmd := stack.Init()
	// Command might be nil or non-nil, both are valid
	_ = cmd
}

func TestModalStackSetSize(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal)

	stack.SetSize(100, 50)

	if stack.width != 100 || stack.height != 50 {
		t.Error("Expected stack size to be updated")
	}
	if main.width != 100 || main.height != 50 {
		t.Error("Expected main component size")
	}
	if modal.width != 100 || modal.height != 50 {
		t.Error("Expected modal component size")
	}
}

func TestModalStackMainAccessor(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	if stack.Main() != main {
		t.Error("Expected Main() to return main component")
	}
}

func TestModalStackFocusOverride(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	// Without modal, main could be focused
	// With modal, modal should receive focus (in a real implementation)
	stack.PushModal(modal)

	// Modal should be on top
	if stack.TopModal() != modal {
		t.Error("Expected modal to override main focus")
	}
}

func TestModalStackEscapeWithoutModal(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	// Escape without modal should just pass through
	escMsg := tea.KeyPressMsg{Code: tea.KeyEsc}
	stack.Update(escMsg)

	if stack.HasModal() {
		t.Error("Should still have no modal after escape")
	}
}

func TestModalStackTopModalNil(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	if stack.TopModal() != nil {
		t.Error("Expected TopModal to return nil when no modals")
	}
}

func TestModalStackRenderingWithDifferentThemes(t *testing.T) {
	themes := []theme.ThemeName{
		theme.ThemeDark,
		theme.ThemeLight,
		theme.ThemeHighContrast,
	}

	for _, themeName := range themes {
		theme.SetTheme(themeName)
		defer theme.SetTheme(theme.ThemeDark)

		main := &MockComponent{name: "main"}
		modal := &MockComponent{name: "modal"}
		th := theme.Current()
		stack := NewModalStack(main, th)

		stack.PushModal(modal)
		view := stack.View()

		if !strings.Contains(view.Content, "main") {
			t.Errorf("Expected main in view with theme %s", themeName)
		}
	}
}

func TestModalStackConsistency(t *testing.T) {
	main := &MockComponent{name: "main"}
	modal := &MockComponent{name: "modal"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	stack.PushModal(modal)

	// Multiple calls should be consistent
	view1 := stack.View()
	view2 := stack.View()

	if !strings.Contains(view1.Content, "modal") {
		t.Error("Expected modal in view1")
	}
	if !strings.Contains(view2.Content, "modal") {
		t.Error("Expected modal in view2")
	}
}

func TestModalStackReturnTypes(t *testing.T) {
	main := &MockComponent{name: "main"}
	th := theme.Current()
	stack := NewModalStack(main, th)

	updated, cmd := stack.Update(struct{}{})

	if updated == nil {
		t.Error("Expected non-nil updated model")
	}
	// cmd can be nil or non-nil
	_ = cmd
}
