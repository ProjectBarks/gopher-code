package components

import (
	"strings"

	tea "charm.land/bubbletea/v2"
	"charm.land/lipgloss/v2"
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

// CtrlCHintMsg signals the status line to show the exit confirmation hint.
type CtrlCHintMsg struct{}

// TokenUpdateMsg updates the token count display.
type TokenUpdateMsg struct {
	InputTokens  int
	OutputTokens int
}

// StatusLine renders the bottom status bar with model, tokens, cost, and mode.
type StatusLine struct {
	session *session.SessionState
	mode       StatusMode
	width      int
	height     int
	focused    bool
	ctrlCHint  bool // Show "Press Ctrl-C again to exit"

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
	case CtrlCHintMsg:
		sl.ctrlCHint = true
		return sl, nil
	case ModeChangeMsg:
		sl.mode = msg.Mode
		sl.ctrlCHint = false // Reset hint on mode change
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
	cs := t.Colors()

	var content string

	switch sl.mode {
	case ModeStreaming, ModeToolRunning:
		// During streaming/tool: show interrupt hint (matching Claude Code)
		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextMuted))
		content = dimStyle.Render("esc to interrupt")

	default:
		dimStyle := lipgloss.NewStyle().
			Foreground(lipgloss.Color(cs.TextMuted))
		if sl.ctrlCHint {
			// After first Ctrl+C: show exit confirmation hint
			content = dimStyle.Render("Press Ctrl-C again to exit")
		} else {
			// Idle: show "? for shortcuts" on the left (matching Claude Code)
			content = dimStyle.Render("? for shortcuts")
		}
	}

	// Pad to fill width — use lipgloss.Width to count VISUAL chars (not bytes)
	// because content includes ANSI escape sequences from styling.
	if sl.width > 0 {
		visualWidth := lipgloss.Width(content)
		if visualWidth < sl.width {
			content = content + strings.Repeat(" ", sl.width-visualWidth)
		} else if visualWidth > sl.width {
			content = truncateField(content, sl.width)
		}
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
