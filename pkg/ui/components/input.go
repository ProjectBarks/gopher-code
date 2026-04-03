package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
)

// CursorBlinkMsg is sent periodically to toggle the cursor.
type CursorBlinkMsg struct{}

// SubmitMsg is sent when the user presses Enter to submit input.
type SubmitMsg struct {
	Text string
}

// InputPane is a multi-line text input with command history.
type InputPane struct {
	value    string
	width    int
	height   int
	focused  bool
	cursor   int // Cursor position in value
	history  []string
	historyIdx int // -1 means "not navigating history"
	multiline bool
}

// NewInputPane creates a new empty input pane.
func NewInputPane() *InputPane {
	return &InputPane{
		history:    make([]string, 0),
		historyIdx: -1,
	}
}

// Init returns the initial command (cursor blink tick).
func (ip *InputPane) Init() tea.Cmd {
	return tea.Tick(530*time.Millisecond, func(time.Time) tea.Msg {
		return CursorBlinkMsg{}
	})
}

// Update handles key presses and other messages.
func (ip *InputPane) Update(msg tea.Msg) (tea.Model, tea.Cmd) {
	switch msg := msg.(type) {
	case tea.KeyPressMsg:
		return ip.handleKey(msg)
	case tea.WindowSizeMsg:
		ip.SetSize(msg.Width, msg.Height)
		return ip, nil
	}
	return ip, nil
}

// View renders the input pane.
func (ip *InputPane) View() tea.View {
	t := theme.Current()

	prompt := t.PromptChar().Render("> ")
	text := ip.value
	if ip.focused {
		// Show cursor
		if ip.cursor <= len(text) {
			before := text[:ip.cursor]
			after := ""
			if ip.cursor < len(text) {
				after = text[ip.cursor:]
			}
			text = before + "█" + after
		}
	}

	// Word wrap for display
	if ip.width > 2 {
		text = wrapText(text, ip.width-2)
	}

	return tea.NewView(prompt + text)
}

// SetSize sets the dimensions of the input pane.
func (ip *InputPane) SetSize(width, height int) {
	ip.width = width
	ip.height = height
}

// Focus gives focus to this pane.
func (ip *InputPane) Focus() {
	ip.focused = true
}

// Blur removes focus from this pane.
func (ip *InputPane) Blur() {
	ip.focused = false
}

// Focused returns whether this pane has focus.
func (ip *InputPane) Focused() bool {
	return ip.focused
}

// Value returns the current input text.
func (ip *InputPane) Value() string {
	return ip.value
}

// SetValue sets the input text.
func (ip *InputPane) SetValue(v string) {
	ip.value = v
	ip.cursor = len(v)
}

// Clear clears the input text.
func (ip *InputPane) Clear() {
	ip.value = ""
	ip.cursor = 0
}

// AddToHistory adds a command to the history.
func (ip *InputPane) AddToHistory(cmd string) {
	ip.history = append(ip.history, cmd)
	ip.historyIdx = -1
}

// --- Internal ---

func (ip *InputPane) handleKey(msg tea.KeyPressMsg) (*InputPane, tea.Cmd) {
	switch {
	case msg.Code == tea.KeyEnter && !ip.multiline:
		text := strings.TrimSpace(ip.value)
		if text != "" {
			ip.Clear()
			return ip, func() tea.Msg {
				return SubmitMsg{Text: text}
			}
		}
		return ip, nil

	case msg.Code == tea.KeyBackspace:
		if ip.cursor > 0 {
			ip.value = ip.value[:ip.cursor-1] + ip.value[ip.cursor:]
			ip.cursor--
		}
		return ip, nil

	case msg.Code == tea.KeyLeft:
		if ip.cursor > 0 {
			ip.cursor--
		}
		return ip, nil

	case msg.Code == tea.KeyRight:
		if ip.cursor < len(ip.value) {
			ip.cursor++
		}
		return ip, nil

	case msg.Code == tea.KeyUp:
		ip.navigateHistoryUp()
		return ip, nil

	case msg.Code == tea.KeyDown:
		ip.navigateHistoryDown()
		return ip, nil

	case msg.Code == tea.KeyHome:
		ip.cursor = 0
		return ip, nil

	case msg.Code == tea.KeyEnd:
		ip.cursor = len(ip.value)
		return ip, nil

	default:
		// Insert printable characters
		if msg.Text != "" {
			ip.value = ip.value[:ip.cursor] + msg.Text + ip.value[ip.cursor:]
			ip.cursor += len(msg.Text)
		}
		return ip, nil
	}
}

func (ip *InputPane) navigateHistoryUp() {
	if len(ip.history) == 0 {
		return
	}
	if ip.historyIdx == -1 {
		ip.historyIdx = len(ip.history) - 1
	} else if ip.historyIdx > 0 {
		ip.historyIdx--
	}
	ip.value = ip.history[ip.historyIdx]
	ip.cursor = len(ip.value)
}

func (ip *InputPane) navigateHistoryDown() {
	if ip.historyIdx == -1 {
		return
	}
	if ip.historyIdx < len(ip.history)-1 {
		ip.historyIdx++
		ip.value = ip.history[ip.historyIdx]
	} else {
		ip.historyIdx = -1
		ip.value = ""
	}
	ip.cursor = len(ip.value)
}
