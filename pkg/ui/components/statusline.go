package components

import (
	"fmt"
	"strings"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/session"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// StatusMode describes the current application mode for the status line.
type StatusMode int

const (
	ModeIdle        StatusMode = iota
	ModeStreaming
	ModeToolRunning
)

// ModeChangeMsg signals a mode change to the status line.
type ModeChangeMsg struct {
	Mode StatusMode
}

// TokenUpdateMsg updates the token count display.
type TokenUpdateMsg struct {
	InputTokens  int
	OutputTokens int
}

// StatusLine renders the bottom status bar with model, tokens, cost, and mode.
type StatusLine struct {
	session *session.SessionState
	mode    StatusMode
	width   int
	height  int
	focused bool

	inputTokens  int
	outputTokens int
}

// NewStatusLine creates a new status line component.
func NewStatusLine(sess *session.SessionState) *StatusLine {
	return &StatusLine{
		session: sess,
		mode:    ModeIdle,
	}
}

// Init initializes the status line.
func (sl *StatusLine) Init() tea.Cmd {
	return nil
}

// Update handles status line messages.
func (sl *StatusLine) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case ModeChangeMsg:
		sl.mode = msg.Mode
		return sl, nil
	case TokenUpdateMsg:
		sl.inputTokens = msg.InputTokens
		sl.outputTokens = msg.OutputTokens
		return sl, nil
	case tea.WindowSizeMsg:
		sl.SetSize(msg.Width, msg.Height)
		return sl, nil
	}
	return sl, nil
}

// View renders the status line.
func (sl *StatusLine) View() tea.View {
	t := theme.Current()

	var parts []string

	// Mode indicator
	switch sl.mode {
	case ModeIdle:
		parts = append(parts, "Idle")
	case ModeStreaming:
		parts = append(parts, "Streaming")
	case ModeToolRunning:
		parts = append(parts, "Tool Running")
	}

	// Model name
	if sl.session != nil {
		model := sl.session.Config.Model
		parts = append(parts, model)
	}

	// Token count
	if sl.inputTokens > 0 || sl.outputTokens > 0 {
		parts = append(parts, fmt.Sprintf("%d/%d tokens", sl.inputTokens, sl.outputTokens))
	}

	content := strings.Join(parts, " │ ")

	// Pad or truncate to fill width
	if sl.width > 0 && len(content) < sl.width {
		content = content + strings.Repeat(" ", sl.width-len(content))
	} else if sl.width > 0 {
		content = truncateField(content, sl.width)
	}

	style := t.StatusBar()
	return tea.NewView(style.Render(content))
}

// SetSize sets the dimensions of the status line.
func (sl *StatusLine) SetSize(width, height int) {
	sl.width = width
	sl.height = height
}

// Focus gives focus to this component.
func (sl *StatusLine) Focus() {
	sl.focused = true
}

// Blur removes focus from this component.
func (sl *StatusLine) Blur() {
	sl.focused = false
}

// Focused returns whether this component has focus.
func (sl *StatusLine) Focused() bool {
	return sl.focused
}
