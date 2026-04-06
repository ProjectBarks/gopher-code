package vim

import (
	"strings"
	"unicode"
)

// ResolveMotion applies a motion key `count` times from the given cursor
// position in `text`. Returns the new rune offset. Pure function.
//
// Supported motions: h l j k w b e W B E 0 ^ $ G gg
// Source: src/vim/motions.ts — resolveMotion / applySingleMotion
func ResolveMotion(key string, text string, cursor int, count int) int {
	runes := []rune(text)
	pos := clampCursor(cursor, len(runes))
	for i := 0; i < count; i++ {
		next := applySingleMotion(key, runes, pos)
		if next == pos {
			break
		}
		pos = next
	}
	return pos
}

func applySingleMotion(key string, runes []rune, pos int) int {
	switch key {
	case "h":
		return motionLeft(runes, pos)
	case "l":
		return motionRight(runes, pos)
	case "j":
		return motionDown(runes, pos)
	case "k":
		return motionUp(runes, pos)
	case "w":
		return motionNextWord(runes, pos, false)
	case "b":
		return motionPrevWord(runes, pos, false)
	case "e":
		return motionEndWord(runes, pos, false)
	case "W":
		return motionNextWord(runes, pos, true)
	case "B":
		return motionPrevWord(runes, pos, true)
	case "E":
		return motionEndWord(runes, pos, true)
	case "0":
		return motionStartOfLine(runes, pos)
	case "^":
		return motionFirstNonBlank(runes, pos)
	case "$":
		return motionEndOfLine(runes, pos)
	case "G":
		return motionLastLine(runes)
	case "gg":
		return 0
	default:
		return pos
	}
}

// IsInclusiveMotion returns true for motions that include the destination char.
func IsInclusiveMotion(key string) bool {
	return key == "e" || key == "E" || key == "$"
}

// IsLinewiseMotion returns true for motions that operate on full lines.
func IsLinewiseMotion(key string) bool {
	return key == "j" || key == "k" || key == "G" || key == "gg"
}

// --- motion primitives ---

func motionLeft(_ []rune, pos int) int {
	if pos > 0 {
		return pos - 1
	}
	return 0
}

func motionRight(runes []rune, pos int) int {
	if pos < len(runes)-1 {
		return pos + 1
	}
	return pos
}

func motionDown(runes []rune, pos int) int {
	text := string(runes)
	lineStart := strings.LastIndex(text[:runeOffsetToByteOffset(text, pos)+1], "\n")
	if lineStart == -1 {
		lineStart = 0
	} else {
		lineStart = byteOffsetToRuneOffset(text, lineStart) + 1
	}
	col := pos - lineStart

	// Find next newline
	nextNL := -1
	for i := pos; i < len(runes); i++ {
		if runes[i] == '\n' {
			nextNL = i
			break
		}
	}
	if nextNL == -1 {
		return pos // already on last line
	}

	// Find end of next line
	nextLineStart := nextNL + 1
	nextLineEnd := len(runes)
	for i := nextLineStart; i < len(runes); i++ {
		if runes[i] == '\n' {
			nextLineEnd = i
			break
		}
	}

	target := nextLineStart + col
	if target > nextLineEnd {
		target = nextLineEnd
	}
	if target >= len(runes) {
		target = len(runes) - 1
	}
	if target < 0 {
		target = 0
	}
	return target
}

func motionUp(runes []rune, pos int) int {
	text := string(runes)
	// Find start of current line
	curLineStart := 0
	for i := pos - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			curLineStart = i + 1
			break
		}
	}
	if curLineStart == 0 {
		return pos // already on first line
	}
	col := pos - curLineStart

	// Find start of previous line
	prevLineEnd := curLineStart - 1 // the \n
	prevLineStart := 0
	for i := prevLineEnd - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			prevLineStart = i + 1
			break
		}
	}

	_ = text // used for byte/rune conversion helpers above
	target := prevLineStart + col
	if target > prevLineEnd {
		target = prevLineEnd
	}
	if target < 0 {
		target = 0
	}
	return target
}

func motionNextWord(runes []rune, pos int, bigWord bool) int {
	n := len(runes)
	if pos >= n-1 {
		return pos
	}
	i := pos
	// Skip current word characters
	if bigWord {
		for i < n && !isWhitespace(runes[i]) {
			i++
		}
	} else {
		if isWordChar(runes[i]) {
			for i < n && isWordChar(runes[i]) {
				i++
			}
		} else if isPunctuation(runes[i]) {
			for i < n && isPunctuation(runes[i]) {
				i++
			}
		}
	}
	// Skip whitespace
	for i < n && isWhitespace(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	return i
}

func motionPrevWord(runes []rune, pos int, bigWord bool) int {
	if pos <= 0 {
		return 0
	}
	i := pos - 1
	// Skip whitespace backwards
	for i > 0 && isWhitespace(runes[i]) {
		i--
	}
	// Skip word or punct backwards
	if bigWord {
		for i > 0 && !isWhitespace(runes[i-1]) {
			i--
		}
	} else {
		if isWordChar(runes[i]) {
			for i > 0 && isWordChar(runes[i-1]) {
				i--
			}
		} else if isPunctuation(runes[i]) {
			for i > 0 && isPunctuation(runes[i-1]) {
				i--
			}
		}
	}
	return i
}

func motionEndWord(runes []rune, pos int, bigWord bool) int {
	n := len(runes)
	if pos >= n-1 {
		return pos
	}
	i := pos + 1
	// Skip whitespace
	for i < n && isWhitespace(runes[i]) {
		i++
	}
	if i >= n {
		return n - 1
	}
	// Move to end of word
	if bigWord {
		for i < n-1 && !isWhitespace(runes[i+1]) {
			i++
		}
	} else {
		if isWordChar(runes[i]) {
			for i < n-1 && isWordChar(runes[i+1]) {
				i++
			}
		} else if isPunctuation(runes[i]) {
			for i < n-1 && isPunctuation(runes[i+1]) {
				i++
			}
		}
	}
	return i
}

func motionStartOfLine(runes []rune, pos int) int {
	for i := pos - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

func motionFirstNonBlank(runes []rune, pos int) int {
	start := motionStartOfLine(runes, pos)
	for i := start; i < len(runes); i++ {
		if runes[i] == '\n' {
			return start
		}
		if !isWhitespace(runes[i]) {
			return i
		}
	}
	return start
}

func motionEndOfLine(runes []rune, pos int) int {
	for i := pos; i < len(runes); i++ {
		if runes[i] == '\n' {
			if i > 0 {
				return i - 1
			}
			return i
		}
	}
	if len(runes) > 0 {
		return len(runes) - 1
	}
	return 0
}

func motionLastLine(runes []rune) int {
	// Go to the start of the last line
	for i := len(runes) - 1; i >= 0; i-- {
		if runes[i] == '\n' {
			return i + 1
		}
	}
	return 0
}

// GoToLine returns the rune offset of the start of line n (1-based).
func GoToLine(text string, line int) int {
	if line <= 1 {
		return 0
	}
	runes := []rune(text)
	n := 1
	for i, r := range runes {
		if r == '\n' {
			n++
			if n == line {
				return i + 1
			}
		}
	}
	// Line beyond end — go to start of last line
	return motionLastLine(runes)
}

// FindCharacter searches for char in the given direction from pos.
// Returns the target rune offset, or -1 if not found.
func FindCharacter(text string, pos int, char rune, ft FindType, count int) int {
	runes := []rune(text)
	switch ft {
	case FindF: // forward to char
		found := 0
		for i := pos + 1; i < len(runes); i++ {
			if runes[i] == '\n' {
				break
			}
			if runes[i] == char {
				found++
				if found == count {
					return i
				}
			}
		}
	case FindB: // backward to char
		found := 0
		for i := pos - 1; i >= 0; i-- {
			if runes[i] == '\n' {
				break
			}
			if runes[i] == char {
				found++
				if found == count {
					return i
				}
			}
		}
	case FindT: // forward till (one before char)
		found := 0
		for i := pos + 1; i < len(runes); i++ {
			if runes[i] == '\n' {
				break
			}
			if runes[i] == char {
				found++
				if found == count {
					if i-1 > pos {
						return i - 1
					}
					return pos
				}
			}
		}
	case FindR: // backward till (one after char)
		found := 0
		for i := pos - 1; i >= 0; i-- {
			if runes[i] == '\n' {
				break
			}
			if runes[i] == char {
				found++
				if found == count {
					if i+1 < pos {
						return i + 1
					}
					return pos
				}
			}
		}
	}
	return -1
}

// --- Character classification (matches TS isVimWordChar, etc.) ---

func isWordChar(r rune) bool {
	return unicode.IsLetter(r) || unicode.IsDigit(r) || r == '_'
}

func isWhitespace(r rune) bool {
	return r == ' ' || r == '\t' || r == '\n' || r == '\r'
}

func isPunctuation(r rune) bool {
	return !isWordChar(r) && !isWhitespace(r)
}

// IsVimWordChar is exported for text objects to use.
func IsVimWordChar(r rune) bool  { return isWordChar(r) }

// IsVimWhitespace is exported for text objects to use.
func IsVimWhitespace(r rune) bool { return isWhitespace(r) }

// IsVimPunctuation is exported for text objects to use.
func IsVimPunctuation(r rune) bool { return isPunctuation(r) }

// --- helpers ---

func clampCursor(pos, length int) int {
	if pos < 0 {
		return 0
	}
	if length == 0 {
		return 0
	}
	if pos >= length {
		return length - 1
	}
	return pos
}

func runeOffsetToByteOffset(s string, runeOff int) int {
	i := 0
	for idx := range s {
		if i == runeOff {
			return idx
		}
		i++
	}
	return len(s)
}

func byteOffsetToRuneOffset(s string, byteOff int) int {
	return len([]rune(s[:byteOff]))
}
