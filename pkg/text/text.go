// Package text provides text processing utilities.
// Source: utils/text/ (truncation, wrapping, escaping)
package text

import (
	"strings"
	"unicode/utf8"
)

// Truncate shortens text to maxLen characters, adding "..." if truncated.
func Truncate(s string, maxLen int) string {
	if utf8.RuneCountInString(s) <= maxLen {
		return s
	}
	runes := []rune(s)
	if maxLen <= 3 {
		return string(runes[:maxLen])
	}
	return string(runes[:maxLen-3]) + "..."
}

// TruncateLines limits the number of lines, adding an omission note.
func TruncateLines(s string, maxLines int) string {
	lines := strings.Split(s, "\n")
	if len(lines) <= maxLines {
		return s
	}
	half := maxLines / 2
	first := lines[:half]
	last := lines[len(lines)-half:]
	omitted := len(lines) - maxLines
	return strings.Join(first, "\n") +
		"\n... (" + itoa(omitted) + " lines omitted) ...\n" +
		strings.Join(last, "\n")
}

// CountLines returns the number of lines in a string.
func CountLines(s string) int {
	if s == "" {
		return 0
	}
	return strings.Count(s, "\n") + 1
}

// IndentLines prefixes each line with the given indent string.
func IndentLines(s, indent string) string {
	lines := strings.Split(s, "\n")
	for i, line := range lines {
		if line != "" {
			lines[i] = indent + line
		}
	}
	return strings.Join(lines, "\n")
}

// StripANSI removes ANSI escape codes from a string.
func StripANSI(s string) string {
	var result strings.Builder
	i := 0
	for i < len(s) {
		if s[i] == '\033' && i+1 < len(s) && s[i+1] == '[' {
			// Skip until we find a letter (end of ANSI sequence).
			j := i + 2
			for j < len(s) && !((s[j] >= 'A' && s[j] <= 'Z') || (s[j] >= 'a' && s[j] <= 'z')) {
				j++
			}
			if j < len(s) {
				j++ // skip the terminating letter
			}
			i = j
		} else {
			result.WriteByte(s[i])
			i++
		}
	}
	return result.String()
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	if n < 0 {
		return "-" + itoa(-n)
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
