// Package prompt_input provides the main prompt input component.
//
// Source: components/PromptInput/PromptInput.tsx (20 files → 1 Go file)
//
// The prompt input is the primary user interaction point. It handles:
// text input, slash command autocomplete, history navigation, vim mode,
// multi-line input, submit/cancel, and mode indicators.
package prompt_input

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"

	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// SubmitMsg is sent when the user submits input (Enter).
type SubmitMsg struct {
	Text string
}

// CancelMsg is sent when the user cancels (Ctrl+C with empty input).
type CancelMsg struct{}

// InputMode describes the current input mode.
type InputMode string

const (
	ModeNormal  InputMode = "normal"
	ModeInsert  InputMode = "insert"  // vim insert mode
	ModeCommand InputMode = "command" // typing a slash command
	ModeSearch  InputMode = "search"  // history search
)

// Model is the prompt input bubbletea model.
type Model struct {
	text          string
	cursorPos     int
	mode          InputMode
	placeholder   string
	width         int
	focused       bool
	historyIndex  int
	history       []string
	suggestions   []string
	suggestionIdx int
	showSuggestions bool
	multiline     bool
	permissionMode string // "default", "plan", "auto", etc.
}

// New creates a new prompt input.
func New(placeholder string) Model {
	return Model{
		mode:        ModeInsert,
		placeholder: placeholder,
		focused:     true,
		width:       80,
		historyIndex: -1,
	}
}

// SetWidth updates the input width.
func (m *Model) SetWidth(w int) { m.width = w }

// SetFocused sets focus state.
func (m *Model) SetFocused(f bool) { m.focused = f }

// SetPermissionMode sets the displayed permission mode.
func (m *Model) SetPermissionMode(mode string) { m.permissionMode = mode }

// SetHistory sets the command history for arrow-key navigation.
func (m *Model) SetHistory(history []string) {
	m.history = history
	m.historyIndex = -1
}

// Value returns the current input text.
func (m Model) Value() string { return m.text }

// IsEmpty returns true if the input is empty.
func (m Model) IsEmpty() bool { return strings.TrimSpace(m.text) == "" }

// Reset clears the input.
func (m *Model) Reset() {
	m.text = ""
	m.cursorPos = 0
	m.historyIndex = -1
	m.showSuggestions = false
}

func (m Model) Init() tea.Cmd { return nil }

func (m Model) Update(msg tea.Msg) (Model, tea.Cmd) {
	if !m.focused {
		return m, nil
	}

	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return m.handleKey(msg)
	case tea.WindowSizeMsg:
		m.width = msg.Width
	}
	return m, nil
}

func (m Model) handleKey(msg tea.KeyPressMsg) (Model, tea.Cmd) {
	// Handle suggestions navigation
	if m.showSuggestions {
		switch msg.Code {
		case tea.KeyTab:
			m.applySuggestion()
			return m, nil
		case tea.KeyEscape:
			m.showSuggestions = false
			return m, nil
		}
	}

	switch msg.Code {
	case tea.KeyEnter:
		if msg.Mod&tea.ModShift != 0 || m.multiline {
			// Shift+Enter or multiline: insert newline
			m.insertChar('\n')
			return m, nil
		}
		text := m.text
		m.addToHistory(text)
		m.Reset()
		return m, func() tea.Msg { return SubmitMsg{Text: text} }

	case tea.KeyEscape:
		if m.showSuggestions {
			m.showSuggestions = false
		}
		return m, nil

	case tea.KeyBackspace:
		if m.cursorPos > 0 {
			runes := []rune(m.text)
			m.text = string(runes[:m.cursorPos-1]) + string(runes[m.cursorPos:])
			m.cursorPos--
			m.updateSuggestions()
		}
		return m, nil

	case tea.KeyLeft:
		if m.cursorPos > 0 {
			m.cursorPos--
		}
		return m, nil

	case tea.KeyRight:
		if m.cursorPos < len([]rune(m.text)) {
			m.cursorPos++
		}
		return m, nil

	case tea.KeyUp:
		m.historyPrev()
		return m, nil

	case tea.KeyDown:
		m.historyNext()
		return m, nil

	case tea.KeyHome:
		m.cursorPos = 0
		return m, nil

	case tea.KeyEnd:
		m.cursorPos = len([]rune(m.text))
		return m, nil

	default:
		// Ctrl+C with empty input → cancel
		if msg.Code == 'c' && msg.Mod&tea.ModCtrl != 0 {
			if m.IsEmpty() {
				return m, func() tea.Msg { return CancelMsg{} }
			}
			m.Reset()
			return m, nil
		}

		// Ctrl+A → beginning of line
		if msg.Code == 'a' && msg.Mod&tea.ModCtrl != 0 {
			m.cursorPos = 0
			return m, nil
		}

		// Ctrl+E → end of line
		if msg.Code == 'e' && msg.Mod&tea.ModCtrl != 0 {
			m.cursorPos = len([]rune(m.text))
			return m, nil
		}

		// Ctrl+K → kill to end of line
		if msg.Code == 'k' && msg.Mod&tea.ModCtrl != 0 {
			runes := []rune(m.text)
			m.text = string(runes[:m.cursorPos])
			return m, nil
		}

		// Ctrl+U → kill to start of line
		if msg.Code == 'u' && msg.Mod&tea.ModCtrl != 0 {
			runes := []rune(m.text)
			m.text = string(runes[m.cursorPos:])
			m.cursorPos = 0
			return m, nil
		}

		// Regular character input
		if msg.Code >= 32 && msg.Code < 127 && msg.Mod == 0 {
			m.insertChar(rune(msg.Code))
			m.updateSuggestions()
			return m, nil
		}
	}

	return m, nil
}

func (m *Model) insertChar(ch rune) {
	runes := []rune(m.text)
	newRunes := make([]rune, 0, len(runes)+1)
	newRunes = append(newRunes, runes[:m.cursorPos]...)
	newRunes = append(newRunes, ch)
	newRunes = append(newRunes, runes[m.cursorPos:]...)
	m.text = string(newRunes)
	m.cursorPos++
}

func (m *Model) historyPrev() {
	if len(m.history) == 0 {
		return
	}
	if m.historyIndex < len(m.history)-1 {
		m.historyIndex++
		idx := len(m.history) - 1 - m.historyIndex
		m.text = m.history[idx]
		m.cursorPos = len([]rune(m.text))
	}
}

func (m *Model) historyNext() {
	if m.historyIndex <= 0 {
		m.historyIndex = -1
		m.text = ""
		m.cursorPos = 0
		return
	}
	m.historyIndex--
	idx := len(m.history) - 1 - m.historyIndex
	m.text = m.history[idx]
	m.cursorPos = len([]rune(m.text))
}

func (m *Model) addToHistory(text string) {
	if strings.TrimSpace(text) == "" {
		return
	}
	// Don't add duplicates of the last entry
	if len(m.history) > 0 && m.history[len(m.history)-1] == text {
		return
	}
	m.history = append(m.history, text)
}

func (m *Model) updateSuggestions() {
	if !strings.HasPrefix(m.text, "/") {
		m.showSuggestions = false
		return
	}
	// Will be connected to slash command system
	m.showSuggestions = len(m.suggestions) > 0
}

func (m *Model) applySuggestion() {
	if m.suggestionIdx < 0 || m.suggestionIdx >= len(m.suggestions) {
		return
	}
	m.text = m.suggestions[m.suggestionIdx]
	m.cursorPos = len([]rune(m.text))
	m.showSuggestions = false
}

// SetSuggestions provides slash command suggestions.
func (m *Model) SetSuggestions(suggestions []string) {
	m.suggestions = suggestions
	m.showSuggestions = len(suggestions) > 0
	m.suggestionIdx = 0
}

func (m Model) View() string {
	colors := theme.Current().Colors()
	promptStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Accent)).Bold(true)
	inputStyle := lipgloss.NewStyle()
	placeholderStyle := lipgloss.NewStyle().Faint(true)
	modeStyle := lipgloss.NewStyle().Foreground(lipgloss.Color(colors.Warning))

	var b strings.Builder

	// Mode indicator
	if m.permissionMode != "" && m.permissionMode != "default" {
		b.WriteString(modeStyle.Render("["+m.permissionMode+"] "))
	}

	// Prompt symbol
	b.WriteString(promptStyle.Render("❯ "))

	// Input text or placeholder
	if m.text == "" {
		b.WriteString(placeholderStyle.Render(m.placeholder))
	} else {
		// Render text with cursor position
		runes := []rune(m.text)
		if m.cursorPos < len(runes) {
			b.WriteString(inputStyle.Render(string(runes[:m.cursorPos])))
			cursorStyle := lipgloss.NewStyle().Reverse(true)
			b.WriteString(cursorStyle.Render(string(runes[m.cursorPos])))
			b.WriteString(inputStyle.Render(string(runes[m.cursorPos+1:])))
		} else {
			b.WriteString(inputStyle.Render(m.text))
			cursorStyle := lipgloss.NewStyle().Reverse(true)
			b.WriteString(cursorStyle.Render(" "))
		}
	}

	return b.String()
}
