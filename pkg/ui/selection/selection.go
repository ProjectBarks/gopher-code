// Package selection provides text selection state management.
//
// Source: ink/selection.ts
//
// In TS, Ink tracks cell-level selection in a screen buffer. In Go/bubbletea,
// we track line-based selection in View() output coordinates. Most terminals
// handle native mouse selection; this package supports programmatic selection
// for copy-on-select and selection highlighting.
package selection

import (
	"strings"
)

// Point is a screen coordinate (0-indexed).
type Point struct {
	Col int
	Row int
}

// State tracks the current text selection.
type State struct {
	// Anchor is where the selection started (mouse-down). Nil = no selection.
	Anchor *Point
	// Focus is the current drag position. Nil = no selection.
	Focus *Point
	// IsDragging is true between mouse-down and mouse-up.
	IsDragging bool
	// Mode indicates the selection granularity.
	Mode SelectionMode
}

// SelectionMode controls selection granularity.
type SelectionMode int

const (
	ModeChar SelectionMode = iota // character-level
	ModeWord                      // word-level (double-click)
	ModeLine                      // line-level (triple-click)
)

// New creates an empty selection state.
func New() *State {
	return &State{}
}

// Start begins a new selection at the given point.
func (s *State) Start(col, row int, mode SelectionMode) {
	s.Anchor = &Point{Col: col, Row: row}
	s.Focus = &Point{Col: col, Row: row}
	s.IsDragging = true
	s.Mode = mode
}

// Update moves the focus to the current mouse position during drag.
func (s *State) Update(col, row int) {
	if !s.IsDragging || s.Anchor == nil {
		return
	}
	s.Focus = &Point{Col: col, Row: row}
}

// End completes the selection (mouse-up).
func (s *State) End() {
	s.IsDragging = false
}

// Clear removes the selection.
func (s *State) Clear() {
	s.Anchor = nil
	s.Focus = nil
	s.IsDragging = false
	s.Mode = ModeChar
}

// HasSelection returns true if there's an active selection.
func (s *State) HasSelection() bool {
	return s.Anchor != nil && s.Focus != nil
}

// IsEmpty returns true if anchor == focus (zero-length selection).
func (s *State) IsEmpty() bool {
	if !s.HasSelection() {
		return true
	}
	return s.Anchor.Col == s.Focus.Col && s.Anchor.Row == s.Focus.Row
}

// Normalized returns the selection bounds with start ≤ end.
func (s *State) Normalized() (start, end Point) {
	if !s.HasSelection() {
		return Point{}, Point{}
	}
	a, f := *s.Anchor, *s.Focus
	if a.Row < f.Row || (a.Row == f.Row && a.Col <= f.Col) {
		return a, f
	}
	return f, a
}

// ContainsPoint returns true if the point is within the selection range.
func (s *State) ContainsPoint(col, row int) bool {
	if !s.HasSelection() {
		return false
	}
	start, end := s.Normalized()

	if row < start.Row || row > end.Row {
		return false
	}
	if row == start.Row && row == end.Row {
		return col >= start.Col && col <= end.Col
	}
	if row == start.Row {
		return col >= start.Col
	}
	if row == end.Row {
		return col <= end.Col
	}
	return true // between start and end rows
}

// GetSelectedText extracts the selected text from a multi-line string.
func (s *State) GetSelectedText(content string) string {
	if !s.HasSelection() || s.IsEmpty() {
		return ""
	}

	lines := strings.Split(content, "\n")
	start, end := s.Normalized()

	if start.Row >= len(lines) {
		return ""
	}

	var result strings.Builder

	for row := start.Row; row <= end.Row && row < len(lines); row++ {
		line := lines[row]
		runes := []rune(line)

		var startCol, endCol int
		if row == start.Row {
			startCol = min(start.Col, len(runes))
		} else {
			startCol = 0
		}
		if row == end.Row {
			endCol = min(end.Col+1, len(runes))
		} else {
			endCol = len(runes)
		}

		if startCol < endCol {
			result.WriteString(string(runes[startCol:endCol]))
		}
		if row < end.Row {
			result.WriteString("\n")
		}
	}

	return result.String()
}

// ExpandToWord expands a point to word boundaries in the given line.
func ExpandToWord(line string, col int) (startCol, endCol int) {
	runes := []rune(line)
	if col >= len(runes) {
		return col, col
	}

	// Find word start (scan left)
	startCol = col
	for startCol > 0 && isWordChar(runes[startCol-1]) {
		startCol--
	}

	// Find word end (scan right)
	endCol = col
	for endCol < len(runes)-1 && isWordChar(runes[endCol+1]) {
		endCol++
	}

	return startCol, endCol
}

// ExpandToLine returns the full line bounds.
func ExpandToLine(line string) (startCol, endCol int) {
	return 0, max(0, len([]rune(line))-1)
}

func isWordChar(r rune) bool {
	return (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') ||
		(r >= '0' && r <= '9') || r == '_' || r == '-' || r == '.'
}
