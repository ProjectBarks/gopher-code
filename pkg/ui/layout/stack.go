package layout

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/core"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// ModalStack manages a main component with an optional modal overlay.
// Modals are rendered on top of the main content with a semi-transparent backdrop.
// The top modal receives all updates; the main component is updated only if no modal is active.
type ModalStack struct {
	main       core.Component
	modals     []core.Component // Stack of modals (bottom to top)
	theme      theme.Theme
	width      int
	height     int
	backdropCh rune // Character for backdrop
}

// NewModalStack creates a new modal stack with the given main component.
func NewModalStack(main core.Component, t theme.Theme) *ModalStack {
	return &ModalStack{
		main:       main,
		modals:     make([]core.Component, 0),
		theme:      t,
		width:      80,
		height:     24,
		backdropCh: ' ',
	}
}

// PushModal adds a modal to the top of the stack.
func (ms *ModalStack) PushModal(modal core.Component) {
	modal.SetSize(ms.width, ms.height)
	ms.modals = append(ms.modals, modal)
}

// PopModal removes the top modal from the stack.
func (ms *ModalStack) PopModal() core.Component {
	if len(ms.modals) == 0 {
		return nil
	}
	modal := ms.modals[len(ms.modals)-1]
	ms.modals = ms.modals[:len(ms.modals)-1]
	return modal
}

// HasModal returns true if there are any modals in the stack.
func (ms *ModalStack) HasModal() bool {
	return len(ms.modals) > 0
}

// TopModal returns the topmost modal, or nil if none.
func (ms *ModalStack) TopModal() core.Component {
	if len(ms.modals) == 0 {
		return nil
	}
	return ms.modals[len(ms.modals)-1]
}

// Update routes messages appropriately:
// - If a modal is active, it receives all updates (except Escape closes it)
// - If no modal is active, the main component receives updates
func (ms *ModalStack) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	var cmds []tea.Cmd

	// Handle Escape to close top modal
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.String() == "esc" && ms.HasModal() {
			ms.PopModal()
			return ms, nil
		}

	case tea.WindowSizeMsg:
		ms.width = msg.Width
		ms.height = msg.Height
		ms.main.SetSize(ms.width, ms.height)
		for _, modal := range ms.modals {
			modal.SetSize(ms.width, ms.height)
		}
	}

	// Route message to appropriate component
	if ms.HasModal() {
		// Modal is active: route to top modal
		topModal := ms.TopModal()
		updated, cmd := topModal.Update(msg)
		ms.modals[len(ms.modals)-1] = updated.(core.Component)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	} else {
		// No modal: route to main component
		updated, cmd := ms.main.Update(msg)
		ms.main = updated.(core.Component)
		if cmd != nil {
			cmds = append(cmds, cmd)
		}
	}

	if len(cmds) == 0 {
		return ms, nil
	}
	return ms, tea.Batch(cmds...)
}

// View renders the main content with optional modal and backdrop.
func (ms *ModalStack) View() tea.View {
	mainView := ms.main.View()
	mainContent := mainView.Content

	// If no modal, just return main content
	if !ms.HasModal() {
		return tea.NewView(mainContent)
	}

	// Render modal with backdrop
	modalView := ms.TopModal().View()
	modalContent := modalView.Content

	// Create semi-transparent backdrop
	backdrop := ms.renderBackdrop(mainContent)

	// Combine backdrop and modal
	combined := ms.overlayModal(backdrop, modalContent)

	return tea.NewView(combined)
}

// renderBackdrop creates a semi-transparent backdrop over the main content.
func (ms *ModalStack) renderBackdrop(mainContent string) string {
	cs := ms.theme.Colors()
	backdropStyle := lipgloss.NewStyle().
		Foreground(lipgloss.Color(cs.TextMuted)).
		Background(lipgloss.Color(cs.Surface))

	lines := strings.Split(mainContent, "\n")
	var backdropLines []string

	for _, line := range lines {
		// Replace each character with a dim version
		dimmedLine := ""
		for _, ch := range line {
			if ch == ' ' {
				dimmedLine += " "
			} else {
				dimmedLine += fmt.Sprintf("%c", ch)
			}
		}
		backdropLines = append(backdropLines, backdropStyle.Render(dimmedLine))
	}

	return strings.Join(backdropLines, "\n")
}

// overlayModal places the modal content on top of the backdrop.
func (ms *ModalStack) overlayModal(backdrop, modal string) string {
	// Simple implementation: render modal in the center
	// In a real implementation, this would handle modal positioning better
	cs := ms.theme.Colors()
	modalBorder := lipgloss.NewStyle().
		Border(lipgloss.RoundedBorder()).
		BorderForeground(lipgloss.Color(cs.BorderFocused)).
		Background(lipgloss.Color(cs.SurfaceOverlay)).
		Padding(1)

	// Apply border styling to modal
	styledModal := modalBorder.Render(modal)

	// For now, just render the modal below the backdrop
	// (Better positioning would center it)
	return backdrop + "\n" + styledModal
}

// Init initializes the stack.
func (ms *ModalStack) Init() tea.Cmd {
	var cmds []tea.Cmd
	if cmd := ms.main.Init(); cmd != nil {
		cmds = append(cmds, cmd)
	}
	for _, modal := range ms.modals {
		if cmd := modal.Init(); cmd != nil {
			cmds = append(cmds, cmd)
		}
	}
	if len(cmds) == 0 {
		return nil
	}
	return tea.Batch(cmds...)
}

// SetSize updates the dimensions for the stack and all components.
func (ms *ModalStack) SetSize(width, height int) {
	ms.width = width
	ms.height = height
	ms.main.SetSize(width, height)
	for _, modal := range ms.modals {
		modal.SetSize(width, height)
	}
}

// Main returns the main component.
func (ms *ModalStack) Main() core.Component {
	return ms.main
}

// Ensure ModalStack implements tea.Model.
var _ tea.Model = (*ModalStack)(nil)
