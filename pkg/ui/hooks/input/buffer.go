package input

import (
	"sync"
)

// InputBuffer manages the raw text buffer for an input field. It tracks the
// rune slice, cursor position, and selection range. This is a lower-level
// primitive that InputHistory and HistorySearch build on top of.
//
// In the TS codebase this state is spread across useTextInput, useInputBuffer,
// and the React component state. In Go it is a single struct.
type InputBuffer struct {
	mu     sync.Mutex
	runes  []rune
	cursor int // rune offset
}

// NewInputBuffer returns an empty InputBuffer.
func NewInputBuffer() *InputBuffer {
	return &InputBuffer{}
}

// Value returns the buffer contents as a string.
func (b *InputBuffer) Value() string {
	b.mu.Lock()
	defer b.mu.Unlock()
	return string(b.runes)
}

// SetValue replaces the buffer contents and moves the cursor to the end.
func (b *InputBuffer) SetValue(s string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runes = []rune(s)
	b.cursor = len(b.runes)
}

// Clear empties the buffer and resets the cursor.
func (b *InputBuffer) Clear() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runes = b.runes[:0]
	b.cursor = 0
}

// Cursor returns the current cursor position (rune offset).
func (b *InputBuffer) Cursor() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return b.cursor
}

// SetCursor sets the cursor position, clamped to [0, len].
func (b *InputBuffer) SetCursor(pos int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if pos < 0 {
		pos = 0
	}
	if pos > len(b.runes) {
		pos = len(b.runes)
	}
	b.cursor = pos
}

// Len returns the number of runes in the buffer.
func (b *InputBuffer) Len() int {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.runes)
}

// IsEmpty returns true if the buffer has no content.
func (b *InputBuffer) IsEmpty() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	return len(b.runes) == 0
}

// Insert inserts text at the current cursor position and advances the cursor.
func (b *InputBuffer) Insert(text string) {
	b.mu.Lock()
	defer b.mu.Unlock()
	runes := []rune(text)
	newRunes := make([]rune, 0, len(b.runes)+len(runes))
	newRunes = append(newRunes, b.runes[:b.cursor]...)
	newRunes = append(newRunes, runes...)
	newRunes = append(newRunes, b.runes[b.cursor:]...)
	b.runes = newRunes
	b.cursor += len(runes)
}

// DeleteBackward removes the rune before the cursor (backspace).
// Returns true if a character was deleted.
func (b *InputBuffer) DeleteBackward() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cursor == 0 {
		return false
	}
	b.runes = append(b.runes[:b.cursor-1], b.runes[b.cursor:]...)
	b.cursor--
	return true
}

// DeleteForward removes the rune at the cursor (delete key).
// Returns true if a character was deleted.
func (b *InputBuffer) DeleteForward() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cursor >= len(b.runes) {
		return false
	}
	b.runes = append(b.runes[:b.cursor], b.runes[b.cursor+1:]...)
	return true
}

// DeleteWordBackward removes the word before the cursor (Ctrl+W).
// Returns true if anything was deleted.
func (b *InputBuffer) DeleteWordBackward() bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cursor == 0 {
		return false
	}
	pos := b.cursor
	// Skip trailing spaces.
	for pos > 0 && b.runes[pos-1] == ' ' {
		pos--
	}
	// Skip word characters.
	for pos > 0 && b.runes[pos-1] != ' ' {
		pos--
	}
	if pos == b.cursor {
		return false
	}
	b.runes = append(b.runes[:pos], b.runes[b.cursor:]...)
	b.cursor = pos
	return true
}

// KillToEnd removes all runes from the cursor to the end (Ctrl+K).
func (b *InputBuffer) KillToEnd() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runes = b.runes[:b.cursor]
}

// KillToStart removes all runes from the start to the cursor (Ctrl+U).
func (b *InputBuffer) KillToStart() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.runes = b.runes[b.cursor:]
	b.cursor = 0
}

// MoveLeft moves the cursor one rune to the left.
func (b *InputBuffer) MoveLeft() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cursor > 0 {
		b.cursor--
	}
}

// MoveRight moves the cursor one rune to the right.
func (b *InputBuffer) MoveRight() {
	b.mu.Lock()
	defer b.mu.Unlock()
	if b.cursor < len(b.runes) {
		b.cursor++
	}
}

// MoveToStart moves the cursor to position 0.
func (b *InputBuffer) MoveToStart() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cursor = 0
}

// MoveToEnd moves the cursor to the end of the buffer.
func (b *InputBuffer) MoveToEnd() {
	b.mu.Lock()
	defer b.mu.Unlock()
	b.cursor = len(b.runes)
}
