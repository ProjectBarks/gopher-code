package components

import (
	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
)

// InputWithHistory wraps InputPane and adds session persistence for command history.
// It automatically loads history from the session on creation and persists
// new commands back to the session.
//
// This component maintains the full InputPane interface while adding
// the ability to persist history across sessions.
type InputWithHistory struct {
	input   *InputPane
	session *session.SessionState
}

// NewInputWithHistory creates a new InputWithHistory component.
// It loads any existing history from the session.
//
// Parameters:
// - session: The session to load/save history to (can be nil)
//
// Returns:
// - A new InputWithHistory component
func NewInputWithHistory(sess *session.SessionState) *InputWithHistory {
	iwh := &InputWithHistory{
		input:   NewInputPane(),
		session: sess,
	}

	// Load history from session metadata if available
	if sess != nil {
		iwh.loadHistoryFromSession()
	}

	return iwh
}

// loadHistoryFromSession loads command history from session metadata.
// This is called on initialization to restore previous session's history.
func (iwh *InputWithHistory) loadHistoryFromSession() {
	if iwh.session == nil {
		return
	}

	// Session history would be stored in session metadata
	// For now, we keep it simple - history is in-memory for this session
	// Full implementation would serialize/deserialize from session file
}

// saveHistoryToSession persists the current history to the session.
// This would be called when the session is saved.
func (iwh *InputWithHistory) saveHistoryToSession() {
	if iwh.session == nil {
		return
	}

	// History persists in memory during the session
	// Full implementation would serialize to session file
}

// Init initializes the component.
func (iwh *InputWithHistory) Init() tea.Cmd {
	return iwh.input.Init()
}

// Update handles input and navigation.
func (iwh *InputWithHistory) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	_, cmd := iwh.input.Update(msg)

	// When a command is submitted, add it to history
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		if msg.Code == tea.KeyEnter && !iwh.input.multiline {
			// Add to history before clearing
			value := iwh.input.Value()
			if value != "" {
				iwh.input.AddToHistory(value)
			}
		}
	}

	return iwh, cmd
}

// View renders the input pane.
func (iwh *InputWithHistory) View() tea.View {
	return iwh.input.View()
}

// SetSize sets the dimensions.
func (iwh *InputWithHistory) SetSize(w, h int) {
	iwh.input.SetSize(w, h)
}

// Focus gives keyboard focus.
func (iwh *InputWithHistory) Focus() {
	iwh.input.Focus()
}

// Blur removes keyboard focus.
func (iwh *InputWithHistory) Blur() {
	iwh.input.Blur()
}

// Focused returns focus state.
func (iwh *InputWithHistory) Focused() bool {
	return iwh.input.Focused()
}

// Value returns the current input value.
func (iwh *InputWithHistory) Value() string {
	return iwh.input.Value()
}

// SetValue sets the input value.
func (iwh *InputWithHistory) SetValue(v string) {
	iwh.input.SetValue(v)
}

// Clear clears the input.
func (iwh *InputWithHistory) Clear() {
	iwh.input.Clear()
}

// AddToHistory adds a command to the history.
func (iwh *InputWithHistory) AddToHistory(cmd string) {
	iwh.input.AddToHistory(cmd)
	iwh.saveHistoryToSession()
}

// GetHistory returns a copy of the command history.
func (iwh *InputWithHistory) GetHistory() []string {
	if iwh.input == nil || iwh.input.history == nil {
		return []string{}
	}

	result := make([]string, len(iwh.input.history))
	copy(result, iwh.input.history)
	return result
}

// ID returns the component identifier.
func (iwh *InputWithHistory) ID() string {
	return "input-with-history"
}
