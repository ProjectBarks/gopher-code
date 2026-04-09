// Package termio provides terminal I/O escape sequences.
//
// Source: ink/termio/ansi.ts, csi.ts, dec.ts, esc.ts, osc.ts, sgr.ts
//
// ANSI/CSI/DEC/ESC/OSC/SGR escape sequence constants and builders.
// These are the raw terminal control sequences used by the rendering
// layer for cursor movement, colors, screen clearing, etc.
package termio

import (
	"fmt"
	"strings"
)

// ---------------------------------------------------------------------------
// ANSI constants — Source: ink/termio/ansi.ts
// ---------------------------------------------------------------------------

const (
	ESC = "\x1b"
	BEL = "\x07"
	SEP = ";"
)

// Escape sequence type introducers (byte after ESC)
const (
	CSIPrefix = ESC + "[" // Control Sequence Introducer
	OSCPrefix = ESC + "]" // Operating System Command
	DCSPrefix = ESC + "P" // Device Control String
	ST        = ESC + `\` // String Terminator
)

// ---------------------------------------------------------------------------
// CSI (Control Sequence Introducer) — Source: ink/termio/csi.ts
// ---------------------------------------------------------------------------

// CSI builds a CSI sequence: ESC [ params final
func CSI(parts ...interface{}) string {
	if len(parts) == 0 {
		return CSIPrefix
	}
	if len(parts) == 1 {
		return CSIPrefix + fmt.Sprint(parts[0])
	}
	// Last part is the final byte, rest are params joined by ;
	params := make([]string, len(parts)-1)
	for i, p := range parts[:len(parts)-1] {
		params[i] = fmt.Sprint(p)
	}
	return CSIPrefix + strings.Join(params, SEP) + fmt.Sprint(parts[len(parts)-1])
}

// Cursor movement
func CursorUp(n int) string       { return CSI(n, "A") }
func CursorDown(n int) string     { return CSI(n, "B") }
func CursorForward(n int) string  { return CSI(n, "C") }
func CursorBack(n int) string     { return CSI(n, "D") }
func CursorPosition(row, col int) string { return CSI(row, col, "H") }
func CursorColumn(col int) string { return CSI(col, "G") }

// Cursor visibility
var (
	CursorShow = CSI("?25h")
	CursorHide = CSI("?25l")
)

// Screen operations
var (
	ClearScreen       = CSI("2J")    // Clear entire screen
	ClearScreenBelow  = CSI("0J")    // Clear from cursor to end
	ClearScreenAbove  = CSI("1J")    // Clear from start to cursor
	ClearLine         = CSI("2K")    // Clear entire line
	ClearLineRight    = CSI("0K")    // Clear from cursor to end of line
	ClearLineLeft     = CSI("1K")    // Clear from start to cursor
)

// Scroll
func ScrollUp(n int) string   { return CSI(n, "S") }
func ScrollDown(n int) string { return CSI(n, "T") }

// Screen modes
var (
	EnableAlternateScreen  = CSI("?1049h")
	DisableAlternateScreen = CSI("?1049l")
	EnableBracketedPaste   = CSI("?2004h")
	DisableBracketedPaste  = CSI("?2004l")
)

// Mouse tracking
var (
	EnableMouseTracking    = CSI("?1000h") + CSI("?1002h") + CSI("?1003h") + CSI("?1006h")
	DisableMouseTracking   = CSI("?1006l") + CSI("?1003l") + CSI("?1002l") + CSI("?1000l")
)

// Focus reporting
var (
	EnableFocusReporting  = CSI("?1004h")
	DisableFocusReporting = CSI("?1004l")
)

// ---------------------------------------------------------------------------
// DEC Private Modes — Source: ink/termio/dec.ts
// ---------------------------------------------------------------------------

// SetDecPrivateMode enables a DEC private mode.
func SetDecPrivateMode(mode int) string { return CSI("?" + fmt.Sprint(mode) + "h") }

// ResetDecPrivateMode disables a DEC private mode.
func ResetDecPrivateMode(mode int) string { return CSI("?" + fmt.Sprint(mode) + "l") }

// ---------------------------------------------------------------------------
// OSC (Operating System Command) — Source: ink/termio/osc.ts
// ---------------------------------------------------------------------------

// OSC builds an OSC sequence: ESC ] parts; BEL
func OSC(parts ...interface{}) string {
	strs := make([]string, len(parts))
	for i, p := range parts {
		strs[i] = fmt.Sprint(p)
	}
	return OSCPrefix + strings.Join(strs, SEP) + BEL
}

// OSC command numbers
const (
	OSCSetTitle    = 0  // Set window title
	OSCHyperlink   = 8  // Hyperlink
	OSCITerm2      = 9  // iTerm2 proprietary
	OSCKitty       = 99 // Kitty notification
	OSCGhostty     = 777 // Ghostty notification
)

// SetTitle sets the terminal window title.
func SetTitle(title string) string { return OSC(OSCSetTitle, title) }

// ClearTitle clears the terminal window title.
func ClearTitle() string { return OSC(OSCSetTitle, "") }

// Hyperlink wraps text in an OSC 8 hyperlink.
func Hyperlink(url, text string) string {
	return OSCPrefix + fmt.Sprintf("8;;%s", url) + BEL + text + OSCPrefix + "8;;" + BEL
}

// ---------------------------------------------------------------------------
// SGR (Select Graphic Rendition) — Source: ink/termio/sgr.ts
// ---------------------------------------------------------------------------

// SGR constants
const (
	SGRReset     = 0
	SGRBold      = 1
	SGRDim       = 2
	SGRItalic    = 3
	SGRUnderline = 4
	SGRBlink     = 5
	SGRInverse   = 7
	SGRHidden    = 8
	SGRStrikethrough = 9

	// Foreground colors (30-37, 90-97)
	SGRFgBlack   = 30
	SGRFgRed     = 31
	SGRFgGreen   = 32
	SGRFgYellow  = 33
	SGRFgBlue    = 34
	SGRFgMagenta = 35
	SGRFgCyan    = 36
	SGRFgWhite   = 37
	SGRFgDefault = 39

	// Background colors (40-47, 100-107)
	SGRBgBlack   = 40
	SGRBgRed     = 41
	SGRBgGreen   = 42
	SGRBgYellow  = 43
	SGRBgBlue    = 44
	SGRBgMagenta = 45
	SGRBgCyan    = 46
	SGRBgWhite   = 47
	SGRBgDefault = 49

	// Bright foreground (90-97)
	SGRFgBrightBlack   = 90
	SGRFgBrightRed     = 91
	SGRFgBrightGreen   = 92
	SGRFgBrightYellow  = 93
	SGRFgBrightBlue    = 94
	SGRFgBrightMagenta = 95
	SGRFgBrightCyan    = 96
	SGRFgBrightWhite   = 97
)

// SGR builds an SGR sequence: ESC [ params m
func SGR(codes ...int) string {
	if len(codes) == 0 {
		return CSIPrefix + "0m"
	}
	parts := make([]string, len(codes))
	for i, c := range codes {
		parts[i] = fmt.Sprint(c)
	}
	return CSIPrefix + strings.Join(parts, ";") + "m"
}

// ResetSGR resets all SGR attributes.
var ResetSGR = SGR(SGRReset)

// FG256 sets the foreground to a 256-color palette index.
func FG256(index int) string { return SGR(38, 5, index) }

// BG256 sets the background to a 256-color palette index.
func BG256(index int) string { return SGR(48, 5, index) }

// FGRGB sets the foreground to an RGB color.
func FGRGB(r, g, b int) string { return SGR(38, 2, r, g, b) }

// BGRGB sets the background to an RGB color.
func BGRGB(r, g, b int) string { return SGR(48, 2, r, g, b) }

// ---------------------------------------------------------------------------
// Tmux wrapping — Source: ink/termio/osc.ts:wrapForMultiplexer
// ---------------------------------------------------------------------------

// WrapForTmux wraps an escape sequence for tmux DCS passthrough.
func WrapForTmux(seq string) string {
	escaped := strings.ReplaceAll(seq, "\x1b", "\x1b\x1b")
	return "\x1bPtmux;" + escaped + "\x1b\\"
}
