package text

// Source: ink/wrap-text.ts, ink/stringWidth.ts, ink/tabstops.ts, ink/wrapAnsi.ts
//
// ANSI-aware text processing: display width, truncation with position control,
// word wrapping, and tab expansion.

import (
	"strings"
	"unicode"
	"unicode/utf8"
)

const Ellipsis = "…"

// DisplayWidth returns the visible width of a string, ignoring ANSI escapes.
// Handles wide characters (CJK) as width 2 and control chars as width 0.
// Source: ink/stringWidth.ts
func DisplayWidth(s string) int {
	stripped := StripANSI(s)
	width := 0
	for _, r := range stripped {
		width += RuneWidth(r)
	}
	return width
}

// RuneWidth returns the display width of a single rune.
// CJK characters and fullwidth forms are width 2, control chars are 0.
func RuneWidth(r rune) int {
	if r < 32 || r == 127 {
		return 0 // control characters
	}
	if isWide(r) {
		return 2
	}
	return 1
}

// isWide returns true for CJK and fullwidth characters.
func isWide(r rune) bool {
	// CJK Unified Ideographs
	if r >= 0x4E00 && r <= 0x9FFF {
		return true
	}
	// CJK Unified Ideographs Extension A
	if r >= 0x3400 && r <= 0x4DBF {
		return true
	}
	// CJK Compatibility Ideographs
	if r >= 0xF900 && r <= 0xFAFF {
		return true
	}
	// Fullwidth forms
	if r >= 0xFF01 && r <= 0xFF60 {
		return true
	}
	// Halfwidth and Fullwidth Forms (fullwidth part)
	if r >= 0xFFE0 && r <= 0xFFE6 {
		return true
	}
	// Hangul Syllables
	if r >= 0xAC00 && r <= 0xD7AF {
		return true
	}
	// CJK Radicals
	if r >= 0x2E80 && r <= 0x2FDF {
		return true
	}
	// Katakana/Hiragana
	if r >= 0x3000 && r <= 0x303F {
		return true
	}
	if r >= 0x3040 && r <= 0x30FF {
		return true
	}
	return false
}

// TruncatePosition specifies where truncation occurs.
type TruncatePosition int

const (
	TruncateEnd    TruncatePosition = iota // "hello wo…"
	TruncateStart                          // "…lo world"
	TruncateMiddle                         // "hel…orld"
)

// TruncateAnsi truncates an ANSI-styled string to a display width,
// adding an ellipsis at the specified position.
// Source: ink/wrap-text.ts:truncate
func TruncateAnsi(s string, columns int, position TruncatePosition) string {
	if columns < 1 {
		return ""
	}
	if columns == 1 {
		return Ellipsis
	}

	width := DisplayWidth(s)
	if width <= columns {
		return s
	}

	stripped := StripANSI(s)
	runes := []rune(stripped)

	switch position {
	case TruncateStart:
		// "…" + last (columns-1) chars
		result := sliceRunesByWidth(runes, width-(columns-1), width)
		return Ellipsis + result

	case TruncateMiddle:
		half := columns / 2
		left := sliceRunesByWidth(runes, 0, half)
		right := sliceRunesByWidth(runes, width-(columns-half)+1, width)
		return left + Ellipsis + right

	default: // TruncateEnd
		result := sliceRunesByWidth(runes, 0, columns-1)
		return result + Ellipsis
	}
}

// sliceRunesByWidth extracts runes from startWidth to endWidth (display widths).
func sliceRunesByWidth(runes []rune, startWidth, endWidth int) string {
	var b strings.Builder
	w := 0
	for _, r := range runes {
		rw := RuneWidth(r)
		if w >= startWidth && w+rw <= endWidth {
			b.WriteRune(r)
		}
		w += rw
		if w >= endWidth {
			break
		}
	}
	return b.String()
}

// WrapText wraps text to a maximum width.
// Respects word boundaries when possible (breaks on space/hyphen).
// Source: ink/wrap-text.ts:wrapText, ink/wrapAnsi.ts
func WrapText(s string, maxWidth int) string {
	if maxWidth <= 0 {
		return s
	}

	lines := strings.Split(s, "\n")
	var result []string

	for _, line := range lines {
		if DisplayWidth(line) <= maxWidth {
			result = append(result, line)
			continue
		}
		result = append(result, wrapLine(line, maxWidth)...)
	}

	return strings.Join(result, "\n")
}

// wrapLine wraps a single line to maxWidth, breaking at word boundaries.
func wrapLine(line string, maxWidth int) []string {
	stripped := StripANSI(line)
	words := splitWords(stripped)

	var lines []string
	var current strings.Builder
	currentWidth := 0

	for _, word := range words {
		wordWidth := DisplayWidth(word)

		if currentWidth == 0 {
			// First word on line
			if wordWidth > maxWidth {
				// Word itself is too long — hard break
				lines = append(lines, hardBreak(word, maxWidth)...)
			} else {
				current.WriteString(word)
				currentWidth = wordWidth
			}
			continue
		}

		// Check if word fits on current line (with space)
		if currentWidth+1+wordWidth <= maxWidth {
			current.WriteString(" ")
			current.WriteString(word)
			currentWidth += 1 + wordWidth
		} else {
			// Start new line
			lines = append(lines, current.String())
			current.Reset()
			if wordWidth > maxWidth {
				lines = append(lines, hardBreak(word, maxWidth)...)
				currentWidth = 0
			} else {
				current.WriteString(word)
				currentWidth = wordWidth
			}
		}
	}

	if current.Len() > 0 {
		lines = append(lines, current.String())
	}

	if len(lines) == 0 {
		return []string{""}
	}
	return lines
}

// hardBreak splits a word that's longer than maxWidth into chunks.
func hardBreak(word string, maxWidth int) []string {
	var chunks []string
	runes := []rune(word)
	start := 0
	w := 0

	for i, r := range runes {
		rw := RuneWidth(r)
		if w+rw > maxWidth && i > start {
			chunks = append(chunks, string(runes[start:i]))
			start = i
			w = 0
		}
		w += rw
	}
	if start < len(runes) {
		chunks = append(chunks, string(runes[start:]))
	}
	return chunks
}

// splitWords splits text on whitespace, preserving no empty segments.
func splitWords(s string) []string {
	return strings.Fields(s)
}

// ExpandTabs replaces tab characters with spaces, aligned to tabstops.
// Source: ink/tabstops.ts
func ExpandTabs(s string, tabWidth int) string {
	if tabWidth <= 0 {
		tabWidth = 8
	}
	if !strings.ContainsRune(s, '\t') {
		return s
	}

	var b strings.Builder
	col := 0
	for _, r := range s {
		if r == '\t' {
			spaces := tabWidth - (col % tabWidth)
			b.WriteString(strings.Repeat(" ", spaces))
			col += spaces
		} else if r == '\n' {
			b.WriteRune(r)
			col = 0
		} else {
			b.WriteRune(r)
			col += RuneWidth(r)
		}
	}
	return b.String()
}

// PadRight pads a string with spaces to reach the target display width.
func PadRight(s string, width int) string {
	w := DisplayWidth(s)
	if w >= width {
		return s
	}
	return s + strings.Repeat(" ", width-w)
}

// PadLeft pads a string with spaces on the left.
func PadLeft(s string, width int) string {
	w := DisplayWidth(s)
	if w >= width {
		return s
	}
	return strings.Repeat(" ", width-w) + s
}

// Center centers a string within the given width.
func Center(s string, width int) string {
	w := DisplayWidth(s)
	if w >= width {
		return s
	}
	left := (width - w) / 2
	right := width - w - left
	return strings.Repeat(" ", left) + s + strings.Repeat(" ", right)
}

// IsWhitespace returns true if the string is empty or contains only whitespace.
func IsWhitespace(s string) bool {
	for _, r := range s {
		if !unicode.IsSpace(r) {
			return false
		}
	}
	return true
}

// VisibleLength returns the number of visible runes (non-ANSI, non-control).
func VisibleLength(s string) int {
	return utf8.RuneCountInString(StripANSI(s))
}
