package components

import (
	"strings"
	"time"

	tea "charm.land/bubbletea/v2"
	"github.com/projectbarks/gopher-code/pkg/ui/hooks/input"
	"github.com/projectbarks/gopher-code/pkg/ui/theme"
	"github.com/projectbarks/gopher-code/pkg/vim"
)

// CursorBlinkMsg is sent periodically to toggle the cursor.
type CursorBlinkMsg struct{}

// SubmitMsg is sent when the user presses Enter to submit input.
type SubmitMsg struct {
	Text string
}

// InputPane is a text input with command history and terminal-style keybindings.
// When VimEnabled is true, Escape enters normal mode where vim motions,
// operators, and text objects are available (pkg/vim).
type InputPane struct {
	runes     []rune // Input buffer as runes for Unicode safety
	width     int
	height    int
	focused   bool
	cursor    int // Cursor position in runes (not bytes)
	History   *input.InputHistory
	multiline bool // True when in multi-line editing mode

	// Vim mode state — see pkg/vim for types.
	VimEnabled bool
	vimMode    vim.Mode           // INSERT or NORMAL
	vimCmd     vim.CommandState   // normal-mode command accumulator
	vimPersist vim.PersistentState // register + last-find + last-change
}

// NewInputPane creates a new empty input pane.
func NewInputPane() *InputPane {
	return &InputPane{
		runes:   make([]rune, 0),
		History: input.NewInputHistory(),
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
	ip.History.Add(cmd)
}

// --- Key handling ---

func (ip *InputPane) handleKey(msg tea.KeyPressMsg) (*InputPane, tea.Cmd) {
	// Vim mode intercepts keys before the normal handler.
	if ip.handleVimKey(msg) {
		return ip, nil
	}

	switch {
	// Submit on Enter (when not in multiline mode)
	case msg.Code == tea.KeyEnter && !ip.multiline:
		text := strings.TrimSpace(string(ip.runes))
		if text != "" {
			ip.Clear()
			ip.History.Reset()
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

	// History navigation with draft preservation and mode filtering.
	case msg.Code == tea.KeyUp:
		if text, changed := ip.History.NavigateUp(string(ip.runes)); changed {
			ip.SetValue(text)
		}
		return ip, nil
	case msg.Code == tea.KeyDown:
		if text, changed := ip.History.NavigateDown(); changed {
			ip.SetValue(text)
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

// ToggleVim toggles vim mode on/off. When turning on, starts in INSERT mode.
func (ip *InputPane) ToggleVim() {
	ip.VimEnabled = !ip.VimEnabled
	if ip.VimEnabled {
		ip.vimMode = vim.ModeInsert
		ip.vimCmd = vim.IdleCommand()
		ip.vimPersist = vim.NewPersistentState()
	}
}

// VimMode returns the current vim mode (INSERT or NORMAL), or empty if vim is off.
func (ip *InputPane) VimMode() vim.Mode {
	if !ip.VimEnabled {
		return ""
	}
	return ip.vimMode
}

// handleVimKey processes a key press in vim mode. Returns true if the key was
// consumed (caller should not run the normal key handler).
func (ip *InputPane) handleVimKey(msg tea.KeyPressMsg) bool {
	if !ip.VimEnabled {
		return false
	}

	// In INSERT mode, only Escape switches to NORMAL.
	if ip.vimMode == vim.ModeInsert {
		if msg.Code == tea.KeyEscape {
			ip.vimMode = vim.ModeNormal
			ip.vimCmd = vim.IdleCommand()
			// vim: cursor backs up one on entering normal
			if ip.cursor > 0 {
				ip.cursor--
			}
			return true
		}
		return false // let normal insert handler run
	}

	// NORMAL mode
	key := msg.Text
	if key == "" {
		// Handle special keys
		switch msg.Code {
		case tea.KeyEscape:
			ip.vimCmd = vim.IdleCommand()
			return true
		}
		return false
	}

	r := []rune(key)[0]

	// Dispatch based on current command state
	switch ip.vimCmd.Type {
	case vim.CmdIdle:
		return ip.handleVimIdle(r, key)
	case vim.CmdCount:
		return ip.handleVimCount(r, key)
	case vim.CmdOperator:
		return ip.handleVimOperator(r, key)
	case vim.CmdFind:
		return ip.handleVimFind(r)
	case vim.CmdOperatorFind:
		return ip.handleVimOperatorFind(r)
	case vim.CmdOperatorTextObj:
		return ip.handleVimOperatorTextObj(r)
	case vim.CmdReplace:
		return ip.handleVimReplace(r)
	case vim.CmdG:
		return ip.handleVimG(r)
	case vim.CmdOperatorG:
		return ip.handleVimOperatorG(r)
	case vim.CmdIndent:
		return ip.handleVimIndent(r)
	}
	return false
}

func (ip *InputPane) handleVimIdle(r rune, key string) bool {
	ctx := ip.vimCtx()
	count := 1

	switch {
	case r == 'i':
		ip.vimMode = vim.ModeInsert
	case r == 'I':
		ip.cursor = vim.ResolveMotion("0", ctx.Text, ctx.Cursor, 1)
		ip.vimMode = vim.ModeInsert
	case r == 'a':
		if ip.cursor < len(ip.runes) {
			ip.cursor++
		}
		ip.vimMode = vim.ModeInsert
	case r == 'A':
		ip.cursor = len(ip.runes)
		ip.vimMode = vim.ModeInsert
	case r == 'o':
		vim.ExecuteOpenLine("below", ctx)
		ip.syncFromCtx(ctx)
		ip.vimMode = vim.ModeInsert
	case r == 'O':
		vim.ExecuteOpenLine("above", ctx)
		ip.syncFromCtx(ctx)
		ip.vimMode = vim.ModeInsert
	case r == 'x':
		vim.ExecuteX(count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'r':
		ip.vimCmd = vim.CommandState{Type: vim.CmdReplace, Count: 1}
	case r == '~':
		vim.ExecuteToggleCase(count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'J':
		vim.ExecuteJoin(count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'p':
		vim.ExecutePaste(true, count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'P':
		vim.ExecutePaste(false, count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'u': // undo — not implemented, no-op
	case r == 'g':
		ip.vimCmd = vim.CommandState{Type: vim.CmdG, Count: 1}
	case r >= '1' && r <= '9':
		ip.vimCmd = vim.CommandState{Type: vim.CmdCount, Digits: string(r)}
	case vim.SimpleMotions[r]:
		ip.cursor = vim.ResolveMotion(key, ctx.Text, ctx.Cursor, 1)
	case r == 'G':
		ip.cursor = vim.ResolveMotion("G", ctx.Text, ctx.Cursor, 1)
	default:
		if op, ok := vim.Operators[r]; ok {
			ip.vimCmd = vim.CommandState{Type: vim.CmdOperator, Op: op, Count: 1}
		} else if ft, ok := vim.FindKeys[r]; ok {
			ip.vimCmd = vim.CommandState{Type: vim.CmdFind, Find: ft, Count: 1}
		} else if r == '>' || r == '<' {
			ip.vimCmd = vim.CommandState{Type: vim.CmdIndent, Dir: r, Count: 1}
		} else {
			return false
		}
	}
	return true
}

func (ip *InputPane) handleVimCount(r rune, key string) bool {
	if r >= '0' && r <= '9' {
		ip.vimCmd.Digits += string(r)
		return true
	}
	count := parseCount(ip.vimCmd.Digits)

	ctx := ip.vimCtx()
	switch {
	case vim.SimpleMotions[r]:
		ip.cursor = vim.ResolveMotion(key, ctx.Text, ctx.Cursor, count)
	case r == 'G':
		ip.cursor = vim.GoToLine(ctx.Text, count)
	case r == 'x':
		vim.ExecuteX(count, ctx)
		ip.syncFromCtx(ctx)
	default:
		if op, ok := vim.Operators[r]; ok {
			ip.vimCmd = vim.CommandState{Type: vim.CmdOperator, Op: op, Count: count}
			return true
		}
		if ft, ok := vim.FindKeys[r]; ok {
			ip.vimCmd = vim.CommandState{Type: vim.CmdFind, Find: ft, Count: count}
			return true
		}
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimOperator(r rune, key string) bool {
	ctx := ip.vimCtx()
	op := ip.vimCmd.Op
	count := ip.vimCmd.Count

	switch {
	case vim.SimpleMotions[r]:
		vim.ExecuteOperatorMotion(op, key, count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'G':
		vim.ExecuteOperatorG(op, count, ctx)
		ip.syncFromCtx(ctx)
	case r == 'g':
		ip.vimCmd = vim.CommandState{Type: vim.CmdOperatorG, Op: op, Count: count}
		return true
	case r == 'i' || r == 'a':
		scope := vim.TextObjScopes[r]
		ip.vimCmd = vim.CommandState{Type: vim.CmdOperatorTextObj, Op: op, Count: count, Scope: scope}
		return true
	default:
		if ft, ok := vim.FindKeys[r]; ok {
			ip.vimCmd = vim.CommandState{Type: vim.CmdOperatorFind, Op: op, Count: count, Find: ft}
			return true
		}
		// dd/cc/yy — line operation
		if opKey, ok := vim.Operators[r]; ok && opKey == op {
			vim.ExecuteLineOp(op, count, ctx)
			ip.syncFromCtx(ctx)
		}
	}
	if op == vim.OpChange {
		ip.vimMode = vim.ModeInsert
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimFind(r rune) bool {
	ctx := ip.vimCtx()
	target := vim.FindCharacter(ctx.Text, ctx.Cursor, r, ip.vimCmd.Find, ip.vimCmd.Count)
	if target >= 0 {
		ip.cursor = target
		ip.vimPersist.LastFind = &vim.LastFindState{Type: ip.vimCmd.Find, Char: string(r)}
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimOperatorFind(r rune) bool {
	ctx := ip.vimCtx()
	vim.ExecuteOperatorFind(ip.vimCmd.Op, ip.vimCmd.Find, r, ip.vimCmd.Count, ctx)
	ip.syncFromCtx(ctx)
	ip.vimPersist.LastFind = &vim.LastFindState{Type: ip.vimCmd.Find, Char: string(r)}
	if ip.vimCmd.Op == vim.OpChange {
		ip.vimMode = vim.ModeInsert
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimOperatorTextObj(r rune) bool {
	if !vim.TextObjTypes[r] {
		ip.vimCmd = vim.IdleCommand()
		return true
	}
	ctx := ip.vimCtx()
	vim.ExecuteOperatorTextObj(ip.vimCmd.Op, ip.vimCmd.Scope, r, ctx)
	ip.syncFromCtx(ctx)
	if ip.vimCmd.Op == vim.OpChange {
		ip.vimMode = vim.ModeInsert
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimReplace(r rune) bool {
	ctx := ip.vimCtx()
	vim.ExecuteReplace(r, ip.vimCmd.Count, ctx)
	ip.syncFromCtx(ctx)
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimG(r rune) bool {
	ctx := ip.vimCtx()
	if r == 'g' {
		ip.cursor = vim.ResolveMotion("gg", ctx.Text, ctx.Cursor, ip.vimCmd.Count)
	} else if r == 'j' {
		ip.cursor = vim.ResolveMotion("j", ctx.Text, ctx.Cursor, ip.vimCmd.Count)
	} else if r == 'k' {
		ip.cursor = vim.ResolveMotion("k", ctx.Text, ctx.Cursor, ip.vimCmd.Count)
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimOperatorG(r rune) bool {
	if r == 'g' {
		ctx := ip.vimCtx()
		vim.ExecuteOperatorGg(ip.vimCmd.Op, ip.vimCmd.Count, ctx)
		ip.syncFromCtx(ctx)
		if ip.vimCmd.Op == vim.OpChange {
			ip.vimMode = vim.ModeInsert
		}
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

func (ip *InputPane) handleVimIndent(r rune) bool {
	if r == ip.vimCmd.Dir {
		ctx := ip.vimCtx()
		vim.ExecuteIndent(ip.vimCmd.Dir, ip.vimCmd.Count, ctx)
		ip.syncFromCtx(ctx)
	}
	ip.vimCmd = vim.IdleCommand()
	return true
}

// vimCtx builds an OperatorContext from the current InputPane state.
func (ip *InputPane) vimCtx() *vim.OperatorContext {
	return &vim.OperatorContext{
		Cursor: ip.cursor,
		Text:   string(ip.runes),
		SetText: func(s string) {
			ip.runes = []rune(s)
		},
		SetOffset: func(off int) {
			ip.cursor = off
		},
		EnterInsert: func(off int) {
			ip.cursor = off
			ip.vimMode = vim.ModeInsert
		},
		GetRegister: func() string {
			return ip.vimPersist.Register
		},
		SetRegister: func(content string, linewise bool) {
			ip.vimPersist.Register = content
			ip.vimPersist.RegisterLinewise = linewise
		},
	}
}

// syncFromCtx updates runes and cursor from the OperatorContext after a mutation.
func (ip *InputPane) syncFromCtx(ctx *vim.OperatorContext) {
	ip.runes = []rune(ctx.Text)
	ip.cursor = ctx.Cursor
	// Clamp cursor
	if ip.cursor < 0 {
		ip.cursor = 0
	}
	if ip.cursor > len(ip.runes) {
		ip.cursor = len(ip.runes)
	}
}

func parseCount(digits string) int {
	n := 0
	for _, d := range digits {
		n = n*10 + int(d-'0')
	}
	if n < 1 {
		return 1
	}
	if n > vim.MaxVimCount {
		return vim.MaxVimCount
	}
	return n
}

