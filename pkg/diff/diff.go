// Package diff provides diff parsing and formatting utilities.
// Source: utils/diff.ts — adjustHunkLineNumbers, countLinesChanged, getPatchForDisplay
package diff

import (
	"strings"
)

// ContextLines is the default number of context lines around changes.
const ContextLines = 3

// DiffTimeoutMs is the default timeout for diff operations.
const DiffTimeoutMs = 5000

// Hunk represents a section of a unified diff.
type Hunk struct {
	OldStart int
	OldLines int
	NewStart int
	NewLines int
	Lines    []string // prefixed with +, -, or space
}

// AdjustHunkLineNumbers shifts hunk positions by offset.
// Use when diffing a slice of a file rather than the whole file.
// Source: utils/diff.ts:17-27
func AdjustHunkLineNumbers(hunks []Hunk, offset int) []Hunk {
	if offset == 0 {
		return hunks
	}
	result := make([]Hunk, len(hunks))
	for i, h := range hunks {
		result[i] = h
		result[i].OldStart += offset
		result[i].NewStart += offset
	}
	return result
}

// LinesCounts holds additions and removals from a diff.
type LinesCounts struct {
	Added   int
	Removed int
}

// CountLinesChanged counts added and removed lines in diff hunks.
// For a new file (empty hunks), pass the content to count all lines as additions.
// Source: utils/diff.ts:49-79
func CountLinesChanged(hunks []Hunk, newFileContent string) LinesCounts {
	if len(hunks) == 0 && newFileContent != "" {
		return LinesCounts{Added: strings.Count(newFileContent, "\n") + 1}
	}

	var counts LinesCounts
	for _, h := range hunks {
		for _, line := range h.Lines {
			if strings.HasPrefix(line, "+") {
				counts.Added++
			} else if strings.HasPrefix(line, "-") {
				counts.Removed++
			}
		}
	}
	return counts
}

// FormatUnifiedDiff formats hunks as a unified diff string.
func FormatUnifiedDiff(path string, hunks []Hunk) string {
	if len(hunks) == 0 {
		return ""
	}

	var sb strings.Builder
	sb.WriteString("--- a/" + path + "\n")
	sb.WriteString("+++ b/" + path + "\n")

	for _, h := range hunks {
		sb.WriteString(formatHunkHeader(h))
		sb.WriteString("\n")
		for _, line := range h.Lines {
			sb.WriteString(line)
			sb.WriteString("\n")
		}
	}
	return sb.String()
}

func formatHunkHeader(h Hunk) string {
	return "@@ -" + itoa(h.OldStart) + "," + itoa(h.OldLines) +
		" +" + itoa(h.NewStart) + "," + itoa(h.NewLines) + " @@"
}

func itoa(n int) string {
	if n == 0 {
		return "0"
	}
	s := ""
	for n > 0 {
		s = string(rune('0'+n%10)) + s
		n /= 10
	}
	return s
}
