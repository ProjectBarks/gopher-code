package ui

import (
	"strings"
	"testing"

	"charm.land/lipgloss/v2"
)

// plainStyle wraps in brackets for easy testing without ANSI codes.
var plainStyle = lipgloss.NewStyle().Bold(true)

func TestHighlightMatches_Basic(t *testing.T) {
	result := HighlightMatches("hello world", "world", DefaultHighlightStyle)
	if !strings.Contains(result, "hello ") {
		t.Error("non-match part should be preserved")
	}
	// The styled portion should be different from the plain text
	if result == "hello world" {
		t.Error("should contain styled text")
	}
}

func TestHighlightMatches_CaseInsensitive(t *testing.T) {
	result := HighlightMatches("Hello World", "hello", DefaultHighlightStyle)
	// Should match "Hello" case-insensitively
	if result == "Hello World" {
		t.Error("should highlight case-insensitively")
	}
	if !strings.Contains(result, " World") {
		t.Error("non-match 'World' should be preserved")
	}
}

func TestHighlightMatches_Multiple(t *testing.T) {
	result := HighlightMatches("foo bar foo baz foo", "foo", DefaultHighlightStyle)
	// Count styled segments - should be 3
	if result == "foo bar foo baz foo" {
		t.Error("should highlight all occurrences")
	}
	// Check that "bar" and "baz" survive unstyled
	if !strings.Contains(result, " bar ") {
		t.Error("'bar' should be unstyled")
	}
}

func TestHighlightMatches_EmptyQuery(t *testing.T) {
	result := HighlightMatches("hello", "", DefaultHighlightStyle)
	if result != "hello" {
		t.Error("empty query should return original text")
	}
}

func TestHighlightMatches_EmptyText(t *testing.T) {
	result := HighlightMatches("", "query", DefaultHighlightStyle)
	if result != "" {
		t.Error("empty text should return empty string")
	}
}

func TestHighlightMatches_NoMatch(t *testing.T) {
	result := HighlightMatches("hello world", "xyz", DefaultHighlightStyle)
	if result != "hello world" {
		t.Errorf("no match should return original, got %q", result)
	}
}

func TestHighlightMatches_NonOverlapping(t *testing.T) {
	// "aa" in "aaa" should match at position 0 only (non-overlapping)
	count := CountMatches("aaa", "aa")
	if count != 1 {
		t.Errorf("non-overlapping count = %d, want 1", count)
	}
}

func TestHighlightSearchResults(t *testing.T) {
	result := HighlightSearchResults("find me here", "me")
	if result == "find me here" {
		t.Error("should highlight 'me'")
	}
}

func TestCountMatches(t *testing.T) {
	tests := []struct {
		text, query string
		want        int
	}{
		{"hello world hello", "hello", 2},
		{"HELLO hello Hello", "hello", 3},
		{"no match here", "xyz", 0},
		{"", "test", 0},
		{"test", "", 0},
		{"aaa", "a", 3},
		{"aaa", "aa", 1}, // non-overlapping
	}
	for _, tt := range tests {
		t.Run(tt.text+"_"+tt.query, func(t *testing.T) {
			got := CountMatches(tt.text, tt.query)
			if got != tt.want {
				t.Errorf("CountMatches(%q, %q) = %d, want %d", tt.text, tt.query, got, tt.want)
			}
		})
	}
}

func TestFindMatchPositions(t *testing.T) {
	positions := FindMatchPositions("hello world hello", "hello")
	if len(positions) != 2 {
		t.Fatalf("expected 2 positions, got %d", len(positions))
	}
	if positions[0] != 0 {
		t.Errorf("first match at %d, want 0", positions[0])
	}
	if positions[1] != 12 {
		t.Errorf("second match at %d, want 12", positions[1])
	}
}

func TestFindMatchPositions_CaseInsensitive(t *testing.T) {
	positions := FindMatchPositions("Hello HELLO hello", "hello")
	if len(positions) != 3 {
		t.Fatalf("expected 3 positions, got %d", len(positions))
	}
}

func TestFindMatchPositions_Empty(t *testing.T) {
	if FindMatchPositions("text", "") != nil {
		t.Error("empty query should return nil")
	}
	if FindMatchPositions("", "query") != nil {
		t.Error("empty text should return nil")
	}
}

func TestHighlightWithCurrent(t *testing.T) {
	text := "foo bar foo baz foo"

	// Highlight with current at index 1 (second "foo")
	result := HighlightWithCurrent(text, "foo", 1)
	if result == text {
		t.Error("should have highlights")
	}
	// All three "foo" occurrences should be styled (we can't easily check
	// which style without parsing ANSI, but we can check the output differs)
	if len(result) <= len(text) {
		t.Error("result should be longer due to ANSI codes")
	}
}

func TestHighlightWithCurrent_NoMatch(t *testing.T) {
	result := HighlightWithCurrent("hello", "xyz", 0)
	if result != "hello" {
		t.Error("no match should return original")
	}
}

func TestHighlightWithCurrent_EmptyQuery(t *testing.T) {
	result := HighlightWithCurrent("hello", "", 0)
	if result != "hello" {
		t.Error("empty query should return original")
	}
}
