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

// InputPane is a text input with command history and terminal-style keybindings.
type InputPane struct {
	runes      []rune // Input buffer as runes for Unicode safety
	width      int
	height     int
	focused    bool
	cursor     int // Cursor position in runes (not bytes)
	history    []string
	historyIdx int  // -1 means "not navigating history"
	multiline  bool // True when in multi-line editing mode
	savedInput string // Saved input when navigating history
}

// NewInputPane creates a new empty input pane.
func NewInputPane() *InputPane {
	return &InputPane{
		runes:      make([]rune, 0),
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

// View renders the input pane with prompt and cursor.
func (ip *InputPane) View() tea.View {
	t := theme.Current()
	prompt := t.PromptChar().Render(PromptPrefix)

	text := string(ip.runes)
	if ip.focused && ip.cursor <= len(ip.runes) {
		before := string(ip.runes[:ip.cursor])
		after := ""
		if ip.cursor < len(ip.runes) {
			after = string(ip.runes[ip.cursor:])
		}
		text = before + "█" + after
	}

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
func (ip *InputPane) Focus()        { ip.focused = true }
// Blur removes focus from this pane.
func (ip *InputPane) Blur()         { ip.focused = false }
// Focused returns whether this pane has focus.
func (ip *InputPane) Focused() bool { return ip.focused }

// Value returns the current input text.
func (ip *InputPane) Value() string { return string(ip.runes) }

// SetValue sets the input text.
func (ip *InputPane) SetValue(v string) {
	ip.runes = []rune(v)
	ip.cursor = len(ip.runes)
}

// HasText returns true if the input buffer contains any text.
func (ip *InputPane) HasText() bool {
	return len(ip.runes) > 0
}

// Clear clears the input text.
func (ip *InputPane) Clear() {
	ip.runes = ip.runes[:0]
	ip.cursor = 0
}

// AddToHistory adds a command to the history.
func (ip *InputPane) AddToHistory(cmd string) {
	ip.history = append(ip.history, cmd)
	ip.historyIdx = -1
}

// --- Key handling ---

func (ip *InputPane) handleKey(msg tea.KeyPressMsg) (*InputPane, tea.Cmd) {
	switch {
	// Submit on Enter (when not in multiline mode)
	case msg.Code == tea.KeyEnter && !ip.multiline:
		text := strings.TrimSpace(string(ip.runes))
		if text != "" {
			ip.Clear()
			ip.historyIdx = -1
			return ip, func() tea.Msg {
				return SubmitMsg{Text: text}
			}
		}
		return ip, nil

	// Backspace: delete character before cursor
	case msg.Code == tea.KeyBackspace:
		if ip.cursor > 0 {
			ip.runes = append(ip.runes[:ip.cursor-1], ip.runes[ip.cursor:]...)
			ip.cursor--
		}
		return ip, nil

	// Delete: remove character at cursor
	case msg.Code == tea.KeyDelete:
		if ip.cursor < len(ip.runes) {
			ip.runes = append(ip.runes[:ip.cursor], ip.runes[ip.cursor+1:]...)
		}
		return ip, nil

	// Cursor movement
	case msg.Code == tea.KeyLeft:
		if ip.cursor > 0 {
			ip.cursor--
		}
		return ip, nil
	case msg.Code == tea.KeyRight:
		if ip.cursor < len(ip.runes) {
			ip.cursor++
		}
		return ip, nil
	case msg.Code == tea.KeyHome:
		ip.cursor = 0
		return ip, nil
	case msg.Code == tea.KeyEnd:
		ip.cursor = len(ip.runes)
		return ip, nil

	// Ctrl shortcuts (standard terminal keybindings)
	case msg.Code == 'a' && msg.Mod == tea.ModCtrl: // Beginning of line
		ip.cursor = 0
		return ip, nil
	case msg.Code == 'e' && msg.Mod == tea.ModCtrl: // End of line
		ip.cursor = len(ip.runes)
		return ip, nil
	case msg.Code == 'k' && msg.Mod == tea.ModCtrl: // Kill to end of line
		ip.runes = ip.runes[:ip.cursor]
		return ip, nil
	case msg.Code == 'u' && msg.Mod == tea.ModCtrl: // Kill to beginning of line
		ip.runes = ip.runes[ip.cursor:]
		ip.cursor = 0
		return ip, nil
	case msg.Code == 'w' && msg.Mod == tea.ModCtrl: // Delete word backward
		ip.deleteWordBackward()
		return ip, nil

	// History navigation: only when input is empty or already navigating
	case msg.Code == tea.KeyUp:
		if len(ip.runes) == 0 || ip.historyIdx >= 0 {
			ip.navigateHistoryUp()
		}
		return ip, nil
	case msg.Code == tea.KeyDown:
		if ip.historyIdx >= 0 {
			ip.navigateHistoryDown()
		}
		return ip, nil

	default:
		// Insert printable characters
		if msg.Text != "" {
			runes := []rune(msg.Text)
			newRunes := make([]rune, 0, len(ip.runes)+len(runes))
			newRunes = append(newRunes, ip.runes[:ip.cursor]...)
			newRunes = append(newRunes, runes...)
			newRunes = append(newRunes, ip.runes[ip.cursor:]...)
			ip.runes = newRunes
			ip.cursor += len(runes)
		}
		return ip, nil
	}
}

func (ip *InputPane) deleteWordBackward() {
	if ip.cursor == 0 {
		return
	}
	// Skip trailing spaces
	pos := ip.cursor
	for pos > 0 && ip.runes[pos-1] == ' ' {
		pos--
	}
	// Skip word characters
	for pos > 0 && ip.runes[pos-1] != ' ' {
		pos--
	}
	ip.runes = append(ip.runes[:pos], ip.runes[ip.cursor:]...)
	ip.cursor = pos
}

func (ip *InputPane) navigateHistoryUp() {
	if len(ip.history) == 0 {
		return
	}
	if ip.historyIdx == -1 {
		// Save current input before entering history
		ip.savedInput = string(ip.runes)
		ip.historyIdx = len(ip.history) - 1
	} else if ip.historyIdx > 0 {
		ip.historyIdx--
	}
	ip.SetValue(ip.history[ip.historyIdx])
}

func (ip *InputPane) navigateHistoryDown() {
	if ip.historyIdx == -1 {
		return
	}
	if ip.historyIdx < len(ip.history)-1 {
		ip.historyIdx++
		ip.SetValue(ip.history[ip.historyIdx])
	} else {
		// Restore saved input
		ip.historyIdx = -1
		ip.SetValue(ip.savedInput)
		ip.savedInput = ""
	}
}
