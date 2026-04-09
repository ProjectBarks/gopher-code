package ui

import (
	"strings"

	"charm.land/lipgloss/v2"
)

// Source: ink/searchHighlight.ts, ink/hooks/use-search-highlight.ts
//
// In TS, search highlighting operates on the Ink screen buffer (cell-level
// inversion). In Go/bubbletea, rendering is string-based, so we highlight
// at the text level — wrapping matches in ANSI styling before View() output.

// DefaultHighlightStyle is the default style for search matches (inverse).
var DefaultHighlightStyle = lipgloss.NewStyle().Reverse(true)

// CurrentMatchStyle is the style for the "current" match (yellow background).
var CurrentMatchStyle = lipgloss.NewStyle().Background(lipgloss.Color("3")).Foreground(lipgloss.Color("0"))

// HighlightMatches wraps all case-insensitive occurrences of query in text
// with the given lipgloss style. Returns the original text if query is empty.
// Non-overlapping matches (same as less/vim/grep behavior).
func HighlightMatches(text, query string, style lipgloss.Style) string {
	if query == "" || text == "" {
		return text
	}

	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	qlen := len(lowerQuery)

	// Fast path: no matches
	if !strings.Contains(lowerText, lowerQuery) {
		return text
	}

	var b strings.Builder
	b.Grow(len(text) + 64)

	prev := 0
	searchFrom := 0
	for {
		idx := strings.Index(lowerText[searchFrom:], lowerQuery)
		if idx < 0 {
			break
		}
		matchStart := searchFrom + idx
		matchEnd := matchStart + qlen

		// Write text before match (unstyled)
		b.WriteString(text[prev:matchStart])
		// Write match (styled)
		b.WriteString(style.Render(text[matchStart:matchEnd]))

		prev = matchEnd
		searchFrom = matchEnd // non-overlapping
	}
	// Write remaining text
	b.WriteString(text[prev:])

	return b.String()
}

// HighlightSearchResults highlights all matches with the default inverse style.
func HighlightSearchResults(text, query string) string {
	return HighlightMatches(text, query, DefaultHighlightStyle)
}

// CountMatches returns the number of non-overlapping case-insensitive matches.
func CountMatches(text, query string) int {
	if query == "" || text == "" {
		return 0
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	qlen := len(lowerQuery)

	count := 0
	searchFrom := 0
	for {
		idx := strings.Index(lowerText[searchFrom:], lowerQuery)
		if idx < 0 {
			break
		}
		count++
		searchFrom += idx + qlen
	}
	return count
}

// FindMatchPositions returns the byte offsets of all non-overlapping
// case-insensitive matches in text. Useful for "N of M" match navigation.
func FindMatchPositions(text, query string) []int {
	if query == "" || text == "" {
		return nil
	}
	lowerText := strings.ToLower(text)
	lowerQuery := strings.ToLower(query)
	qlen := len(lowerQuery)

	var positions []int
	searchFrom := 0
	for {
		idx := strings.Index(lowerText[searchFrom:], lowerQuery)
		if idx < 0 {
			break
		}
		positions = append(positions, searchFrom+idx)
		searchFrom += idx + qlen
	}
	return positions
}

// HighlightWithCurrent highlights all matches with inverse, and the current
// match (at currentIdx) with the yellow "current match" style.
// Used for Ctrl+F search navigation: "3 of 12 matches".
func HighlightWithCurrent(text, query string, currentIdx int) string {
	if query == "" || text == "" {
		return text
	}

	positions := FindMatchPositions(text, query)
	if len(positions) == 0 {
		return text
	}

	qlen := len(strings.ToLower(query)) // use lowered length for consistency
	var b strings.Builder
	b.Grow(len(text) + len(positions)*32)

	prev := 0
	for i, pos := range positions {
		end := pos + qlen
		b.WriteString(text[prev:pos])
		if i == currentIdx {
			b.WriteString(CurrentMatchStyle.Render(text[pos:end]))
		} else {
			b.WriteString(DefaultHighlightStyle.Render(text[pos:end]))
		}
		prev = end
	}
	b.WriteString(text[prev:])

	return b.String()
}
