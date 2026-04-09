package ui

import (
	"fmt"
	"io"
	"os"
	"strings"
)

// Source: ink/useTerminalNotification.ts, ink/termio/osc.ts
//
// Terminal notifications via OSC escape sequences. Supports iTerm2, Kitty,
// Ghostty, and plain BEL. Also supports progress reporting via OSC 9;4.

// ANSI/OSC constants
const (
	esc       = "\x1b"
	bel       = "\x07"
	oscPrefix = esc + "]" // ESC ]
	st        = esc + "\\" // String Terminator

	// OSC command numbers
	oscITerm2  = 9
	oscKitty   = 99
	oscGhostty = 777

	// iTerm2 OSC 9 subcommands
	iterm2Progress = 4

	// Progress operation codes
	progressClear         = 0
	progressSet           = 1
	progressError         = 2
	progressIndeterminate = 3
)

// ProgressState represents the state of a progress indicator.
type ProgressState string

const (
	ProgressRunning       ProgressState = "running"
	ProgressCompleted     ProgressState = "completed"
	ProgressError         ProgressState = "error"
	ProgressIndeterminate ProgressState = "indeterminate"
)

// TerminalNotifier sends terminal notifications via escape sequences.
type TerminalNotifier struct {
	w      io.Writer
	isKitty bool
	isTmux  bool
}

// NewTerminalNotifier creates a notifier that writes to the given writer.
// Pass nil to write to os.Stdout.
func NewTerminalNotifier(w io.Writer) *TerminalNotifier {
	if w == nil {
		w = os.Stdout
	}
	term := os.Getenv("TERM_PROGRAM")
	return &TerminalNotifier{
		w:       w,
		isKitty: term == "kitty",
		isTmux:  os.Getenv("TMUX") != "",
	}
}

// osc builds an OSC sequence: ESC ] <parts joined by ;> BEL (or ST for kitty)
func (n *TerminalNotifier) osc(parts ...interface{}) string {
	strs := make([]string, len(parts))
	for i, p := range parts {
		strs[i] = fmt.Sprint(p)
	}
	terminator := bel
	if n.isKitty {
		terminator = st
	}
	return oscPrefix + strings.Join(strs, ";") + terminator
}

// wrapForMultiplexer wraps an escape sequence for tmux DCS passthrough.
func (n *TerminalNotifier) wrapForMultiplexer(seq string) string {
	if !n.isTmux {
		return seq
	}
	escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;" + escaped + "\x1b\\"
}

func (n *TerminalNotifier) write(s string) {
	n.w.Write([]byte(s))
}

// NotifyITerm2 sends a notification via iTerm2's OSC 9 protocol.
func (n *TerminalNotifier) NotifyITerm2(message, title string) {
	display := message
	if title != "" {
		display = title + ":\n" + message
	}
	n.write(n.wrapForMultiplexer(n.osc(oscITerm2, "\n\n"+display)))
}

// NotifyKitty sends a notification via Kitty's OSC 99 protocol.
func (n *TerminalNotifier) NotifyKitty(message, title string, id int) {
	n.write(n.wrapForMultiplexer(n.osc(oscKitty, fmt.Sprintf("i=%d:d=0:p=title", id), title)))
	n.write(n.wrapForMultiplexer(n.osc(oscKitty, fmt.Sprintf("i=%d:p=body", id), message)))
	n.write(n.wrapForMultiplexer(n.osc(oscKitty, fmt.Sprintf("i=%d:d=1:a=focus", id), "")))
}

// NotifyGhostty sends a notification via Ghostty's OSC 777 protocol.
func (n *TerminalNotifier) NotifyGhostty(message, title string) {
	n.write(n.wrapForMultiplexer(n.osc(oscGhostty, "notify", title, message)))
}

// NotifyBell sends a plain BEL character (triggers terminal bell/notification).
func (n *TerminalNotifier) NotifyBell() {
	// Raw BEL — inside tmux this triggers tmux's bell-action.
	// Don't wrap for multiplexer (would make it DCS payload).
	n.write(bel)
}

// Progress reports progress to the terminal via iTerm2's OSC 9;4 sequences.
// Supported by: ConEmu, Ghostty 1.2.0+, iTerm2 3.6.6+.
// Pass empty state to clear progress.
func (n *TerminalNotifier) Progress(state ProgressState, percentage int) {
	if percentage < 0 {
		percentage = 0
	}
	if percentage > 100 {
		percentage = 100
	}

	switch state {
	case ProgressCompleted, "":
		n.write(n.wrapForMultiplexer(
			n.osc(oscITerm2, iterm2Progress, progressClear, ""),
		))
	case ProgressError:
		n.write(n.wrapForMultiplexer(
			n.osc(oscITerm2, iterm2Progress, progressError, percentage),
		))
	case ProgressIndeterminate:
		n.write(n.wrapForMultiplexer(
			n.osc(oscITerm2, iterm2Progress, progressIndeterminate, ""),
		))
	case ProgressRunning:
		n.write(n.wrapForMultiplexer(
			n.osc(oscITerm2, iterm2Progress, progressSet, percentage),
		))
	}
}

// ClearProgress clears any active progress indicator.
func (n *TerminalNotifier) ClearProgress() {
	n.Progress("", 0)
}

// SetTitle sets the terminal window title via OSC 0.
func (n *TerminalNotifier) SetTitle(title string) {
	n.write(n.wrapForMultiplexer(n.osc(0, title)))
}

// ClearTitle clears the terminal window title.
func (n *TerminalNotifier) ClearTitle() {
	n.write(n.wrapForMultiplexer(n.osc(0, "")))
}
