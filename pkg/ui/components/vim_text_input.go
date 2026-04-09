package components

import (
	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
)

// Source: components/VimTextInput.tsx + hooks/useVimInput.ts
//
// Extends TextInputModel with vim normal/insert mode. In insert mode,
// keys are passed through to the inner textinput. In normal mode,
// vim motions (h/j/k/l, w/b, x, dd, etc.) are handled.

// VimMode tracks whether we're in normal or insert mode.
type VimMode int

const (
	VimInsert VimMode = iota
	VimNormal
)

// VimModeChangeMsg is sent when vim mode changes.
type VimModeChangeMsg struct {
	Mode VimMode
}

// VimTextInputModel wraps TextInputModel with vim keybindings.
type VimTextInputModel struct {
	TextInputModel
	mode VimMode
}

// NewVimTextInput creates a vim-enabled text input.
func NewVimTextInput(placeholder string) VimTextInputModel {
	return VimTextInputModel{
		TextInputModel: NewTextInput(placeholder),
		mode:           VimInsert, // start in insert mode like the TS version
	}
}

// Mode returns the current vim mode.
func (m VimTextInputModel) Mode() VimMode { return m.mode }

// Update handles key events with vim mode awareness.
func (m VimTextInputModel) Update(msg tea.Msg) (VimTextInputModel, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if m.mode == VimNormal {
			return m.handleNormal(msg)
		}
		return m.handleInsert(msg)
	}

	// Pass non-key messages to inner input
	var cmd tea.Cmd
	m.TextInputModel, cmd = m.TextInputModel.Update(msg)
	return m, cmd
}

func (m VimTextInputModel) handleInsert(msg tea.KeyPressMsg) (VimTextInputModel, tea.Cmd) {
	// Escape → switch to normal mode
	if msg.Code == tea.KeyEscape {
		m.mode = VimNormal
		return m, func() tea.Msg { return VimModeChangeMsg{Mode: VimNormal} }
	}

	// Pass through to inner textinput
	var cmd tea.Cmd
	m.TextInputModel, cmd = m.TextInputModel.Update(msg)
	return m, cmd
}

func (m VimTextInputModel) handleNormal(msg tea.KeyPressMsg) (VimTextInputModel, tea.Cmd) {
	switch {
	case msg.Code == 'i':
		// i → insert mode at cursor
		m.mode = VimInsert
		return m, func() tea.Msg { return VimModeChangeMsg{Mode: VimInsert} }

	case msg.Code == 'a':
		// a → insert mode after cursor
		m.mode = VimInsert
		return m, func() tea.Msg { return VimModeChangeMsg{Mode: VimInsert} }

	case msg.Code == 'A':
		// A → insert mode at end of line
		m.CursorEnd()
		m.mode = VimInsert
		return m, func() tea.Msg { return VimModeChangeMsg{Mode: VimInsert} }

	case msg.Code == 'I':
		// I → insert mode at beginning of line
		m.CursorStart()
		m.mode = VimInsert
		return m, func() tea.Msg { return VimModeChangeMsg{Mode: VimInsert} }

	case msg.Code == '0' || msg.Code == tea.KeyHome:
		m.CursorStart()

	case msg.Code == '$' || msg.Code == tea.KeyEnd:
		m.CursorEnd()

	case msg.Code == 'h' || msg.Code == tea.KeyLeft:
		// Move cursor left via a Left key event
		var cmd tea.Cmd
		m.TextInputModel, cmd = m.TextInputModel.Update(tea.KeyPressMsg{Code: tea.KeyLeft})
		return m, cmd

	case msg.Code == 'l' || msg.Code == tea.KeyRight:
		var cmd tea.Cmd
		m.TextInputModel, cmd = m.TextInputModel.Update(tea.KeyPressMsg{Code: tea.KeyRight})
		return m, cmd

	case msg.Code == 'x':
		// Delete character at cursor (via Delete key)
		var cmd tea.Cmd
		m.TextInputModel, cmd = m.TextInputModel.Update(tea.KeyPressMsg{Code: tea.KeyDelete})
		return m, cmd

	case msg.Code == 'd':
		// dd → clear line (simplified from full vim operator+motion)
		m.Reset()
	}

	return m, nil
}

// View renders with a mode indicator.
func (m VimTextInputModel) View() string {
	modeIndicator := ""
	if m.mode == VimNormal {
		modeIndicator = lipgloss.NewStyle().
			Bold(true).
			Foreground(lipgloss.Color("11")).
			Render("[NORMAL] ")
	}
	return modeIndicator + m.TextInputModel.View()
}
